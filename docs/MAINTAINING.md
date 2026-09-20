# Komari 维护手册（自维护版 · 当前 0.0.19）

自用项目：只维护 `0.0.x` 自己的版本线。服务端、面板前端（`frontend/`）、agent（`agent/`）
源码都在本仓库内，构建不克隆上游；上游 komari 1.4.3 只是历史来源。

代码细节规范在 `.trellis/spec/`（改代码前读对应文件），本文只讲**怎么干活**。

## 版本口径

- 唯一默认值：`scripts/version.env` 的 `KOMARI_VERSION`；新版号必须同步这三处：
  `install-komari.sh` 的 `REPO_TAG`、`install-agent.sh` 的 `default_agent_version`、
  `install-agent.ps1` 的 `$DefaultAgentVersion`（自检第 1 项会核对）。
- 递增 patch 位；**发版前先查号发过没有**（`gh release list -R zhemed/komari --limit 5`）。
  已发布：`0.0.1`…**`0.0.19`**（当前 Latest；0.0.18 保留为含 compose 那套机制的历史版本）。
- 带后缀的 tag（`0.0.1-fix1`）永远不会被面板判为"可更新"（`parseSemver` 只认三段）。

## 构建

| 需要什么 | 命令 |
|---|---|
| 本机二进制（开发用，需 Go + gcc） | `./scripts/build-komari.sh` → `bin/komari` |
| 静态发布二进制（需 zig） | `KOMARI_STATIC=1 KOMARI_OUTPUT=dist/komari-linux-amd64 ./scripts/build-komari.sh` |
| agent 全 14 平台（纯 Go，不需 gcc/zig） | `./scripts/build-agent.sh` → `dist/agent/` |
| 重新生成前端产物（改了 `frontend/` 时） | `./scripts/build-frontend.sh`（需 Node；产物 `web/public/defaultTheme/` 已入库） |

工具链：Go ≥ 1.25（`go.mod` 为准）、gcc（CGO，sqlite3）、zig 仅静态构建、Node 仅重建前端。

## 发布一个版本

```bash
# 1) 提交 + 同步字面量到新版本号（见上）  2) 构建
KOMARI_STATIC=1 KOMARI_OUTPUT=dist/komari-linux-amd64 ./scripts/build-komari.sh
KOMARI_STATIC=1 KOMARI_GOARCH=arm64 KOMARI_OUTPUT=dist/komari-linux-arm64 ./scripts/build-komari.sh
./scripts/build-agent.sh && ./scripts/gen-release-sums.sh
# 3) 自检  4) 打 tag 推送  5) 发 release（18 资产）6) 推镜像
./scripts/check-repo.sh --full
git tag <版本> && git push origin <版本>
gh release create <版本> -R zhemed/komari --title "<版本>" --notes-file <说明.md> \
  dist/komari-linux-amd64 dist/komari-linux-arm64 dist/komari-SHA256SUMS \
  dist/komari-agent-SHA256SUMS dist/agent/komari-agent-*
./scripts/build-server-image.sh --push && ./scripts/build-agent-image.sh --push
```

- **先提交后构建**：二进制里的 hash 来自构建时的 `git rev-parse HEAD`。
- `gh` 有两个 remote，务必带 `-R zhemed/komari`（否则会解析到上游）。
- 发布说明要写清"哪条修复、怎么验证的"（证据标准见
  `.trellis/spec/guides/evidence-and-claims-guide.md`）。
- 本仓库没有发布流水线；唯一 CI 是 `.github/workflows/trellis-gate.yml`（流程审计 + 自检自测）。

## 部署口径（用户定稿）

- **主方案：Docker 镜像** —— `docker run -d --name komari --restart always --network host
  -v ./data:/app/data ghcr.io/zhemed/komari:latest`；数据在宿主 `./data`。
  升级走面板「有新版本」（容器内下载→校验→替换→原地重启，零配置）。
- **备选：二进制 + systemd** —— `KOMARI_TAG=<版本> bash install-komari.sh`（菜单选 2 升级）；
  装到 `/opt/komari`，数据 `data/`，`komari.service`（`Restart=always`）。
  过程：停服务 → 备份 `komari.backup.<时间戳>` → 下载 → 校验 → 替换 → 启动 →
  程序自己备份数据到 `data/backup/upgrade-<时间戳>.zip`。
- **compose 一律不用**（用户明确剔除；自检第 13/14 项会拦）。事故记录：
  `.trellis/spec/guides/incident-compose-autosync.md`。
- 非交互跑安装脚本的三个坑：要真 PTY（whiptail）、菜单用方向键、pexpect 需 `encoding=None`。

升级/换镜像后的验收：

```bash
curl -s http://127.0.0.1:25774/api/version    # version/hash 对不对
docker ps --filter name=komari --format '{{.Image}} {{.Status}}'   # 或 systemctl is-active komari
journalctl -u komari --since '10 min ago' | grep -ciE 'error|fail|panic'   # 容器用 docker logs
```

## 数据与回滚

- 数据：`<工作目录>/data/`（`komari.db` 配置与用户、`metrics.db` 指标、`backup/` 升级自动备份）。
- 回滚：二进制换回 `komari.backup.<时间戳>`（或换回旧镜像 tag）后重启；
  数据用 `data/backup/upgrade-<时间戳>.zip`。面板"安装指定版本"也能回滚。

## agent 发行线

- 与服务器**同一个 release**（同一批 18 个资产），版本号一致；默认**不自动升级**
  （要跟随发布加 `--enable-auto-update` 或 `AGENT_ENABLE_AUTO_UPDATE=1`）。
- 构建脚本自带三道门禁（资产过滤 `^komari-agent-`、`Repo` 固定 zhemed/komari、
  安装脚本版本字面量一致），改 agent 前先读 `.trellis/spec/backend/build-and-pinning.md`。
- 容器里的 agent 会跳过二进制自更新——升级请换镜像。

## 自检与闸门

```bash
./scripts/check-repo.sh          # 秒级：版本字面量/文档锚点/脏文件/密钥/产物哈希/流程闸门/自检自测等
./scripts/check-repo.sh --full   # 追加 go build|vet|test、GOPROXY=off 离线构建、agent 门禁
```

提交必须绑定 Trellis 任务（消息带 `[task:<slug>]`），三层闸门见
`.trellis/spec/guides/trellis-gate-guide.md`；每个新克隆要跑一次 `./scripts/install-git-hooks.sh`。

## 本地产物（可重建，别入库）

`bin/`、`dist/`、`.build/`、`frontend/node_modules/`、`frontend/dist/` 全部可重建；
`.build/rel-notes-*.md` 是历次发布说明，**不要删**；`utils/geoip/data/` 是手工下载的 GeoIP 库。
