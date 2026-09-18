# Docker Compose 部署方案审查

## Goal

用户计划把部署形态切到 docker compose，伞目录约定 `/opt/docker/<project>/`（750、一项目一子目录），
并给出一份**尚未定稿**的 compose（钉版本 / restart / network_mode host / env / volume / healthcheck），
要求检查。目标：对着**真实镜像**逐条验证，给出可直接用的定稿与证据。

## Requirements

- **R1** 每条结论必须有实测证据（命令 + 输出），不写"应该可以"。
- **R2** 覆盖：镜像里 healthcheck 要用的工具是否存在、`KOMARI_LISTEN`/`TZ`/`GIN_MODE` 是否真被读取、
  卷路径与运行身份、host 网络与 ports 的取舍、日志上限、钉版本与面板一键升级的交互。
- **R3** 产出可直接复制的定稿 compose，并把"必须改的项"与"已确认正确的项"分开列。
- **R4** 不越界：不在用户宿主机 `/opt/docker/` 实际部署（越出工作区，需另行授权）。
- **R5** 走完 Trellis 流程并归档。

## Acceptance Criteria

- [x] 证据清单（`.trellis/tasks/09-18-compose-deploy-review/research/findings.md`）
- [x] 定稿 compose 含逐条注释，标注每处改动的实测依据
- [x] 临时容器与临时目录已清理（`docker ps -a` 无残留）
- [ ] journal + 归档

## Out of Scope

- 不改仓库文档（是否把 compose 形态写进 README/MAINTAINING 由用户决定）；
- 不实际部署到 `/opt/docker/komari/`；
- 不涉及 agent 侧 compose。
