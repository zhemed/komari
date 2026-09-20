# 移除面板「文档」入口（菜单 + 帮助按钮）并发 0.0.20

## Goal

用户选定：只清面板两处（底部菜单「文档」+ 设置页「帮助」按钮）与其 i18n 键，不动 agent 与维护文档；发 0.0.20。步骤：改 menuConfig.json 与 general.tsx、删 common.documentation 与 common.help（5 语言包）、重建前端产物并更新哈希、升版本字面量、构建服务器静态双架构 + agent 14 平台 + 校验和、发 release、推镜像、升级生产并验收。

## Requirements

- TBD

## Acceptance Criteria

- [ ] TBD

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
