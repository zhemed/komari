# 解除内网地址限制并移除 HTTPS 警告横幅

## Goal

后端 SSRF 防护改为默认允许私网/内网地址（保留 KOMARI_BLOCK_PRIVATE_ENDPOINTS=1 可重新开启）；前端移除 warn_https 红色横幅（3 处）；重出 0.0.2 并更新本地部署

## Requirements

- TBD

## Acceptance Criteria

- [ ] TBD

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
