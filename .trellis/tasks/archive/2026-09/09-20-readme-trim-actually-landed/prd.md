# 修：README 删减未落盘（还原覆盖）并重新提交

## Goal

事故：做判别性验证时用 cp 备份+截断 README，随后 cp 还原把删减覆盖；提交 9fb8a86 因此不含 README。任务：重新写 README（Docker 主方案、compose 零出现、145→~94 行）、确认落盘、用临时文件而非真文件做验证、提交推送。

## Requirements

- TBD

## Acceptance Criteria

- [ ] TBD

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
