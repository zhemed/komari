# 删除面板里提到 compose 的提示文案（含 5 个语言包 + 重建产物）

## Goal

用户选择'一并删掉这句提示'：从 AdminPanelBar.tsx 移除 upgrade.inplace_hint 的渲染块，并从 5 个语言包精确删除该键（逐行删，不用 json 重写以免缩进噪声），重建前端产物并更新 frontend-build.env 的目录树哈希。socket 重建模式按用户决定保留。

## Requirements

- TBD

## Acceptance Criteria

- [ ] TBD

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
