# 文档一致性：容器升级的部署命令与说明

## Goal

让文档与 0.0.13~0.0.16 的**真实行为**一致：容器里**不挂任何东西**也能在面板一键升级；
`/var/run/docker.sock` 只是"版本与镜像完全一致"的**可选**增强，不是升级的必要条件。

用户 2026-09-17 提问："方式一：Docker 镜像（无需源码）部署命令要不要改"——答案是**要改**，
命令里那行 socket 挂载是旧说法残留，与紧跟其后的说明自相矛盾。

## Background（取证：当前残留的 7 处，行号为改前）

| 位置 | 现状 | 与真实行为的偏差 |
|---|---|---|
| `README.md:59` | 命令块里 `-v /var/run/docker.sock:/var/run/docker.sock \` | 默认**不需要**挂；挂上等于交出宿主 root 等价权限 |
| `README.md:107` | "没挂 socket 时只给可复制的 pull 命令" | 0.0.13 起没挂也能容器内替换升级 |
| `docs/MAINTAINING.md:595` | 表格行"容器 **挂了 docker socket** ✅ 重建容器" | 行文暗示"必须挂才行" |
| `docs/MAINTAINING.md:596` | 表格行"容器 **没挂 socket** ⚠️ 不替换任何东西：仅返回命令" | 与真实行为直接矛盾（会容器内替换） |
| `docs/MAINTAINING.md:636` | §14.6 标题"（挂 docker socket 时，0.0.11 起…）" | 标题把可选增强写成了前提 |
| `docs/MAINTAINING.md:655` | 示例注释"← 这一行让它能一键升级" | 应为"想用重建容器模式时加这一行" |
| `docs/MAINTAINING.md:662` | "没挂 socket 时行为不变（只给可复制的命令）" | 同上，已过时 |
| `docs/MAINTAINING.md:713` | "想要面板内一键升级，**只能**改用二进制 + systemd" | 就在 §14.4.2 正确说明的正下方，自相矛盾 |
| `internal/upgrade/upgrade.go:100` | 注释"ModeManual：容器但没挂 socket —— 只给命令" | manual 现在的触发条件只有只读 rootfs |
| `web/rpc/jsonrpc/admin.upgrade.go:297` | 注释"支持 = 二进制 + systemd，或容器 + socket" | 漏了 `ModeContainerReplace`（0.0.13 起的主路径） |
| `internal/upgrade/docker.go:41-42` | 注释"helper 用**目标镜像**启动…跑完由 daemon 自动删除" | 0.0.12 已改为当前镜像 + 不自动删除（正是 0.0.11 缺陷的成因） |

对照组（已正确，不动）：`README.md:65-69`（0.0.13 轮补的三条口径）、
`docs/MAINTAINING.md:638-647`（更正段 + 3 行对比表）、`.trellis/spec/backend/server-upgrade.md`（0 处残留）。

## Requirements

- **R1** README「方式一」命令块**不带** socket 挂载（默认零配置），并让紧跟的说明覆盖三条：
  ① 网页一键升级开箱可用（容器内替换 + 原地重启）；② 挂 socket 是可选增强（拉镜像 + 重建容器，
  版本与镜像一致），代价是宿主 root 等价权限；③ 不挂 socket 时，重建容器会退回镜像版本
  （`docker restart` 不会升级）。
- **R2** README L104-107 的功能点描述改成与 R1 同一口径（不许再出现"没挂 socket 只给命令"）。
- **R3** MAINTAINING §14.2 表格、§14.6 标题、§14.4.2 示例注释、§14.6 正文那句，全部改成同一口径。
- **R4** 不改任何运行行为（只改文档与代码注释；注释修正不得改变逻辑），改完后逐处 grep 复核，
  不引入新的不一致。
- **R5** 按 Trellis 流程走完：本任务 → 实现 → check（check-repo + grep 复核）→ journal → archive。

## Acceptance Criteria

- [ ] `grep -n "docker.sock" README.md docs/MAINTAINING.md` 每处都只把 socket 描述为**可选**
      的重建容器模式，不再出现"必须挂"或"没挂只给命令"的表述
- [ ] README 命令块可直接复制执行且不含 socket 行；说明解释可选增强与代价
- [ ] `./scripts/check-repo.sh` 全绿
- [ ] 任务归档，journal 记录

## Out of Scope

- 不改代码、不发版：README 由关于页运行时从仓库读取，纯文档改动无需重建产物；
- 不改历史发布说明（那是带时间戳的记录，0.0.13/0.0.14 的更正段保留）；
- 不改 `install-komari.sh` 等安装脚本的默认行为（本来就与文档一致）。
