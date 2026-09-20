# Komari

自用服务器监控：单个 Go 二进制 + 内嵌 Web 前端，被监控节点装一个轻量 agent 上报指标。
本仓库由 [zhemed](https://github.com/zhemed) 独立维护，版本线 `0.0.x`，代码源自上游
[komari](https://github.com/komari-monitor/komari) 1.4.3——**不是上游官方发行版**。

## 部署（Docker 镜像，唯一主方案）

```bash
docker run -d --name komari --restart always \
  --network host \
  -v ./data:/app/data \
  ghcr.io/zhemed/komari:latest
```

- 多架构镜像（amd64/arm64）；数据在宿主机的 `./data`（SQLite），换镜像/重建容器都不丢
- 启动后访问 `http://localhost:25774` 完成初始化（首次是安装向导）
- 不想用 host 网络：把 `--network host` 换成 `-p 25774:25774`

**升级**：管理端「有新版本」弹窗点一下即可——容器里零配置：在容器内下载新版本、校验
`komari-SHA256SUMS`、替换二进制并原地重启。想固定镜像版本时换 tag 重跑上面那条命令
（数据在宿主目录，不受影响）。

### 备选：二进制 + systemd

```bash
curl -fsSL https://raw.githubusercontent.com/zhemed/komari/main/install-komari.sh -o install-komari.sh
sudo bash install-komari.sh        # 交互菜单：安装 / 升级 / 卸载 / 状态 / 日志 / 重启 / 停 / 清理备份
```

装到 `/opt/komari`（数据 `data/`），创建 `komari.service`（`Restart=always`）；
升级重跑同一条命令、菜单选「2 升级 Komari」。口径与验收步骤见
[docs/MAINTAINING.md](./docs/MAINTAINING.md) §3.4.1。

### 节点 agent

面板「节点 → 添加」里生成的就是同款命令（自带自动发现密钥，不必事先建节点）：

```bash
# Linux / macOS：默认装到 /opt/komari-agent，创建 systemd/launchd 服务
wget -qO- https://raw.githubusercontent.com/zhemed/komari/refs/heads/main/install-agent.sh | sudo bash -s -- \
  -e <面板地址> -t <节点Token>

# Docker
docker run -d --name komari-agent --restart always \
  ghcr.io/zhemed/komari-agent:latest -e <面板地址> -t <节点Token>
```

agent 默认**不自动升级**；要跟随发布加 `--enable-auto-update`（或 `AGENT_ENABLE_AUTO_UPDATE=1`）。
Windows 用 `install-agent.ps1`；全部平台见 release 资产列表（14 个平台）。

## 源码构建

服务端、面板前端（`frontend/`）与 agent（`agent/`）源码都在本仓库内，构建**不克隆上游**、可离线：

```bash
./scripts/build-komari.sh      # 本机二进制（需 Go；前端产物已随仓库下发）
./scripts/build-agent.sh       # agent 全 14 平台
./scripts/build-frontend.sh    # 仅改了 frontend/ 时需要（需 Node）
```

## 特性

- **实时监控**：CPU、内存、磁盘、网络、负载、进程数，历史曲线与聚合查询
- **多节点管理**：分组、标签、隐藏、权重、到期/账单、卡片与表格视图
- **流量统计**：上下行、月重置、限额；累计流量跨重启不丢
- **延迟监测**：HTTP/TCP/ICMP 定时探测与可用性统计
- **远程运维**：Web 终端（WebSSH）、远程执行、会话管理
- **访问控制**：多用户、2FA、OAuth/OIDC 单点登录、全局 API Key
- **其它**：GeoIP 地区识别、IPv4/IPv6 双栈、主题与主题市场、多语言、备份恢复、审计日志

### 与上游的差异

- **没有插件系统**（插件市场/运行时/上传安装均不存在）
- **没有通知系统**（离线/负载/流量/到期/登录通知与各渠道已整体移除；需要告警请接外部方案）
- **默认不自动更新**（服务器与 agent 都只在显式要求时更新）
- agent 与服务器同一条血统（停在 1.4.3 同期），不跟上游 agent 1.5.x

## 构建与维护

维护者文档：[docs/MAINTAINING.md](./docs/MAINTAINING.md)（构建契约、发布清单、agent 发行线、
容器镜像、数据与备份、回滚方式、部署口径）。提交前自检：

```bash
./scripts/check-repo.sh          # 秒级；发版前用 --full 追加 Go 门禁、离线构建、agent 门禁
```

## 维护与许可

以 MIT 许可发布，见 [LICENSE](./LICENSE)；第三方组件归属见 [NOTICE](./NOTICE)。
历史来源（仅作来源说明，不代表与上游同步）：服务端与面板源自
[komari-monitor/komari](https://github.com/komari-monitor/komari) `1.4.3`（`bf6b45ec`）与
[komari-monitor/komari-web](https://github.com/komari-monitor/komari-web) `1.4.3`（`4a74e8a8`），
agent 源自 komari-agent 1.4.3 同期（`1186aafb`）。上游版权归其作者所有。
