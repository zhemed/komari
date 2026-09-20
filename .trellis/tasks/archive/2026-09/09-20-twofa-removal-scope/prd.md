# 排查 2FA：范围盘点与彻底移除可行性

## Goal

用户问：查看账号的 2FA，相关所有内容能否彻底移除。任务：① 只读盘点 2FA 的全部落点（后端 RPC/模型/存储、前端页面与 i18n、数据库字段、登录流程与 OAuth/OIDC 交互、依赖库、安装/文档）；② 列出移除的连带影响与风险（已绑定的用户会不会被锁、有没有别的功能依赖 TOTP）；③ 给出可执行的移除清单与步骤（含数据库字段处理与前端产物重建），不改代码——先出方案等用户拍板。

## Requirements

- TBD

## Acceptance Criteria

- [ ] TBD

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
