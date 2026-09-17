<!-- TRELLIS:START -->
# Trellis Instructions

These instructions are for AI assistants working in this project.

This project is managed by Trellis. The working knowledge you need lives under `.trellis/`:

- `.trellis/workflow.md` — development phases, when to create tasks, skill routing
- `.trellis/spec/` — package- and layer-scoped coding guidelines (read before writing code in a given layer)
- `.trellis/workspace/` — per-developer journals and session traces
- `.trellis/tasks/` — active and archived tasks (PRDs, research, jsonl context)

If a Trellis command is available on your platform (e.g. `/trellis:finish-work`, `/trellis:continue`), prefer it over manual steps. Not every platform exposes every command.

If you're using Codex or another agent-capable tool, additional project-scoped helpers may live in:
- `.agents/skills/` — reusable Trellis skills
- `.codex/agents/` — optional custom subagents

Managed by Trellis. Edits outside this block are preserved; edits inside may be overwritten by a future `trellis update`.

<!-- TRELLIS:END -->

<!-- MANDATORY-RULES:START 用户 2026-09-17 定调的强制规则；本段在托管块之外，不会被 trellis update 覆盖 -->

# 强制规则：Trellis 闸门（不可协商）

**调用 Trellis 不是口头承诺，是强制规则。** 用户 2026-09-17 明确要求，适用范围**最严**：
**任何会话工作——包括只读调查（看代码、查日志、读库定位原因）——开工第一步必须是建 Trellis 任务。**

1. **先建任务，再动手**：
   ```bash
   python3 .trellis/scripts/task.py create "<标题>" --slug <slug> -d "<描述>"
   python3 .trellis/scripts/task.py start <MM-DD-slug>
   ```
   唯一例外：用户明确说"这次跳过 Trellis"。没有第二条例外。
2. **提交必须绑定任务**：提交消息带 `[task:<slug>]`（slug 见上）。改动非 `.trellis/` 文件而
   没有进行中的任务，提交会被 `.githooks/pre-commit` 与 `commit-msg` **当场拒绝**。
3. **收尾必须归档**：`task.py finish` → `task.py archive <name> --skip-branch-validation`，
   并写 journal（`.trellis/workspace/`）。
4. **绕过会被抓**：`git commit --no-verify` 能绕过本地 hook，但 `scripts/check-repo.sh` 第 8 项
   与 GitHub Actions `trellis-gate` 会审计出来（第二次检查必然暴露，且阻断发版）。
   不许把 `--no-verify` 当常规手段。
5. **诚实边界**：机器能强制"提交前有任务"；"只读调查也开任务"没有可审计产物，靠本规则 +
   journal 留痕。不要假装它是自动的。

细节：`.trellis/spec/guides/trellis-gate-guide.md`、`docs/MAINTAINING.md`「流程闸门」。

<!-- MANDATORY-RULES:END -->
