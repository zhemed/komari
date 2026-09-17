# 执行计划：Trellis 强制闸门

## Step 1 · 消息与任务约定

提交消息统一带 `[task:<slug>]`（slug 即 `.trellis/tasks/<MM-DD>-<slug>` 去掉日期前缀的部分，
例如 `trellis-mandatory-gate`）。审计与 hook 都用这个锚点。

## Step 2 · 写三件套

- [ ] `.githooks/pre-commit`：暂存改动含非 `.trellis/` → 必须有 in_progress 任务
- [ ] `.githooks/commit-msg`：同上条件 → 消息必须含 `[task:<slug>]` 且 slug 存在；merge 放行
- [ ] `scripts/check-trellis-gate.sh`：`--audit-only` 模式 + 本地 hooksPath 检查；审计起点读
      `.trellis/gates/enforce-from`
- [ ] `scripts/install-git-hooks.sh`：设 `core.hooksPath=.githooks` + chmod +x
- [ ] `.trellis/gates/enforce-from`：写入**闸门提交之前的 HEAD**（否则鸡生蛋问题）
- [ ] `.github/workflows/trellis-gate.yml`

## Step 3 · 接入检查与文档

- [ ] `check-repo.sh`：插为第 8 项（原 8/9/10 顺延为 9/10/11），失败时提示安装命令
- [ ] `AGENTS.md`：在 `<!-- TRELLIS:END -->` **之后**追加"强制规则"段（不会被 trellis update 覆盖）
- [ ] `docs/MAINTAINING.md`：新增闸门章节 + §3.4 发版清单加一行
- [ ] `.trellis/spec/guides/trellis-gate-guide.md` + `guides/index.md` 加行

## Step 4 · 证据（在临时克隆里做，不污染主仓库历史）

```bash
git clone /root/komari /tmp/gate-lab && cd /tmp/gate-lab
./scripts/install-git-hooks.sh
# A) 无任务提交 → 拒
# B) 建任务后无 [task:] → 拒
# C) 正确消息 → 过
# D) --no-verify 绕过 → check-trellis-gate.sh --audit-only 报错
# E) 纯 .trellis/ 提交 → 放行
```

## Step 5 · 落地与观察

- [ ] 本仓库 `./scripts/install-git-hooks.sh` + `./scripts/check-repo.sh --full`（11 项）
- [ ] 提交（消息带 `[task:trellis-mandatory-gate]`）→ push → `gh run list` 看 CI 结论
- [ ] journal + archive
