# Komari

自用服务器监控：单个 Go 二进制 + 内嵌 Web 前端，被监控节点装一个轻量 agent 上报指标。

## 特性

- **实时监控**：CPU、内存、磁盘、网络、负载、进程数的实时指标与历史曲线
- **多节点管理**：分组、标签、隐藏节点、权重与到期/账单信息、卡片/表格两种视图
- **远程运维**：Web 终端（WebSSH）、文件管理、远程执行、登录会话管理
- **延迟监测**：HTTP/TCP/ICMP 定时探测与可用性统计
- **流量统计**：上下行流量、月重置、流量限额与流量报表
- **访问控制**：多用户、两步验证（2FA）、OAuth/OIDC 单点登录、全局 API Key
- **网络信息**：GeoIP 地区识别、IPv4/IPv6 双栈、网卡与挂载点过滤
- **主题系统**：内置主题 + 主题市场，可换肤；界面多语言
- **运维能力**：数据备份与恢复、审计日志、性能分析（pprof）
- **节点自治**：agent 覆盖 linux/darwin/windows/freebsd 共 14 个平台，支持自动发现注册

## 本 fork 的取舍

本仓库是 [komari](https://github.com/komari-monitor/komari) 1.4.3 血统的**自维护分支**，
版本线 `0.0.x`，**不是上游官方发行版**。刻意的差异：

- **没有插件系统**：插件市场、运行时、上传安装全都不存在；
- **没有通知系统**：离线/负载/流量/到期/登录通知与 Telegram、Bark、Webhook 等渠道已整体移除——
  需要告警请自己接外部方案（数据都在本地 SQLite 与 HTTP 接口里）；
- **默认不自动更新**：服务器与 agent 都不会自己去拉新版本（agent 要跟随发布需显式
  `--enable-auto-update`）；
- 主题系统与主题市场保留；仓库历史只包含我们自己的提交。

完整说明见 [docs/MAINTAINING.md](./docs/MAINTAINING.md)。

## 部署

### 环境要求

- Linux（amd64/arm64），systemd 或 Docker
- 默认端口 **25774**；数据默认放在工作目录的 `./data`（SQLite）

### 一键安装（systemd）

```bash
curl -fsSL https://raw.githubusercontent.com/zhemed/komari/main/install-komari.sh -o install-komari.sh
sudo bash install-komari.sh
```

交互式菜单提供安装 / 升级 / 卸载 / 查看状态 / 查看日志 / 重启 / 停止 / 清理升级备份；
默认装到 `/opt/komari`，创建 `komari.service`（`Restart=always`）。

### 方式一：Docker 镜像（无需源码）

```bash
docker run -d --name komari --restart always \
  --network host \
  -v ./data:/app/data \
  ghcr.io/zhemed/komari:latest
```

- 多架构镜像（amd64/arm64），数据保存在宿主机的 `./data`
- 启动后访问 `http://localhost:25774` 完成初始化
- 不想用 host 网络时，把 `--network host` 换成 `-p 25774:25774`

### 方式二：源码构建

```bash
git clone https://github.com/zhemed/komari.git
cd komari

./scripts/build-komari.sh        # 开发用：动态链接，需 Go ≥1.25 + gcc；前端产物已随仓库下发
./bin/komari server              # 默认监听 0.0.0.0:25774

# 发布形态（静态链接，需 zig）与自建镜像
KOMARI_STATIC=1 KOMARI_OUTPUT=dist/komari-linux-amd64 ./scripts/build-komari.sh
./scripts/build-server-image.sh --push
```

### 节点安装 agent

面板「节点 → 添加」里生成的就是同款命令（自带自动发现密钥，不必事先建节点）：

```bash
# Linux / macOS：默认装到 /opt/komari-agent，创建 systemd/launchd 服务
wget -qO- https://raw.githubusercontent.com/zhemed/komari/refs/heads/main/install-agent.sh | sudo bash -s -- \
  -e <面板地址> -t <节点Token>

# Docker
docker run -d --name komari-agent --restart always \
  ghcr.io/zhemed/komari-agent:latest -e <面板地址> -t <节点Token>
```

- agent 由本仓库发布（与服务器同一条 `0.0.x` 线、同一个 release），**默认不自动升级**；
  要跟随发布加 `--enable-auto-update`（或环境变量 `AGENT_ENABLE_AUTO_UPDATE=1`）
- Windows 用 `install-agent.ps1`；全部平台/架构见 release 资产列表
- 容器里 agent 会跳过二进制自更新，升级请换镜像

## 构建与维护

维护者视角的内容都在 [docs/MAINTAINING.md](./docs/MAINTAINING.md)：构建契约与版本固定、
前端 vendor 与补丁系列、发布清单、agent 发行线、容器镜像、数据目录与备份、回滚方式。
提交前自检：

```bash
go build ./... && go vet ./... && go test ./...
```

## 维护与许可

本项目由 [zhemed](https://github.com/zhemed) 维护，版本线 `0.0.x`（发版递增 patch 位）。
以 MIT 许可发布，见 [LICENSE](./LICENSE)；第三方组件归属见 [NOTICE](./NOTICE)。

派生自上游 [komari-monitor/komari](https://github.com/komari-monitor/komari) `1.4.3`
（commit `bf6b45ec`）与 [komari-monitor/komari-web](https://github.com/komari-monitor/komari-web)
`1.4.3`（commit `4a74e8a8`）；agent 源码来自
[komari-monitor/komari-agent](https://github.com/komari-monitor/komari-agent)。
上游版权归其作者所有。
