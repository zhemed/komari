# Komari

自用服务器监控：单个 Go 二进制 + 内嵌 Web 前端，被监控节点装一个轻量 agent 上报指标。

## 特性

- **实时监控**：CPU、内存、磁盘、网络、负载、进程数的实时指标与历史曲线
- **多节点管理**：分组、标签、隐藏节点、权重与到期/账单信息、卡片/表格两种视图
- **远程运维**：Web 终端（WebSSH）、远程执行、登录会话管理
- **延迟监测**：HTTP/TCP/ICMP 定时探测与可用性统计
- **流量统计**：上下行流量、月重置、流量限额；**累计流量跨重启（含机器重启）不丢**
- **访问控制**：多用户、两步验证（2FA）、OAuth/OIDC 单点登录、全局 API Key
- **网络信息**：GeoIP 地区识别、IPv4/IPv6 双栈、网卡与挂载点过滤
- **主题系统**：内置主题 + 主题市场，可换肤；界面多语言
- **运维能力**：数据备份与恢复、审计日志、性能分析（pprof）
- **节点自治**：agent 覆盖 linux/darwin/windows/freebsd 共 14 个平台，支持自动发现注册

## 这个版本与上游的差异

本仓库由 [zhemed](https://github.com/zhemed) 独立维护，版本线 `0.0.x`。代码源自上游
[komari](https://github.com/komari-monitor/komari) 1.4.3，但此后与上游各走各的路：
**不是上游官方发行版**，服务端、面板与 agent 的源码都在本仓库内，上游库只作为历史来源。

- **没有插件系统**：插件市场、运行时、上传安装全都不存在；
- **没有通知系统**：离线/负载/流量/到期/登录通知与 Telegram、Bark、Webhook 等渠道已整体移除——
  需要告警请自己接外部方案（数据都在本地 SQLite 与 HTTP 接口里）；
- **默认不自动更新**：服务器与 agent 都不会自己去拉新版本（agent 要跟随发布需显式
  `--enable-auto-update`）；
- **agent 也停在 1.4.3 同期**（不跟上游 agent 1.5.x），与服务器同一条血统；
- **源码全在仓库内**：面板前端（`frontend/`）与 agent（`agent/`）的源码都是本仓库的快照，
  构建时**不克隆上游、不打补丁**；整套东西可以离线构建（只改后端时连 Node 都不需要）；
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

### 方式一：Docker（无需源码）

**一条命令部署**（写入 `/opt/docker/komari/docker-compose.yml` 并启动；幂等，不会覆盖已有部署）：

```bash
curl -fsSL https://raw.githubusercontent.com/zhemed/komari/refs/heads/main/install-compose.sh | sudo bash
```

需要换目录/版本/容器名，或要做**不挂 socket 的最小权限部署**时加参数（`| sudo bash -s -- --help` 看全部）：

```bash
curl -fsSL https://raw.githubusercontent.com/zhemed/komari/refs/heads/main/install-compose.sh | \
  sudo bash -s -- --dir /opt/docker/komari --tag 0.0.18 --no-socket
```

也可以自己写这份 compose（脚本生成的就是它）：

```yaml
# /opt/docker/komari/docker-compose.yml
services:
  komari:
    image: ghcr.io/zhemed/komari:0.0.17   # ① 钉具体版本（≥0.0.13 才有面板一键升级）
    container_name: komari
    restart: unless-stopped
    network_mode: host                     # ② 用 host 网络时不要写 ports
    environment:
      TZ: Asia/Shanghai
      KOMARI_LISTEN: 0.0.0.0:25774
    volumes:
      - ./data:/app/data                            # ③ 唯一有状态的东西（备份它就够了）
      - /var/run/docker.sock:/var/run/docker.sock   # ④ 升级走"拉镜像+重建容器"，见下方说明
    logging:                               # ⑤ docker 默认 json-file 无上限
      driver: json-file
      options:
        max-size: "10m"
        max-file: "3"
    healthcheck:                           # ⑥ 用 curl：镜像自带，且不会把 HTML 灌进健康日志
      test: ["CMD", "curl", "-fsSL", "-o", "/dev/null", "http://127.0.0.1:25774/"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 30s
```

```bash
mkdir -p /opt/docker/komari && cd /opt/docker/komari   # 目录约定：一项目一子目录
docker compose up -d
```

- 多架构镜像（amd64/arm64），数据保存在宿主机的 `./data`
- 启动后访问 `http://localhost:25774` 完成初始化
- **面板一键升级**：挂了 `/var/run/docker.sock`（上面 ④）时用"拉镜像 + helper 重建容器"，
  **版本与镜像始终一致**；代价是该容器获得**宿主机 root 等价权限**，不需要就地升级就别挂。
  升级成功后 helper 会**自动把上面 ① 的 tag 改成新版本**（0.0.18 起），所以之后改这个文件不会把版本拉回去
- 想用端口映射而不是 host 网络：删掉 `network_mode: host`，改成 `ports: ["25774:25774"]`
- 不用 compose 的最小形态：
  `docker run -d --name komari --restart unless-stopped --network host -v ./data:/app/data ghcr.io/zhemed/komari:0.0.18`

### 方式二：源码构建

```bash
git clone https://github.com/zhemed/komari.git
cd komari

./scripts/build-komari.sh        # 开发用：动态链接，需 Go ≥1.25 + gcc；前端产物已随仓库下发
./bin/komari server              # 默认监听 0.0.0.0:25774

# 改了面板前端（frontend/）或 agent（agent/）时
./scripts/build-frontend.sh      # 需 Node + 网络（npm ci）；重建 web/public/defaultTheme/
./scripts/build-agent.sh         # 纯 Go 交叉编译，14 个平台

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

- **面板内一键升级服务器**：管理端"有新版本"弹窗里可直接升级或安装指定版本（= 回滚）。
  二进制 + systemd 形态：下载后校验 `komari-SHA256SUMS`、替换前自检版本行、保留旧二进制备份；
  容器形态默认就是零配置：在容器内替换二进制并原地重启（不需要挂载、不需要 restart 策略）；
  可选把 `/var/run/docker.sock` 挂进来改用"拉镜像 + 重建容器"模式（版本与镜像完全一致，
  等于把宿主 root 等价权限交给该容器，请自行确认）。
- agent 由本仓库发布（与服务器同一条 `0.0.x` 线、同一个 release），**默认不自动升级**；
  要跟随发布加 `--enable-auto-update`（或环境变量 `AGENT_ENABLE_AUTO_UPDATE=1`）
- Windows 用 `install-agent.ps1`；全部平台/架构见 release 资产列表
- 容器里 agent 会跳过二进制自更新，升级请换镜像

## 构建与维护

维护者视角的内容都在 [docs/MAINTAINING.md](./docs/MAINTAINING.md)：构建契约与版本固定、
前后端与 agent 源码（都在本仓库内）、发布清单、agent 发行线、容器镜像、数据目录与备份、回滚方式。
提交前自检（一条命令，含版本口径/文档锚点/产物哈希/Trellis 流程闸门）：

```bash
./scripts/check-repo.sh          # 秒级；发版前用 --full 追加 Go 门禁、离线构建、agent 门禁
```

想单独跑 Go 三件套也行：`go build ./... && go vet ./... && go test ./...`

## 维护与许可

本项目由 [zhemed](https://github.com/zhemed) 维护，版本线 `0.0.x`（发版递增 patch 位）。
以 MIT 许可发布，见 [LICENSE](./LICENSE)；第三方组件归属见 [NOTICE](./NOTICE)。

历史来源（仅作来源说明，不代表本仓库与上游同步）：服务端与面板源自
[komari-monitor/komari](https://github.com/komari-monitor/komari) `1.4.3`（commit `bf6b45ec`）与
[komari-monitor/komari-web](https://github.com/komari-monitor/komari-web) `1.4.3`
（commit `4a74e8a8`），agent 源自
[komari-monitor/komari-agent](https://github.com/komari-monitor/komari-agent) 的 1.4.3 同期提交
（commit `1186aafb`）。上游版权归其作者所有。
