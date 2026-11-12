# 实施记录：本机注册为本地实例节点

## 步骤

- [x] RPC `admin:addClient {"name":"ubuntu-dsh"}` → uuid + token（token 只写本机 600 文件）
- [x] 下载 release 0.0.25 的 `komari-agent-linux-amd64` + `komari-agent-SHA256SUMS`，sha256 校验通过
- [x] 安装到 `/opt/komari-agent/komari-agent`（`install -m 755`）
- [x] 写 `/etc/systemd/system/komari-agent.service`（`-e <本机实例> -t <token>`，Restart=always、开机自启）
- [x] `systemctl enable --now komari-agent`

## 验证证据（2026-11-12 本机实测）

| 项 | 结果 |
|---|---|
| 资产校验 | `komari-agent-linux-amd64: OK`（release 清单） |
| 二进制形态 | ELF x86-64，statically linked |
| 服务 | `enabled` + `active`；日志 `Komari Agent 0.0.25` → `Basic info uploaded successfully` |
| 节点 | `admin:listClients`：`ubuntu-dsh`，version 0.0.25，有内存/CPU/IPv4/IPv6 |
| 公开接口 | `/api/nodes` 返回该节点（cpu_name / virtualization / arch / cores） |
| 面板 | 首页节点卡片渲染真实数据（截图确认；ink 主题） |
| 三网行 | 未出现（本机实例还没有 ping 任务）——需在后台「延迟监测」添加 3 个任务后自动出现 |

## 备注

- agent 自更新默认关闭（我们内联的契约），不会自动刷成别的东西。
- 移除：面板删节点 → `systemctl disable --now komari-agent` → 删 unit 与 `/opt/komari-agent`。
