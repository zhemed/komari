# 强制规则：把 Trellis 变成机器能拦住的闸门

## Goal

用户 2026-09-17 定调：**调用 Trellis 不是口头承诺，而是强制规则**。用户已选择最严范围
（连只读调查也要先建任务）+ 本地双闸 + GitHub CI 兜底。

现状（取证）：仓库 `.githooks/` 不存在、`core.hooksPath` 未设置、`.github/workflows/` 不存在、
`scripts/check-repo.sh` 10 项全是产物/代码检查，**没有一项检查流程合规** —— 跳过 Trellis
在机器层面零成本。本任务把"规则"变成"提交时被拒 + 事后可审计 + GitHub 上标红"。

## 三层闸门（用户选定）

| 层 | 位置 | 拦什么 | 能否绕过 |
|---|---|---|---|
| 第一层：当场拦 | `.githooks/pre-commit` + `commit-msg`（`core.hooksPath=.githooks`） | 有非 `.trellis/` 改动却无进行中任务；消息里没有 `[task:<slug>]` | `--no-verify` 可绕过（git 内建，无法真正封死） |
| 第二层：事后审计 | `scripts/check-trellis-gate.sh`，被 `check-repo.sh` 第 8 项调用 | 逐个提交核对"改动非 `.trellis/` 就必须带 `[task:…]`"，含被 `--no-verify` 绕过的 | 无法绕过：检查一次就暴露，且发版前必须跑 |
| 第三层：远程兜底 | `.github/workflows/trellis-gate.yml`（push/PR） | 同上审计，违规提交在 GitHub 上直接标红 | 无法绕过（除非删工作流，删了也是可见动作） |

## Requirements

- **R1 规则本身（AGENTS.md）**：在 Trellis 托管块**之外**写死硬规则（托管块会被 `trellis update`
  覆盖）：任何会话工作——**包括只读调查**——先 `task.py create`；用户明确说"跳过"才例外。
- **R2 hook 语义**：
  - `pre-commit`：暂存区**含非 `.trellis/` 改动**时，必须存在 `status=in_progress` 的活动任务，
    否则拒绝并打印可直接复制的建任务命令；纯 `.trellis/` 提交（journal/archive）放行。
  - `commit-msg`：同上条件时，消息必须含 `[task:<slug>]`，且 slug 对应真实任务目录；
    merge 提交放行。
- **R3 审计脚本**：`scripts/check-trellis-gate.sh` 支持 `--audit-only`（CI 用，跳过本地 hooksPath 检查），
  审计范围 = `.trellis/gates/enforce-from` 里的起点 SHA 到 HEAD（不含 merge，排除纯 `.trellis/` 提交）。
- **R4 一键安装**：`scripts/install-git-hooks.sh` 设置 `core.hooksPath=.githooks` 并补执行位；
  `check-repo.sh` 第 8 项在未安装时报错并给出该命令。
- **R5 CI**：`.github/workflows/trellis-gate.yml`，`actions/checkout` 用 `fetch-depth: 0`，
  跑 `./scripts/check-trellis-gate.sh --audit-only`。
- **R6 文档**：`docs/MAINTAINING.md` 增闸门章节（规则、三层、绕过与后果、消息格式、
  新克隆要跑一次安装脚本）；`.trellis/spec/guides/` 增指南并挂到索引。
- **R7 诚实边界**：机器只能强制"提交前必须有任务"；"只读调查也开任务"没有可审计的产物，
  靠 AGENTS.md + journal 留痕，文档必须写明这一点，不许假装全自动。

## Acceptance Criteria（全部要实测，不留"应该行"）

- [ ] 临时克隆 A：无活动任务改文件提交 → 被 `pre-commit` 拒绝（exit≠0，提示建任务命令）
- [ ] 临时克隆 A：建任务后消息不带 `[task:slug]` → 被 `commit-msg` 拒绝
- [ ] 临时克隆 A：带 `[task:slug]` 且 slug 存在 → 提交成功
- [ ] 临时克隆 A：`--no-verify` 绕过成功提交 → `check-trellis-gate.sh --audit-only`
      报出该提交并 exit≠0（证明第二层兜住第一层）
- [ ] 临时克隆 A：纯 `.trellis/` 提交不被 hook 打扰
- [ ] 本地 `core.hooksPath` 未设置时 `check-repo.sh` 第 8 项失败；`install-git-hooks.sh` 后通过
- [ ] 本仓库 `check-repo.sh --full` 11 项全绿
- [ ] push 后 GitHub Actions 该工作流真实运行且结论为 success（`gh run list` 取证）

## Out of Scope

- 不改服务端/前端行为，不发版（仓库工具链与文档）；
- 不试图封死 `--no-verify`（git 设计如此），改为"绕过必留痕"；
- 不动全局 `~/.dsh/AGENTS.md`（如用户要全机器生效，另开任务）。
