# 重新精简 README（取回 36 行版并落地）

## Goal

回滚后 README 是 94 行版；用户要求重新精简。做法：从恢复分支 pre-rollback-0.0.19 取回当时那版 36 行 README，核对（Docker 命令在首位、compose 零命中、版本号与实际 0.0.18 一致），提交推送；不动其他文件。

## Requirements

- TBD

## Acceptance Criteria

- [ ] TBD

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
