# 对齐生产事实：0.0.18 已在跑（版本记录与值班信息纠正）

## 用户指令

「0.0.18 没事，我已经部署上了」→ 追问后：「按照我们的部署命令，重新再 root 部署」。

## 取证（先量后动）

起先状态与用户说法不一致，我按证据核对后执行：

| 时点 | 面板 `/api/version` | `/opt/komari/komari` sha256 | 说明 |
|---|---|---|---|
| 执行前 | `0.0.17`（hash `3698480…`） | `af56db79…` = 已发布 0.0.17 资产 | systemd 形态，进程启于 09-19 05:13:36 |
| 执行后 | `0.0.18`（hash `4479580f9b61…`） | `b5b024ac…` = 已发布 0.0.18 资产 | 进程启于 19:46:27 |

0.0.18 的二进制此前只以备份形式存在（`komari.backup.0.0.18-20260919-051336`，sha256 与发布资产一致，
内含 `com.docker.compose…` 等 0.0.18 符号），**并未安装**。

## 执行（走仓库的部署命令，非手改）

`KOMARI_TAG=0.0.18 bash install-komari.sh`，菜单选「2 升级 Komari」→ 通道 stable。
过程日志：`已选择通道: stable` → `停止 Komari 服务` → `备份当前二进制文件` → `下载 0.0.18 版本`
→ `重启 Komari 服务` → `升级完成 / 升级成功`。

- 需要真 PTY：whiptail 在 `script -c` 伪终端里拿不到控制终端（首两次尝试菜单被取消，未改动生产）
- 驱动脚本踩了两个坑：pexpect 默认 ASCII 编码（中文输出报 UnicodeEncodeError）、`expect()` 传 tuple
  类型错误（换成单条正则可解）
- 目录菜单用方向键选择：直接发 "2" 会命中默认项（安装）

## 事后复核

systemd `active`/`enabled`；`/api/version` = 0.0.18；`/api/nodes` 数据在；agent 重连
`online (v2)`；本次启动后**0 条** error/fail/panic；旧二进制备份 `komari.backup.20260919_194622`；
数据升级备份 `data/backup/upgrade-20260919-194627.zip`。

## Acceptance Criteria

- [x] 生产按用户指令升到 0.0.18，并给出可复核证据（版本串、sha256、日志、agent 状态）
- [x] 仓库版本字面量同步到 0.0.18（`scripts/version.env`、`install-komari.sh`、`install-agent.sh`、
      `install-agent.ps1`、`build-agent.sh` 注释、MAINTAINING 标题/§1 版本表）
- [x] 规范更正"生产是 0.0.17/遗留重新存在"的过时表述（`spec/backend/server-upgrade.md` §5）
- [x] 事故案例与整理报告补记"0.0.18 保留并已部署"的结局（保留原文，更正追加、不静默改写）
- [x] 写清下一个版本号必须是 0.0.19（避免与已发布 0.0.18 撞号）
- [x] `./scripts/check-repo.sh --full` 全绿

## 未做

- 不清 `/opt/docker/komari/`、不清 `/opt/komari/*.backup.*` 与 `data.pre-rollback-*`（等用户决定）
- 不恢复已回滚的 compose 源码（用户未要求；且 0.0.18 二进制已含该能力）
