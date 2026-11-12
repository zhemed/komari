# 实施清单：本机 Komari 本地实例（二进制 + systemd）

> 依赖：`prd.md`（R1–R6 / AC1–AC6）。本机无 Docker/Go，全程用 release 资产。

## 1. 部署步骤

- [x] 下载 release `0.0.25` 资产：`komari-linux-amd64` + `komari-SHA256SUMS`
- [x] 用 release 清单校验 sha256（`grep ' komari-linux-amd64$' komari-SHA256SUMS | sha256sum -c -`）
- [x] 安装二进制到 `/opt/komari/komari`（`install -m 755`）
- [x] 写 `/etc/systemd/system/komari.service`（`Restart=always`、`WorkingDirectory=/opt/komari`、
      `ExecStart=... server -l <LAN-IP>:25774`、`User=root`、开机自启）
- [x] `systemctl daemon-reload && systemctl enable --now komari`
- [x] 验证：服务 active/enabled；`/install` 200；数据目录生成 SQLite；重启后仍正常
- [x] 本机运行手册写到仓库外（含 IP/端口/升级/备份/回滚/卸载），不进公开仓库

## 2. 验证证据（2026-11-12 本机实测）

| 项 | 命令 | 结果 |
|---|---|---|
| 资产校验 | `sha256sum -c`（release 清单） | `komari-linux-amd64: OK`（sha256 `b40c781f…`） |
| 二进制形态 | `file` | ELF x86-64，`statically linked` |
| 版本 | 启动日志 | `Komari Monitor 0.0.25 (hash: 942bb4a…)` |
| 服务状态 | `systemctl is-enabled/is-active` | `enabled` / `active` |
| 端口 | `ss -lntp` | 仅监听 `<LAN-IP>:25774`（未绑 `0.0.0.0`） |
| 首装入口 | `curl /install` | 200（`/` 307 跳转；`/api/install/status` = `ready/required`） |
| 数据目录 | `ls /opt/komari/data` | `komari.db` + WAL/SHM + `theme/` 已生成 |
| 重启 | `systemctl restart komari` | 恢复 `active`，`/install` 仍 200 |
| 防火墙 | `ufw status` / `iptables -S INPUT` | ufw inactive、INPUT ACCEPT（LAN 可直连） |

## 3. 初始化与后续（已完成初始化）

- [x] 首次初始化：已通过 `/api/install/complete` 完成（管理员 `admin`，站点 `Komari Local`；
      初始凭据写在本机文件、不进对话/仓库）
- [x] 初始化后复核：`/api/version` → `0.0.25`；`POST /api/login` → 200；`GET /` → 200；`metrics.db` 已生成
- [ ] 下次发版（0.0.26）按本机运行手册的升级流程更新；release 已收敛为只有 linux/amd64

## 4. 回滚 / 卸载

- 二进制回滚：`/opt/komari/komari.backup.<时间戳>`（升级时保留）
- 完整卸载：`systemctl disable --now komari` → 删 unit → 备份后删 `/opt/komari`
- 本任务不涉及 VPS：无远端改动可回滚
