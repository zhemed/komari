# 本机注册为本地实例节点（安装 komari-agent 0.0.25）

## Goal

把本机（DSH 主机）注册为本地 Komari 实例（`/opt/komari`，0.0.25）的节点，用于测试节点卡片与
三网显示；agent 用 release 0.0.25 的 `komari-agent-linux-amd64`，systemd 托管、开机自启。

## 背景与已确认事实

- 本地实例已部署并激活 ink 主题 1.0.4（另见运行手册）。
- 本机 linux/amd64；release 0.0.25 含 `komari-agent-linux-amd64` + `komari-agent-SHA256SUMS`。
- 节点注册走 RPC `admin:addClient`（返回 uuid + token），agent 用 `-e <endpoint> -t <token>` 上报。
- 生产（另一台机器）不参与本任务。

## Requirements

- **R1** 用 release 0.0.25 的 agent 二进制，并用 release 的 `komari-agent-SHA256SUMS` 校验 sha256。
- **R2** 装到 `/opt/komari-agent`，`komari-agent.service` 托管（`Restart=always`、开机自启、User=root）。
- **R3** 节点名 `ubuntu-dsh`；token 只写本机 600 凭据文件与 unit，不进对话记录、不进仓库。
- **R4** 验证：agent active、节点出现在 `/api/nodes` 且在线、面板节点卡片有真实数据。
- **R5** 不碰生产；不配置 ping 任务（三网行待用户给目标后在后台配）。

## Acceptance Criteria

- [x] **AC1**：agent 二进制 sha256 与 release 清单一致。
- [x] **AC2**：`komari-agent.service` enabled + active，日志出现 `Basic info uploaded successfully`。
- [x] **AC3**：`admin:listClients` / `/api/nodes` 能看到 `ubuntu-dsh`（version 0.0.25，有 CPU/内存/IP）。
- [x] **AC4**：首页节点卡片渲染真实数据（截图确认，ink 主题）。
- [x] **AC5**：凭据只落在本机 600 文件；无站点专属值进入公开仓库。
