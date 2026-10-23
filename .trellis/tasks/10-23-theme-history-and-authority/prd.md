# 并入主题完整历史 + 标签移动 + 权威来源文档

## 需求

1. 用 `git subtree add --prefix=themes/ink <主题仓库> local/period-status-card` 把主题自身的提交历史并入本仓（此前是普通文件并入，无历史）；
2. 并入后**重新应用**：`komari-theme.json` 版本 → `0.0.1`；删除噪声文件（`.github/`、`AGENTS.md`、`CLAUDE.md`、`AIAGENTREADME.md`、`AICACHE.md`、`.vscode/`），并保持 `.env` 不入库（与并入前一致）；
3. 验证：`./build.sh` 通过（zip + sha256）、`themes/ink` lint 通过、`git status` 干净、`git log -- themes/ink` 能看到主题自身提交、内容树指纹与并入前一致；
4. 把 `theme-ink/v0.0.1`、`server/v0.0.24-own.1` 两个标签移到并入后的 HEAD 并强制推送（仅这两个标签）；记录旧 tag 对象；
5. `ops-docs` 写明：本仓为自维护权威来源；主题不再跟随任何上游；内核可选同步上游的步骤（当前内核 0 处我方改动，冲突风险极低）；
6. 移除本地 `komari-own` 目录里指向已删仓库的 `origin` 远端。

## 验收标准

- `git log --oneline -- themes/ink | head` 出现主题自身提交（179 条历史可达）；
- `git ls-tree -r HEAD themes/ink | git hash-object --stdin` 与并入前指纹一致；
- 构建产物 manifest = 0.0.1；lint 通过；`git status` 干净；
- 远端 `ls-remote --tags` 显示两标签指向新 HEAD；`main` 为 fast-forward（无 force-push 主分支）；
- 内核代码路径未改动；线上未动。

## 非目标

- 不改内核代码；不动线上；不删本地目录；不改写已有历史（仅移动标签）。
