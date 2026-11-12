# 三网行丢包条去掉斜纹纹理（与延迟条一致）

## Goal

用户 2026-11-12 反馈：生产节点卡片右侧丢包条出现黄/蓝斜纹，与左侧延迟条（实线色块）不一致，要求右边和左边一样。定位：buildTaskHistoryBars 里延迟条传 pattern:false，丢包条调用 getLossToneClass 未传 pattern，色觉友好/默认纹理生效。改为丢包条也走实线色块；重建主题、发 theme-latest、更新本机与生产主题。

## Requirements

- TBD

## Acceptance Criteria

- [ ] TBD

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
