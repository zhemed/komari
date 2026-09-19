# Komari

自托管服务器监控：单个 Go 二进制 + 内嵌 Web 面板，被监控节点跑一个轻量 agent 上报指标。
本仓库是 [zhemed](https://github.com/zhemed) 维护的自有发行版（版本线 `0.0.x`），代码源自上游
komari 1.4.3，此后独立演进——**不是上游官方发行版**。

## 快速开始

**方式一：二进制 + systemd**（推荐；默认装到 `/opt/komari`，本机生产即此形态）：

```bash
curl -fsSL https://raw.githubusercontent.com/zhemed/komari/main/install-komari.sh | sudo bash
```

**方式二：Docker + compose**（备选，**生产未采用**；compose 路径有已知坑，见
[docs/MAINTAINING.md](./docs/MAINTAINING.md) §15）：

```bash
curl -fsSL https://raw.githubusercontent.com/zhemed/komari/refs/heads/main/install-compose.sh | sudo bash
```

装完访问 `http://<主机>:25774` 完成初始化。Linux（amd64/arm64）均可，数据放在 `./data`（SQLite）。
节点在面板「节点 → 添加」里生成安装命令（`install-agent.sh` / `install-agent.ps1` /
`ghcr.io/zhemed/komari-agent`，覆盖 14 个平台）。

**方式三：源码构建**

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

## 能力

实时指标与历史曲线、多节点分组与视图、Web 终端与远程执行、HTTP/TCP/ICMP 延迟监测、
流量统计（**累计值跨重启不丢**）、多用户 + 2FA + OAuth/OIDC、GeoIP 双栈、主题与多语言、
备份/审计/pprof。服务器与 agent 升级可在面板里一键完成。

## 这个发行版的三点不同

- **没有插件系统**（插件市场、运行时、上传安装全部移除）；
- **没有通知系统**（离线/负载/流量/到期/登录通知与 Telegram、Bark、Webhook 等渠道整体移除）——
  需要告警请接外部方案，数据都在本地 SQLite 与 HTTP 接口里；
- **默认不自动升级**：服务器与 agent 都不会自己拉新版本；agent 要跟随发布需显式
  `--enable-auto-update`。

服务端、面板（`frontend/`）与 agent（`agent/`）源码都在本仓库内：构建**不克隆上游、不打补丁**，
只改后端时连 Node 都不需要。

## 文档

- **[docs/MAINTAINING.md](./docs/MAINTAINING.md)** —— 所有细节的落点：构建与发布清单、
  容器镜像、部署约定与升级交互、数据与备份、回滚、已知遗留；
- 部署脚本参数：`install-compose.sh --help`（换目录/版本、端口映射、不挂 docker.sock 等）；
- 镜像挂了 `/var/run/docker.sock` 时面板可一键升级（代价：宿主 root 等价权限），
  不挂则走"容器内替换"，重建容器会退回镜像版本。

## 许可

MIT，见 [LICENSE](./LICENSE)；第三方组件归属见 [NOTICE](./NOTICE)。
上游版权归其作者所有（服务端/面板 1.4.3 `bf6b45ec`/`4a74e8a8`，agent 同期 `1186aafb`）。
