# install-compose.sh：备份文件提示 + 保留最近 3 份

## Goal

用户建议（可选）：--force 的 .bak-<时间戳> 会累积。落地两种做法：成功摘要里加"确认无误后可删 .bak-*"的提示并显示份数；同时自动清理较旧的备份——但只清我们自己生成的 compose.yaml.bak-*（保留最近 3 份），历史名备份（用户原文件）永不自动删。

## Requirements

- TBD

## Acceptance Criteria

- [ ] TBD

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
