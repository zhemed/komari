# 回滚文件名：compose.yaml 改回 docker-compose.yml（含生产与文档）

## Goal

用户明确指令：compose.yaml 改回 docker-compose.yml——改名引入了严重 bug（改名后容器 config_files 标签仍指旧路径、且不带 --force-recreate 时 up -d 不会重建，导致 0.0.18 的 compose tag 自动同步静默失效；旧版安装脚本用户目录被当"历史名"拒绝）。回滚范围：install-compose.sh、生产 /opt/docker/komari、文档 §15.1/§15.6、测试 fixture。

## Requirements

- TBD

## Acceptance Criteria

- [ ] TBD

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
