# 回滚 Dockerfile 基础镜像：digest 钉版 → 回到 alpine:3.21 tag

## Goal

用户指令"取消构建方式，回滚之前的方式"：仓库体检时我把两个 Dockerfile 的基础镜像从 alpine:3.21 钉成了 digest（dc61273），代价是 3.21.x 安全更新不再自动跟进。按指令回滚成 tag，并更正文档里对应的记录。

## Requirements

- TBD

## Acceptance Criteria

- [ ] TBD

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
