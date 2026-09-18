# 实际部署：切换到 /opt/docker/komari（compose + 最新版 0.0.17）

## Goal

用户授权：推送本地提交，并按最新版本（0.0.17）在 /opt/docker/komari/ 实际部署 compose 形态。需先摸清当前生产形态（systemd? docker?）与数据位置，做数据备份后迁移，切换时保证面板地址与数据不变、可回滚。

## Requirements

- TBD

## Acceptance Criteria

- [ ] TBD

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
