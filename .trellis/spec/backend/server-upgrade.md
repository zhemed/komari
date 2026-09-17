# 服务器一键升级（backend）

> 规范描述**代码现状**。锚点会随代码漂移，改到相关代码时顺手更新。

---

## 0. 一句话契约

**升级动作全部在服务端执行**（`internal/upgrade`），前端只触发与展示进度；
目标仓库只来自服务端配置，**接口不接受请求方传入的 URL**。

---

## 1. 文件与职责

| 位置 | 职责 |
|---|---|
| `internal/upgrade/releases.go` | release 列表、版本比较（`ParseVersion`/`CompareVersions`）、平台资产名、`komari-SHA256SUMS` 解析 |
| `internal/upgrade/download.go` | 下载到临时文件 + 流式 SHA256 校验；失败删除临时文件（`ErrChecksumMismatch`） |
| `internal/upgrade/install.go` | 部署形态探测（容器/ systemd / 目录可写）、`--help` 自检、备份 + 原子替换 |
| `internal/upgrade/state.go` | 阶段状态与 `upgrade-state.json`（原子写）；`Reconcile` 把 restarting 收敛成 completed |
| `internal/upgrade/upgrade.go` | `Prepare`（解析目标 + 前置检查 + 取校验和）与 `Execute`（下载→校验→自检→替换） |
| `internal/dockerapi/*` | Docker Engine API 最小客户端（unix socket）：拉镜像、inspect、改名、建/起/停/删容器、日志；`Recreate` 含回滚 |
| `cmd/dockerSelfRecreate.go` | helper 子命令 `komari docker-self-recreate`：由旧容器启动、跑在独立容器里执行重建 |
| `web/rpc/jsonrpc/admin.upgrade.go` | 4 个 admin RPC + 设置读取 + 审计日志 + 退出口（helper 接管时不退出） |

RPC：`admin:getServerUpgradeSettings`、`admin:listServerReleases`、`admin:upgradeServer`、
`admin:upgradeStatus`（`admin:setServerUpgradeSettings` 已删除：设置走通用设置接口）。

---

## 2. 数据与配置

| 键 | 默认 | 说明 |
|---|---|---|
| `server_upgrade_enabled` | `true` | 关闭后接口拒绝、面板隐藏按钮 |
| `server_update_repo` | `zhemed/komari` | 只从该仓库的 release 下载；`admin:editSettings` 落库前用 `validateUpgradeSettingChanges` 校验 `owner/repo` 形式 |

状态文件：`<二进制目录>/upgrade-state.json`（**不是** data 目录——web 层没有可用的数据目录 helper，
DB 路径在 `cmd` 包里，导入会成环；升级本来就要写二进制目录，且前置检查已确认可写）。

---

## 3. 支持的部署形态（不许在代码里假装更多）

`Prepare` 的顺序很重要：**容器 → 平台/资产/校验和 → 无 systemd（仅下载）→ 要求目录可写**。

| 形态 | 结果 |
|---|---|
| linux + systemd + 可写 | 完整替换 |
| 容器 + **有可用 docker socket** | `Mode=docker-recreate`：拉镜像（进度）→ 起 helper 容器重建自身（`internal/dockerapi`） |
| 容器 + **无 socket** | `Mode=manual`：`PullHint()`，`Execute` 不做任何替换 |
| 无 `/run/systemd/system` | `Plan.DownloadOnly=true`，落 `<二进制目录>/upgrades/` 或 `$TMPDIR/komari-upgrades` |
| darwin/windows/其它架构 | `ServerAssetName` 返回 false → 报错 |

---

## 4. 不变量（改这块必须保持）

1. 替换前必须 `VerifyBinary`：跑 `<新二进制> --help`，输出含 `Komari Monitor <tag>`。
   **本项目的二进制没有 `--version` flag**（实测退出码 1 + "unknown flag"），别改成它。
2. 校验和不符 → 删临时文件 + 返回 `ErrChecksumMismatch`，**绝不替换**。
3. 替换 = `os.Rename(旧→backup)` + `os.Rename(新→正式)`；第二步失败要尝试把 backup 换回来。
4. 不做自动回滚（0.0.8 决策）：兜底为 systemd 启动限流 + "安装指定版本" + 备份文件。
5. 并发保护：`upgrade.IsRunning()` + `upgradeStartMu`，重复触发返回"已有升级任务在进行中"。
6. **容器重建模式的顺序不可改**：预检（新镜像 `--help` 含版本行）→ 旧容器改名 →
   建同名新容器（沿用旧 Config/HostConfig/网络）→ **停旧** → 起新；失败则删半成品 + 改名回滚 + 启动旧容器。
   helper 必须跑在**独立容器**里（自己不能重建自己）；helper 用**目标镜像**启动（它必然带该子命令）。
7. 重建时若旧 `Config.Hostname` 等于旧容器短 ID，必须删掉该字段（否则新容器被钉上旧 ID，
   而"自身容器识别"依赖 hostname==短 ID）。
8. 容器模式仍受 `server_upgrade_enabled` 约束；socket 存在本身就意味着用户显式选择了这个能力。

---

## 5. 测试要求

- `internal/upgrade/*_test.go`：版本比较/资产选择/校验和解析、校验失败不覆盖原二进制、
  自检失败不覆盖、容器分支只给 pull 命令、无 systemd 仅下载、已是最新报 `ErrUpToDate`、
  缺校验和资产给出可操作提示、`Reconcile` 收敛。
- `web/rpc/jsonrpc/admin.upgrade_test.go`：`validateUpgradeRepo`（挡 URL/注入）、
  `buildUpgradeOptions`（二进制路径与状态目录取自可执行文件）。
- 端到端（发版后必须实测一次）：面板/接口从 N 升到 N+1，再用"安装指定版本"退回 N，数据无损。

---

## 6. 发版配套

- `scripts/gen-release-sums.sh` 生成 `dist/komari-SHA256SUMS`，release 资产数 17 → 18（见
  `docs/MAINTAINING.md` §3.4）。
- `install-komari.sh` 的 `verify_download_checksum` 是**尽力而为**：清单不存在（≤0.0.7）时只告警，
  否则回滚到旧版本这条路会被自己堵死。
