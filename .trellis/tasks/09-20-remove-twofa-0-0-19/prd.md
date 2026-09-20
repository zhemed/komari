# 彻底移除 2FA（两层）+ 发 0.0.19

## Goal

用户选定：账号级 + 敏感操作两层一起删，连带清理 pquerna/otp 依赖、关于页许可清单、i18n 8 键 ×5 语言包、CLI disable-2fa 命令、前端 5 个文件与产物重建；发 0.0.19（:latest 移动）并替换生产。数据库 two_factor 列保留不删（不可逆且无必要）。

## Requirements

- TBD

## Acceptance Criteria

- [ ] TBD

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
