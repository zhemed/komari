# 本机部署 Komari 本地实例（二进制 + systemd）

## Goal

在本机（DSH 主机，linux/amd64）用**已发布的 0.0.25 二进制 + systemd** 起一个**全新独立**的
Komari 实例：数据独立（新库）、不触碰 VPS 生产（另一台机器上的 0.0.25 容器保持不动），
便于本地维护、调试与后续验证。

## 背景与已确认事实

- 用户 2026-11-12 定稿：目标 = 本机全新实例；形态 = 二进制 + systemd（本机未装 Docker/Go，
  用发布资产最省事，且 systemd 可开机自启）。
- 本机（DSH 主机）：Ubuntu 22.04 / x86_64 / systemd 可用 / 端口 25774 空闲 / 未装 Docker、Go、nginx。
- 发布资产：`zhemed/komari` release `0.0.25` 含 `komari-linux-amd64` 与 `komari-SHA256SUMS`
  （静态链接，`docs/MAINTAINING.md` §3.3/§3.4）。
- VPS 生产：另一台机器上的容器 `ghcr.io/zhemed/komari:0.0.25`（**本任务不碰**；站点专属值不写入本仓库）。

## Requirements

- **R1 全新独立实例**：新数据目录、新数据库，不迁移 VPS 数据、不改 VPS 配置。
- **R2 用发布资产**：下载 `0.0.25` 的 `komari-linux-amd64`，用 release 的 `komari-SHA256SUMS`
  校验 sha256，不一致即中止。
- **R3 systemd 托管**：安装到 `/opt/komari`，unit 名 `komari.service`（`Restart=always`、
  `WorkingDirectory=/opt/komari`、开机自启），数据落在 `/opt/komari/data`。
- **R4 只监听局域网**：绑定本机 LAN 地址的 `25774` 端口，不绑定 `0.0.0.0`（本机有公网 IPv6，
  避免面板意外暴露）。
- **R5 可维护**：本机留一份运行手册（服务启停、日志、备份、升级、回滚），不进公开仓库
  （仓库约定：不写站点专属值）。
- **R6 不动生产**：不登录/不修改 VPS；不发布新版本、不推 tag。

## Acceptance Criteria

- [x] **AC1**：`komari.service` 处于 `active (running)`，`systemctl is-enabled` 为 `enabled`。
- [x] **AC2**：`curl http://<LAN-IP>:25774/api/version` 返回 `0.0.25`；`/install` 返回 200。
- [x] **AC3**：数据目录 `/opt/komari/data` 已生成（SQLite 库文件存在）。
- [x] **AC4**：二进制 sha256 与 release `komari-SHA256SUMS` 一致（记录校验输出）。
- [x] **AC5**：本机运行手册存在（含启停/日志/备份/升级/回滚命令），且不在仓库内。
- [x] **AC6**：VPS 生产未被改动（本任务无任何 VPS 操作）。

## 非目标

- 不迁移 VPS 数据、不切换生产入口、不改 DNS/nginx。
- 不安装 Docker、不安装 Go/zig（本实例只用发布二进制）。
- 不在公开仓库写本机 IP/端口等站点专属值（运行手册放仓库外）。

## Notes

- 首次访问需要在 `/install` 完成管理员账号初始化（浏览器操作）。
- 后续升级：重复"下载 → 校验 → 替换二进制 → restart"即可（与 `install-komari.sh` 的升级口径一致）。
