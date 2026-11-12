# 发布 0.0.26（内置主题 + 主题商店移除 + 三网双条 + 主题 URL 修复）

## Goal

把本轮改动随服务端 0.0.26 正式发布：ink 主题内置进二进制（首次启动自动安装、新库默认激活）、
主题商店移除、三网行双条版式、主题更新 URL 误判修复。按 `docs/MAINTAINING.md` §3.4 手动执行整套发布。

## 发布内容（相对 0.0.25）

| 改动 | 提交 |
|---|---|
| ink 主题内置进二进制（`web/public/builtinThemes/ink.zip` + 首次启动安装/激活） | `24d0ec1` |
| 主题商店移除（后端市场 API/目录 + 后台 UI 入口） | `446708d` |
| ink 三网行双条版式（延迟条 + 丢包条） | `23f0dca` |
| 主题导入/更新 GitHub URL 误判修复（release 下载地址被当仓库） | `4a0632b` |

## 版本

- 内核：`0.0.25` → **`0.0.26`**（`own.6` → `own.7`）
- 主题：`1.0.4` → **`1.0.5`**（内置包与 `theme-latest` 同步）
- 字面量同步：`scripts/version.env`、`install-komari.sh`、`install-agent.sh`、`install-agent.ps1`、
  README、MAINTAINING（当前版本/已发布区间/本次发版说明）、ops-docs/VERSION、DEPLOY、主题 manifest

## 发布步骤（照 §3.4）

1. [x] 版本字面量同步 + 主题 1.0.5 重建（`build.sh --skip-server --skip-install`）
2. [x] 提交版本 bump（先提交后构建，保证二进制内 VCS hash 与 tag 一致）
3. [ ] 静态服务器产物：`KOMARI_STATIC=1 KOMARI_OUTPUT=dist/komari-linux-amd64 ./scripts/build-komari.sh`
4. [ ] agent 产物：`./scripts/build-agent.sh`；校验和：`./scripts/gen-release-sums.sh`
5. [ ] `./scripts/check-repo.sh --full` 全绿
6. [ ] 打 tag `0.0.26` / `server/v0.0.26-own.7` / `theme-ink/v1.0.5` 并 push
7. [ ] GitHub release `0.0.26`（4 资产）+ `theme-latest` 滚动更新（ink-theme.zip 1.0.5）
8. [ ] 镜像推送 ghcr（linux/amd64）
9. [ ] 发布后核验（资产下载/校验和、tag、镜像、theme-latest）

## 边界

- **生产服务器不动**：本次只发布（GitHub release + 镜像 + theme-latest），部署由用户另行决定。
- 只维护 linux/amd64（2026-11-12 定稿），4 个 release 资产。
- 主题市场目录已删除，发布时不再回填 catalog。

## Acceptance Criteria

- [x] GitHub release `0.0.26` 存在，4 个资产齐全且 `komari-SHA256SUMS` 校验通过。
- [x] `theme-latest` prerelease 的 `ink-theme.zip` = 主题 1.0.5 构建产物（sha256 一致）。
- [x] `ghcr.io/zhemed/komari:0.0.26` 与 `komari-agent:0.0.26` 可匿名拉取（linux/amd64）。
- [x] 发布二进制 `/api/version` 报 `0.0.26` 且 hash 与 tag 指向的提交一致。
- [x] `check-repo.sh --full` 17 项全绿；生产未动。
