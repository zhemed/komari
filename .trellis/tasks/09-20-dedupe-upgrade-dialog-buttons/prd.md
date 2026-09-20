# 升级弹窗按钮去重：列表首项与底部按钮重复

## Goal

用户发现别的机器升级时，弹窗里红框的「安装此版本」（列表首项=最新）与底部「立即升级（容器内替换）到 0.0.20」功能重复。任务：读代码确认两者目标版本是否相同；若相同则隐藏列表首项的那个按钮（保留旧版本的「安装此版本」用于回滚/指定版本），评测对回滚路径的影响；出方案等用户确认。

## Requirements

- TBD

## Acceptance Criteria

- [ ] TBD

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
