# Komari（自维护版）

轻量、自托管的服务器监控：**单个 Go 二进制 + 内嵌 Web 前端**，由轻量 agent 上报指标。

本仓库是**自维护分支**，版本线从 **0.0.1** 起步，独立演进——**不跟随上游 1.5.x**。

## 与上游的关系

代码派生自上游 [komari](https://github.com/komari-monitor/komari) `1.4.3`（commit `bf6b45ec`）
与 [komari-web](https://github.com/komari-monitor/komari-web) `1.4.3`（commit `4a74e8a8`），
此后由我们自己维护。**本仓库不是上游官方发行版。**

与上游的刻意差异（完整说明见 [docs/MAINTAINING.md](./docs/MAINTAINING.md)）：

- **移除插件系统**：插件市场、运行时、上传安装与插件 RPC 全部不存在。
- **后台“发现新版本”检查指向本仓库**，不再提示升级到上游 1.5.x。
- **默认主题产物 vendor 进仓库**：无需网络与 Node 即可构建后端。
- **安装脚本指向本仓库并锁定 tag**，不会安装上游版本。
- 仓库历史只包含我们自己的提交（上游代码以单个快照根提交引入）。

## 快速开始

### 直接运行二进制

```bash
# 从本仓库 release 下载对应架构的资产（资产名形如 komari-linux-amd64）
chmod +x komari-linux-amd64
./komari-linux-amd64 server
```

- 默认监听 `0.0.0.0:25774`，可用 `-l/--listen` 或环境变量 `KOMARI_LISTEN` 修改。
- 数据写在**当前工作目录**的 `./data`（SQLite 主库 `komari.db`）。
- 首次启动访问 `/install` 完成初始化。

```text
Komari [command]

  server        启动服务
  chpasswd      强制修改管理员密码
  disable-2fa   强制关闭 2FA
  permit-login  恢复密码登录

全局参数:
  -d, --database string   SQLite 数据库路径（默认 ./data/komari.db）
  -t, --db-type string    数据库类型（默认 sqlite）
```

### 安装脚本（systemd）

```bash
sudo bash install-komari.sh
```

安装到 `/opt/komari`、创建 `komari` systemd 服务、默认端口 `25774`。
脚本从本仓库 release 下载（`KOMARI_TAG` 默认 `0.0.1`，`KOMARI_REPO` 可覆盖）。

### Docker

`Dockerfile` 基于 `alpine:3.21`，因此**必须使用静态链接的二进制**：

```bash
KOMARI_STATIC=1 ./scripts/build-komari.sh          # 产出 bin/komari（静态）
mkdir -p /tmp/komari-ctx && cp bin/komari /tmp/komari-ctx/komari-linux-amd64
cp Dockerfile /tmp/komari-ctx/
docker build -t komari:0.0.1 /tmp/komari-ctx
docker run -d --name komari -p 25774:25774 -v komari-data:/app/data komari:0.0.1
```

> `Dockerfile` 用 `ARG TARGETOS/TARGETARCH`（默认 `linux/amd64`）定位构建上下文里的
> `komari-${TARGETOS}-${TARGETARCH}` 文件。

## 构建

### 普通构建（开发用）

需要 Go ≥ 1.25 与 gcc（cgo，SQLite 驱动）。**不需要网络与 Node**——默认主题产物已随仓库下发。

```bash
./scripts/build-komari.sh        # → bin/komari
```

### 静态构建（发布用）

需要 [zig](https://ziglang.org/)（用其 musl 目标做静态链接）：

```bash
KOMARI_STATIC=1 ./scripts/build-komari.sh                      # linux/amd64 静态
KOMARI_STATIC=1 KOMARI_GOARCH=arm64 ./scripts/build-komari.sh  # linux/arm64 静态
```

版本号与提交 hash 由脚本以 ldflags 注入（与上游口径一致）：
`CurrentVersion` 默认 `0.0.1`，可用 `KOMARI_VERSION` 覆盖。

### 重新生成前端产物（需要网络 + Node）

```bash
./scripts/sync-frontend.sh
```

按 `scripts/frontend-pin.env` 固定的 commit 重新构建默认主题，应用 `scripts/patches/` 下的补丁，
校验产物哈希后原子替换 `web/public/defaultTheme/`。哈希不一致时脚本会失败——这是**可复现性门禁**，
不要为了通过而清空哈希。

### 提交前自检

```bash
GOPROXY=off ./scripts/build-komari.sh        # 离线构建（验证产物完整）
go build ./... && go vet ./... && go test ./...
```

本仓库**没有 CI**（上游流水线已移除，避免产出与我们对不上的工件），以上命令需本地执行。

## 数据与备份

| 路径 | 说明 |
|---|---|
| `./data/komari.db` | 主库（配置、账号、节点等） |
| `./data/metrics.db` | 指标库 |
| `./data/theme/` | 已安装主题 |
| `./data/backup/` | 备份归档 |

- **升级自动备份**：当二进制的版本标识与库中记录不同时，启动会把整个 `./data`
  打包到 `./data/backup/upgrade-<时间>.zip` 再继续（`database/dbcore/dbcore.go`）。
  因此**每次以不同 commit 构建的二进制启动都可能产生一次备份**，属预期行为。
- 后台可上传备份并自动重启以应用。

## 版本与发布

- 版本号形如 `0.0.x`，发版**递增 patch 位**（前端版本比较只取 `x.y.z` 三段，带后缀的 tag 不会被识别为更新）。
- 发布流程（手动）：
  1. `KOMARI_STATIC=1 ./scripts/build-komari.sh`
  2. 资产命名为 `komari-linux-<arch>`（与 `install-komari.sh` 的期望一致）
  3. `gh release create <tag> komari-linux-amd64 ...`

## 来源与许可

本项目以 MIT 许可发布，见 [LICENSE](./LICENSE)。第三方组件归属见 [NOTICE](./NOTICE)。

派生自上游 [komari-monitor/komari](https://github.com/komari-monitor/komari) `1.4.3`
（commit `bf6b45ec`）与 [komari-monitor/komari-web](https://github.com/komari-monitor/komari-web)
`1.4.3`（commit `4a74e8a8`）；上游版权归其作者所有（`Copyright (c) 2025 Komari Moniter`）。

维护约定、构建契约与“哪些地方故意偏离上游”记录在 [docs/MAINTAINING.md](./docs/MAINTAINING.md)。
