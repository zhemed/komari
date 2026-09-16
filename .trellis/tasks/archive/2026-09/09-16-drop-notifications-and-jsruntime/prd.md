# 彻底移除通知系统与 jsruntime

## Goal

删除通知子系统（messageSender/notifier/database-notification/模型/RPC/路由/调度）与随之失去消费者的 pkg/jsruntime（含 goja 依赖）；前端删除 5 个通知页面与菜单路由；发 0.0.3 并更新本地部署

## Requirements

- TBD

## Acceptance Criteria

- [ ] TBD

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
