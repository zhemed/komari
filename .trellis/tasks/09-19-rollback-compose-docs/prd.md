# 回滚仓库侧 compose 主推地位：systemd 回到推荐路径

## Goal

用户反复要求回滚 compose（"compose 有严重 bug"）。生产已于 2026-09-19 回滚到 systemd 二进制；仓库仍把 compose 当推荐路径（README 方式一 = Docker/compose、MAINTAINING §15 当部署约定、install-compose.sh 无警示）。本次把 systemd 恢复为推荐路径，compose 降级为"备选 + 已知坑"标注。

## Requirements

- TBD

## Acceptance Criteria

- [ ] TBD

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
