# 构建 0.0.19 发布资产（服务器静态 + agent 14 平台 + 校验和）

## Goal

用户：构建 0.0.19。任务：① 同步 4 处版本字面量到 0.0.19（含文档与 build-agent 注释）；② 准备 zig（上次清理删了）或确认可行的静态构建路径；③ 构建服务器 amd64/arm64 发布产物 + agent 14 平台 + SHA256SUMS；④ 对资产做验收（size/静态性/--help 版本行）；⑤ 自检 --full；⑥ 写 0.0.19 发布说明。是否发布（tag/release/推镜像）另行确认。

## Requirements

- TBD

## Acceptance Criteria

- [ ] TBD

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
