# 设计：面板一键升级服务器

## 1. 边界与职责

```
面板（浏览器）                服务端（komari）                      外部
──────────────                ─────────────────                    ──────
admin:listServerReleases ──▶  GitHub API 列表 + 平台/版本过滤
admin:upgradeServer(tag) ──▶  ① 前置检查（容器/systemd/权限）
                              ② 下载资产 → 校验 SHA256
                              ③ 自检 `--version` → 原子替换（留 backup）
                              ④ 写状态文件 → 退出（systemd 拉起）
admin:upgradeStatus      ──▶  读内存态/状态文件（重启后仍可读）
/api/version 轮询        ──▶  版本变化 = 升级完成
```

- **前端不参与**下载、校验、替换；也不允许传 URL（避免 SSRF/任意代码执行入口）。
- 目标仓库用**服务端配置**（新设置项 `update_repo`，默认 `zhemed/komari`）；
  前端那份 `VITE_KOMARI_UPDATE_REPO` 只用于它自己的"有没有新版本"提示，不作为升级依据。

## 2. 新增模块

`internal/upgrade/`（纯 Go 标准库，**不引入** `go-github-selfupdate`）：

| 文件 | 职责 |
|---|---|
| `releases.go` | GitHub releases 列表、语义化版本比较、按 `GOOS/GOARCH` 选资产（`komari-linux-amd64` 等）、`komari-SHA256SUMS` 解析 |
| `download.go` | 下载到临时文件（同目录，保证 rename 原子性）、流式 SHA256 校验、大小上限与超时 |
| `install.go` | 前置检查（容器标记、`/run/systemd/system`、目录可写）、`--version` 自检、备份、`os.Rename` 原子替换 |
| `state.go` | `data/upgrade-state.json`：`from/to/tag/result/error/时间/发起人`，供重启后面板展示"上次升级" |
| `upgrade.go` | 编排 ①~④；暴露 `Status()` 给 RPC 轮询 |

为什么不用 `go-github-selfupdate`：主模块原本没有该依赖（agent 才有），引入它同时带来
校验和/回滚/容器分支都不受控的问题；我们的需求（固定仓库、SHA256 校验、备份、容器分支、
指定 tag）用标准库约 300 行即可覆盖，且不新增供应链面。

## 3. RPC 契约（全部 admin-only，沿用 `/api/admin` 权限组）

| 方法 | 参数 | 返回 | 说明 |
|---|---|---|---|
| `admin:listServerReleases` | `{limit?}` | `[{tag,name,published_at,prerelease,current}]` | 只列稳定版（跳过 draft/prerelease） |
| `admin:upgradeServer` | `{tag?}` | `{started:true, from, to}` | 空 tag = 最新稳定版；重复调用返回 `already running` |
| `admin:upgradeStatus` | `{}` | `{phase, from, to, error?, last_result?}` | `phase ∈ idle/downloading/verifying/replacing/restarting/failed`；进程重启后从状态文件读 `last_result` |

错误约定沿用 `pkg/rpc`：参数错误 `InvalidParams`，权限不足由权限组返回 403，
容器/前台裸跑等不可执行场景返回带**可操作提示**的错误（例：`pull ghcr.io/zhemed/komari:0.0.8`）。

## 4. 关键流程与不变量

1. **前置检查**（任一条不满足即拒绝，且不触碰磁盘）：
   `/.dockerenv` 存在 → 容器分支（返回 pull 提示，不算错误）；
   `/run/systemd/system` 不存在 → 下载模式（仅下载到 `data/upgrades/` 并给命令）；
   二进制所在目录或文件不可写 → 报错。
2. **下载**：资产 URL 由 release API 返回（不拼字符串）；写 `<binary-dir>/.komari-upgrade.tmp`；
   边下边算 SHA256，与 `komari-SHA256SUMS` 中该资产条目比对；不符即删除临时文件并失败。
3. **自检**：`chmod 0755` 后执行 `<tmp> --version`，要求输出包含 `Komari Monitor <tag>`；
   不通过则删除临时文件。**绝不替换未经自检的文件。**
4. **替换**：`os.Rename` 现有二进制 → `<binary>.backup.<当前版本>`；再把 tmp `os.Rename` 成正式路径。
   两步 rename 之间若崩溃，磁盘上仍有可用 backup，文档给恢复命令。
5. **状态与退出**：写 `data/upgrade-state.json`（result=`success_pending_restart`）→ 记录审计日志 →
   `os.Exit(42)`；systemd `Restart=always` 拉起新进程；新进程启动时把状态改为 `success`
   （`/api/version` 版本变化即为最终判据）。
6. **不碰数据**：升级流程不写任何业务表；DB 迁移由新版本自身启动时按既有逻辑执行
   （0.0.6 起有自动备份，见 `docs/MAINTAINING.md`）。

## 5. 前端设计（`AdminPanelBar.tsx` 现有弹窗内）

- 弹窗底部按钮组：`立即升级`（有新版时）/`安装此版本`（版本列表里每行）/`复制 pull 命令`（容器态）。
- 状态机：点击 → 调 `admin:upgradeServer` → 每 1s 轮询 `admin:upgradeStatus`；
  轮询失败（连接被重启切断）视为"重启中"，随后每 2s 轮询 `/api/version` 直到版本变化或超时（60s）→ 显示结果。
- 失败展示：状态文件里的 `error` 原文 + 手工恢复命令（含 backup 路径与 `KOMARI_TAG=` 回退命令）。
- 权限：仅管理员（弹窗本身在管理端）；新增 5 语言文案。

## 6. 发版流程改动（配套，必须做）

- `scripts/build-komari.sh` / 发版清单：新增 `komari-SHA256SUMS`（两行，amd64/arm64），
  作为 release 资产随服务器二进制一起上传（资产数 17 → 18）。
- `docs/MAINTAINING.md` §3.4 第 6 步的命令与资产清单同步更新；
  并写清"缺 `komari-SHA256SUMS` 的历史 release（≤0.0.7）不支持一键升级"。

## 7. 兼容性与限制（写进文档，不假装支持）

| 形态 | 行为 |
|---|---|
| linux/amd64、linux/arm64 + systemd | ✅ 完整一键升级 |
| 容器 | ⚠️ 只给 `docker pull` 提示 |
| 前台裸跑 / 无 systemd | ⚠️ 只下载 + 给命令 |
| darwin/windows | ❌ 拒绝（无对应发布资产）：返回当前版本需手工升级 |
| 非 root / 只读目录 | ❌ 明确报错 |

## 8. 安全边界（诚实写明）

- 面板因此获得"下载并执行代码"的能力 → 仅管理员、固定仓库、审计日志、可关闭（R9）。
- SHA256 只保证"下载的文件与发布清单一致"；**发布账号被攻破时不提供保护**。
  签名体系（minisign/GPG）留到 v2，接口位置预留在 `download.go` 的校验步骤之后。

## 9. 风险与回滚

| 风险 | 缓解 |
|---|---|
| 新二进制启动即崩 | systemd `StartLimitBurst=5`/10s 后进入 failed（不会无限重启）；文档给 backup/KOMARI_TAG 恢复命令 |
| 磁盘写满 / 下载中断 | 临时文件 + 校验失败即删除；不改动正式二进制 |
| 前端产物内嵌，改动必须发版 | 记入实施计划：前端构建 → 更新 `scripts/frontend-build.env` 的 `FRONTEND_TREE_SHA256` |
| 升级动作本身被滥用 | 权限 + 审计 + 开关 |
