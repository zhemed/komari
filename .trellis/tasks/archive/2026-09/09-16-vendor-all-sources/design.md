# 设计：源码 vendor 化

## 1. 目录与来源

| 目录 | 来源 | 处理 |
|---|---|---|
| `frontend/` | 上游 `komari-web@4a74e8a8` 全量（去掉 `.github/`） | 打上我们 0001–0006 补丁后的内容直接入库 |
| `agent/` | 上游 `komari-agent@1186aafb` 全量（去掉 `.github/`、`install.sh`、`install.ps1`） | 打上我们 0001–0003 补丁后的内容直接入库 |

- `agent/install.sh` / `install.ps1` **不入 `agent/`**：它们的成品（我们改过的版本）在仓库根
  `install-agent.sh` / `install-agent.ps1`，前端的安装命令 URL 直指根路径；放两份会造成双源。
- 导入方式：`git ls-files` + 复制工作区内容（快照），**不带上游历史**（历史策略见 MAINTAINING §10）。
- 溯源：上游仓库 + commit 记录在 `scripts/frontend-pin.env` / `scripts/agent-pin.env` 的注释里，
  导入提交信息里写明"import upstream <repo>@<commit> with our patches folded in"。

## 2. 构建路径改造

| 旧 | 新 |
|---|---|
| `build-agent.sh`：clone 上游 → checkout pin → apply 3 个补丁 → 校验源码树哈希 → 安装脚本回放比对 → 构建 | `build-agent.sh`：直接在 `agent/` 构建；保留版本一致性 + 过滤/自更新目标断言；新增 `-buildvcs=false`（agent 的身份是我们注入的版本号，不该带本仓库的提交信息） |
| `sync-frontend.sh`：clone 上游 → checkout pin → apply 6 个补丁 → npm ci → build → 注入 | `build-frontend.sh`：在 `frontend/` 里 npm ci → build（`SOURCE_DATE_EPOCH` 用记录的上游 commit 时间，保持产物哈希稳定）→ 注入 `web/public/defaultTheme/` → 校验目录树哈希与上游 URL 残留 |

- 前端构建产物（`frontend/dist`、`node_modules`）不进仓库：`.gitignore` 补
  `frontend/node_modules/`、`frontend/dist/`、`frontend/.vite/`、`agent/build/`。
- `web/public/defaultTheme/` 仍是**提交进仓库的产物**（`//go:embed` 需要它，且离线构建依赖它）。

## 3. 等价性证明（本次改造的关键验收）

1. **agent**：在老路径（`.build/agent-src`，pin+补丁）与新路径（`agent/`）分别用
   `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -buildvcs=false -ldflags
   "-X .../update.CurrentVersion=0.0.5 -X .../update.Repo=zhemed/komari"` 构建 → `sha256sum` 必须相同；
   同时与 0.0.5 release 资产比对（该资产是旧路径、默认 `-buildvcs` 产出，若因 VCS 信息不同而不等，
   则在报告里说明差异原因，并证明两者差异仅来自 buildvcs）。
2. **前端**：新路径重建 → 目录树哈希必须仍为 `af0bd793…`（与当前提交里的产物一致）。

## 4. 风险与回滚

- 风险：导入时漏文件导致构建失败（用`go build ./...`、`npm run build`、两个哈希门禁兜底）。
- 风险：`.gitignore` 误伤 vendored 源码（导入后跑 `git status`，确认源码全部被跟踪）。
- 回滚：整个改造是若干个提交，`git revert` 即可；补丁系列在 revert 后回来，`build-*.sh` 同步恢复。
