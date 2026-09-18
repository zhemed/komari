# install-compose.sh 三处改进（--no-start 跳过预检 / --force 行为与文案一致 / 迁移提示补重建说明）

## Goal

用户点名三处改进：① --no-start 应跳过同名容器预检（干跑对比用）；② 历史文件的错误文案与 --force 实际行为不一致（现在 --force 会写出 compose.yaml 并让两个文件名并存）——要改成移开历史文件、只留一个；③ 迁移提示补一句"会触发一次重建"（并给 --force-recreate 的正确命令）。改完再验一轮。

## Requirements

- TBD

## Acceptance Criteria

- [ ] TBD

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
