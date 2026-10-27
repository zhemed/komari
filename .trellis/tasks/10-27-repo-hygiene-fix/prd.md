# 修复仓库自检既有失败（.env / hooks / 历史锚点审计）

## Goal

修复 `scripts/check-repo.sh` 的既有失败，让仓库自检恢复绿色；只做仓库卫生与流程闸门治理，
不改产品功能、不改线上、不重写 git 历史。

## Requirements

1. **`.env` 不该入库**：`themes/ink/.env` 从 git 索引移除（本地磁盘文件保留）；
   提供可入库的 `.env.example` 作为默认值说明（代码已有 `|| ''` / `|| '/api'` 兜底，移除后行为不变）；
   在 `themes/ink/.gitignore` 显式忽略 `.env`（根 `.gitignore` 已有 `.env` 规则，双保险）。
2. **hooks 未安装**：运行 `./scripts/install-git-hooks.sh` 把本地 `core.hooksPath` 指向 `.githooks`；
   确认 pre-commit / commit-msg 可执行，后续提交走本仓闸门。
3. **历史锚点审计失败**：`scripts/check-trellis-gate.sh` 的审计起点 `.trellis/gates/enforce-from`
   仍停在旧 SHA，导致经由 subtree 并入的上游/主题历史（177 个无锚点提交）被审计报红。
   把基线前移到「外部历史最后落地提交」`b34d353`（`Add 'themes/ink/' from commit 2d00c65`，
   其之后 20 个改动提交全部带锚点、审计 0 违规），**不重写历史**。
4. 在 `scripts/check-trellis-gate.sh` 头部与 `.trellis/spec/guides/trellis-gate-guide.md` 写清
   「基线前移」规则与禁止事项，避免后人误以为可以随手放宽闸门。
5. 验证：
   - `./scripts/check-trellis-gate.sh` 通过（hooks 已装 + 基线后 0 违规）；
   - `./scripts/check-repo.sh` 快速模式 0 失败；
   - 判别性测试：构造「基线后、无任务锚点」的临时提交（detached HEAD，不落分支/不入库），
     审计脚本必须报红；验证后恢复 main，确保工作区干净。

## Constraints

- 不重写历史（禁止 rebase -i / filter-branch / 强推）。
- 不使用 `git commit --no-verify` 绕过闸门；判别性测试用 `git commit-tree` + detached HEAD 构造，不跑 hooks。
- 不改产品代码/线上；不联网。
- 提交带 `[task:repo-hygiene-fix]` 锚点。

## Acceptance Criteria

- [ ] `git ls-files themes/ink/.env` 为空；`themes/ink/.env.example` 存在；`.gitignore` 含 `.env`。
- [ ] `git config --get core.hooksPath` = `.githooks`；两个 hook 可执行。
- [ ] `.trellis/gates/enforce-from` = `b34d353`；`check-trellis-gate.sh` 通过（含本地 hooks 检查）。
- [ ] `./scripts/check-repo.sh` 快速模式 0 失败。
- [ ] 判别性测试：临时无锚点提交被审计判红；测试后 HEAD 回到 main、工作区 clean。
- [ ] 变更提交完成（komari-src），主仓 Trellis 归档 + journal 提交完成。

## Notes

- 轻量任务：PRD-only。
- 上一轮授权来源：主仓任务 `10-27-komari-repo-hygiene-fix` 的 directive（用户原话「授权第五项未做与风险，开始维护」）。
