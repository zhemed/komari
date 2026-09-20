# 走 C：用干净代码重发 0.0.18 并替换含 compose 的那版

## Goal

用户选择 C：代码已回滚、重新发版替换含 compose 的 0.0.18。要点：① 先查生产容器钉的 tag（决定能否接到新版本）；② 用当前 main（compose 已剔除）构建服务器静态双架构 + agent 14 平台 + 校验和；③ 重新发 0.0.18（覆盖 release 资产与标题说明）；④ 镜像 :0.0.18/:latest 覆盖为干净内容；⑤ 生产按用户确认后重建容器。

## Requirements

- TBD

## Acceptance Criteria

- [ ] TBD

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
