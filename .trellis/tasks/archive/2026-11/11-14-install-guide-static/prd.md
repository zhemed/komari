# 修复安装向导被主题前端抢占（install 模式应强制内置前端）

## Goal

全新实例自动激活 ink 主题后，/install 由当前主题渲染，安装向导打不开（页面不存在 + RPC 404）。让 RunInstallGuide 与 recovery/migration 一致使用 restrictedStatic。

## Requirements

- TBD

## Acceptance Criteria

- [ ] TBD

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
