# install-compose.sh：备份名同秒撞名（唯一后缀 + 无 sleep 回归测试）

## Goal

用户实测（可选改进，但属真 bug）：同一秒内连续 --force 时，备份名只到秒 → 后一次把前一次同名备份覆盖，4 次运行后只剩 1 份，"只保留最近 3 份"实际失效（脚本/CI 高频调用时）。修法：备份名加纳秒（不支持的平台回退 PID+RANDOM）保证唯一；并把回归测试改成无间隔连跑。

## Requirements

- TBD

## Acceptance Criteria

- [ ] TBD

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
