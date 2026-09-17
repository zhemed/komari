# 强制规则扩展到全局（~/.dsh/AGENTS.md）

## Goal

用户 2026-09-17 确认：把 komari 里已落地的 Trellis 强制规则推广到**本机所有启用 Trellis 的项目**
（判定标准沿用全局约定：项目根存在 `.trellis/`）。

## 要求

- **R1 写在托管块之外**：`~/.dsh/AGENTS.md` 已有 `TRELLIS-GLOBAL` 与 `BROWSER-CDP` 两个标记块；
  新规则用独立标记块 `TRELLIS-MANDATORY:START/END`，插在 TRELLIS-GLOBAL 之后，避免被 trellis 更新覆盖。
- **R2 只在有 `.trellis/` 的项目生效**：不得让没有 Trellis 的项目被要求建任务（沿用既有判定标准）。
- **R3 内容与项目版一致**：开工先建任务（含只读调查）→ 提交带 `[task:<slug>]` → 收尾 finish/archive/journal；
  禁止把 `--no-verify` 当常规手段；项目自带闸门优先（可能更严）。
- **R4 诚实边界照写**：只读调查没有可审计产物，机器只拦"提交时没任务"，不许声称全自动。
- **R5 本仓库留痕**：`.trellis/spec/guides/trellis-gate-guide.md` 记一条"已推广到全局"，
  提交带 `[task:global-trellis-rule]`。

## 验收

- [ ] `~/.dsh/AGENTS.md` 含 `TRELLIS-MANDATORY` 段，且两个既有托管块内容**未被改动**
- [ ] 段内明确"仅适用于项目根有 `.trellis/` 的项目"
- [ ] komari 指南更新并提交（消息带锚点），`check-repo.sh` 通过
- [ ] journal + 归档

## Out of Scope

- 不给其它项目安装闸门脚本（各项目自选）；不建新的 git 仓库来管理 `~/.dsh`。
