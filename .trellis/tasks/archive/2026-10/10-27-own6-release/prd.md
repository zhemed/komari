# 发版 0.0.25+own.6 与主题 1.0.4（移除远程控制后的正式发布）

## Goal

按 `ops-docs/VERSION.md` 与 `docs/MAINTAINING.md` §3.4 的发布流程，把「Web 终端/远程执行彻底移除」
的成果正式发布：内核 `0.0.25+own.6`（服务端静态 amd64/arm64 + agent 14 平台 + 两套 ghcr 镜像 +
GitHub release `0.0.25` 18 个资产），主题 `1.0.4`（重建 admin-app + 滚动 release `theme-latest` +
市场目录 sha256 回填 + tag `theme-ink/v1.0.4`）。

## Requirements

1. 版本字面量：`scripts/version.env`（0.0.25 / own.6）、`install-komari.sh` REPO_TAG、
   `install-agent.sh`、`install-agent.ps1`、README/MAINTAINING 当前版本口径、`已发布`区间右端、
   `ops-docs/VERSION.md` 当前值；主题 manifest 1.0.3 → 1.0.4。→ 已改，check-repo 第 1 项全绿。
2. 构建（顺序：**先提交、后构建**，保证二进制里的 git hash 与 tag 一致）：
   - zig 0.16.0 解压到 `.build/tools/`（release 静态构建依赖）；
   - `KOMARI_STATIC=1 KOMARI_OUTPUT=dist/komari-linux-amd64 ./scripts/build-komari.sh`（arm64 同理）；
   - `./scripts/gen-release-sums.sh` → `dist/komari-SHA256SUMS`；
   - `./scripts/build-agent.sh`（14 平台）→ `dist/agent/`，整理 `komari-agent-SHA256SUMS`；
   - 主题：`./build.sh --skip-server --skip-install` → `ops-dist/theme/ink-build-<sha>.zip` + sha256。
3. 镜像：`gh auth token | docker login ghcr.io`；`./scripts/build-server-image.sh --push`、
   `./scripts/build-agent-image.sh --push`（`:0.0.25` 与 `:latest`）。
4. 发布：commit + push（含 5 个既有未推提交）；tag `0.0.25`、`server/v0.0.25-own.6`、
   `theme-ink/v1.0.4`、滚动 `theme-latest` 指向新主题提交；`gh release create 0.0.25`（18 资产 + notes）；
   覆盖上传 `theme-latest` 的 `ink-theme.zip` 与 `.sha256`（prerelease 硬约束）。
5. 市场目录：`deploy/theme-market/v1.json` 更新 version=1.0.4 与 sha256，提交。
6. `./scripts/check-repo.sh --full` 全绿后发布；发布后验证资产/镜像可获取。

## Constraints

- 站点专属值不入库；release notes 写入 `/tmp`（不提交）。
- 不重写历史；提交带 `[task:own6-release]` 锚点。
- 网络动作范围：GitHub 本仓（push/release/asset）、ghcr 本仓镜像、ziglang.org 下载 zig（发版构建依赖）。
- 生产部署与 DB 清理由主仓任务 `10-27-komari-own6-release-cleanup` 执行并记录。

## Acceptance Criteria

- [ ] 版本字面量全仓一致，check-repo 第 1/14 项通过。
- [ ] `dist/` 含 2 个静态服务端二进制 + 校验和；`dist/agent/` 含 14 平台 + 校验和；主题 zip + sha256。
- [ ] `check-repo.sh --full` 全绿。
- [ ] ghcr 镜像两套推送成功（0.0.25 + latest，可匿名拉取验证）。
- [ ] GitHub release 0.0.25 存在且 18 个资产齐全；`theme-latest` 资产更新；tag 全部推送。
- [ ] 市场目录 v1.json 与发布资产 sha256 一致并已提交。
- [ ] 工作区 CLEAN；journal + 任务归档完成。

## 验收结果（2026-10-27 回填）

- [x] 版本字面量一致：check-repo 第 1 项全绿（0.0.25 / own.6 / REPO_TAG / agent pin / 文档口径 / 已发布区间）。
- [x] 产物：2 个静态服务端二进制 + `komari-SHA256SUMS`；14 平台 agent + `komari-agent-SHA256SUMS`；
      主题 zip + sha256（见 `findings.md` 第三节）。
- [x] `check-repo.sh --full` 16 项全部通过。
- [x] ghcr 两套镜像 0.0.25 + latest 推送成功，匿名 manifest 200。
- [x] GitHub Release `0.0.25`：18 资产齐全、Latest；`theme-latest` 资产更新（1.0.4）、市场目录 sha 一致。
- [x] tag 推送：`0.0.25`、`server/v0.0.25-own.6`、`theme-ink/v1.0.4`、`theme-latest`。
- [x] 工作区 CLEAN；journal + 任务归档完成（本任务）。
