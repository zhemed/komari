# 回滚提交：main 回到 0.0.18 状态（662864a）

## Goal

用户要求全部回滚。仓库文件树回到 662864a（0.0.18 状态：README 94 行、Docker 主方案、compose 已剔除、无 0.0.19）。方式：以 662864a 的文件树提交一个回滚提交（不 rewrite 历史），保留 pre-rollback-0.0.19 标签。随后删 release 0.0.19 与 tag、镜像 :latest 退到 0.0.18。生产不动。

## Requirements

- TBD

## Acceptance Criteria

- [ ] TBD

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
