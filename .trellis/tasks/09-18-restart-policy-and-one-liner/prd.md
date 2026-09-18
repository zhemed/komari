# 重启策略评估 + 一条命令部署

## Goal

用户要求两件事：① 评估 docker 重启策略 `unless-stopped` 与 `always` 的取舍；② 把
`/opt/docker/komari/docker-compose.yml` 那套部署做成"一条命令"（复制粘贴即部署）。

## Requirements

- **R1** 重启策略评估必须有证据：我们代码里哪些路径依赖重启策略、重建容器后策略是否保留、
  实测退出后是否被拉起；Docker 既定语义（手工 stop 后的差别）如未实测必须标注。
- **R2** 一条命令部署：`install-compose.sh`（`curl | bash` 形态），建目录 → 写 compose
  （与 §15.2 定稿一致）→ `up -d` → 等 healthy → 打印用法。
- **R3** 保护性：已存在 compose 文件时拒绝覆盖（`--force` 除外）；`./data` 永不删除/覆盖；
  同名容器冲突要在启动前拦下并给出解法。
- **R4** 接入自检：`check-repo` 校验该脚本的版本字面量与语法；文档（README/MAINTAINING）同步。
- **R5** 全流程遵守强制规则（建任务 → start → 提交带锚点 → journal → 归档）。

## Acceptance Criteria

- [x] 重启策略对照表（含实测条目与"未实测"标注），定稿 `unless-stopped`
- [x] `install-compose.sh` E2E：临时目录 + 端口映射 + `--no-socket` → 容器 healthy；重跑被拒；撞名被拦
- [x] `check-repo` 能拦住脚本版本写错（判别性验证）
- [x] `./scripts/check-repo.sh --full` 全绿
- [x] journal + 归档（**补记：首次提交被闸门拦下——任务只 create 没 start**）

## Out of Scope

- 不改部署形态本身（host 网络、socket、日志上限等沿用 §15 定稿）；
- 不发版（0.0.18 待用户点头）。
