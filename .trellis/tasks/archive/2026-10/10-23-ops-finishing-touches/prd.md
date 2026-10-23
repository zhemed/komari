# 收尾：标签 / 文档 / 清理 / 删除归档仓库

## 需求

1. 在 `zhemed/komari` 上补打并推送发行标签 `theme-ink/v0.0.1`、`server/v0.0.24-own.1`（指向 `65ca84e`）；
2. 把两条运维经验写进 `ops-docs/BUILD.md`：GitHub HTTPS 首推 408 的调优与重试写法；提交闸门 `[task:<slug>]` 规则与 `.githooks` 单次覆盖法；
3. 清理 `themes/ink/` 中上游自带的 agent/CI 噪声文件（`.github/`、`AGENTS.md`、`CLAUDE.md`、`AIAGENTREADME.md`、`AICACHE.md`、`.vscode/`），**删后必须验证构建**；
4. 删除已归档的 `zhemed/komari-own`：先 `git bundle --all` 存 `/root/backups/` 并 `bundle verify`，再 `gh repo delete`，最后确认 404。

## 验收标准

- `git ls-remote --tags origin` 能看到两个标签；
- 文档已入库并推送；README 有指引；
- `./build.sh` 在删文件后仍产出 `ops-dist/theme/ink-build-<sha>.zip` 与 `.sha256`（manifest 0.0.1）；
- bundle 校验通过；`gh api repos/zhemed/komari-own` 返回 404；
- 内核代码路径未改动；线上未动。

## 非目标

- 不改内核代码；不动线上；不删本地目录（`komari-own` 本地保留）。
