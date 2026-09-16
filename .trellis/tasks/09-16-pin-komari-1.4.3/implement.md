# 执行计划：锁定并维护 komari 1.4.3 分叉基线

对应任务：`.trellis/tasks/09-16-pin-komari-1.4.3`（需求见 `prd.md`，设计见 `design.md`）。

## 执行前提（Gate 0）

- 已获用户对最终规划摘要的**明确批准**后，才执行 `task.py start` 进入本计划。
- 本任务未经 `task.py create` 播种 `implement.jsonl` / `check.jsonl`（任务目录内仅有 `prd.md` 与 `task.json`），故不涉及 JSONL 门禁；Phase 2 的规范上下文通过 `trellis-before-dev` 加载。
- 环境事实：本机 `go1.26.6` / `node v22.23.2` / `npm 10.9.8` / `gcc 11.4.0`；沙箱下 npm/GOPATH/GOCACHE 必须重定向到工作区（`.build/`），用户自己的 shell 无此限制。

## 实施清单（按序）

### 1. 忽略构建缓存

- 动作：`.gitignore` 追加 `.build/`（当前 767MB，含 GOPATH/GOCACHE/npm cache/运行数据）。
- 校验：`git status --short | grep -c '^?? .build/'` 结果为 0。
- 回滚：单行删除。

### 2. 前端更新检查补丁

- 动作：新建 `scripts/patches/0001-update-check-repo.patch`，把 `src/components/admin/AdminPanelBar.tsx:263` 的硬编码 `https://api.github.com/repos/komari-monitor/komari/releases?per_page=100` 改为按 `import.meta.env.VITE_KOMARI_UPDATE_REPO` 拼接，默认值 `"zhemed/komari"`。
- 校验：在 komari-web@4a74e8a8 检出上 `git apply --check` 通过；`grep -rn "komari-monitor/komari/releases" src/` 无结果。
- 回滚：删除 patch 文件。

### 3. 前端再生成脚本

- 动作：新建 `scripts/sync-frontend.sh`。行为：把 `komari-web` 克隆/检出到 pin commit `4a74e8a81e2e4b1c3da8ad795f9523151efb6b56` → `git apply` 补丁 → `npm ci` → `npm run build` → 用 rsync/rm 保证**干净替换** `web/public/defaultTheme/`（`dist/`、`komari-theme.json`、`preview.png`）→ 计算并校验产物哈希。
- 校验：脚本可重复执行；第二次执行的哈希与第一次一致（AC5）；执行后 `web/public/defaultTheme/dist/index.html` 存在。
- 回滚：删除脚本 + `git checkout -- web/public/defaultTheme`（若已入库）。

### 4. 后端构建脚本

- 动作：新建 `scripts/build-komari.sh`。行为：校验 `web/public/defaultTheme/dist/index.html` 存在（否则给出与 `public.go:130` panic 对应的清晰报错）→ `go build -trimpath -ldflags="-s -w -X github.com/komari-monitor/komari/utils.CurrentVersion=1.4.3 -X github.com/komari-monitor/komari/utils.VersionHash=$(git rev-parse HEAD)"` → 输出到 `bin/komari`。
- 校验：脚本第 2 次运行可覆盖输出且成功。
- 回滚：删除脚本。

### 5. 生成并 vendor 主题产物

- 动作：运行 `scripts/sync-frontend.sh`，生成 `web/public/defaultTheme/`（预期 `dist` ≈6.7MB，含 446 条 PWA 预缓存）。
- 校验：`git check-ignore -v web/public/defaultTheme` **不命中**（确认可入库）；`du -sh web/public/defaultTheme`；`grep -rl "komari-monitor/komari/releases" web/public/defaultTheme/` 无结果（AC4 的产物侧证据）。
- 回滚：`rm -rf web/public/defaultTheme`。

### 6. 安装脚本去上游化

- 动作：改 `install-komari.sh:36` 的 `REPO` 指向自有仓库；处理 `:291` 的 `releases/latest` 语义，使其不再静默安装上游 1.5.x（无自有 release 时明确失败）。
- 校验：`bash -n install-komari.sh` 语法通过；`grep -n "komari-monitor/komari" install-komari.sh` 不再出现在下载路径上。
- 回滚：`git checkout -- install-komari.sh`。

### 7. 文档与来源说明

- 动作：
  - 新建 `docs/MAINTAINING-1.4.3.md`：记录后端 commit `bf6b45ec…`、前端 commit `4a74e8a8…`、工具链基线（Go ≥1.25、Node 22/23、`npm ci`、gcc/cgo）、构建与再生成命令、发版号规则（补丁版必须递增三段 patch 位）、"不跟上游 1.5.x / 安全修复需自行 backport"的运维提醒。
  - `README.md` 顶部加简短 fork 说明并指向上述文档，避免被误认为上游官方版本。
- 校验：文档内所有 commit 与命令与实际一致（逐条对照 `git rev-parse`）。
- 回滚：删除文档 / `git checkout -- README.md`。

### 8. 端到端验证（AC 全覆盖）

1. **AC1 离线构建**：`env GOPROXY=off GOFLAGS=-mod=mod ./scripts/build-komari.sh`（在 `GOPATH`/`GOCACHE` 已预热的条件下验证"不需要网络与 Node"）。
2. **AC2 版本**：启动二进制，日志须含 `Komari Monitor 1.4.3 (hash: <git rev-parse HEAD>)`。
3. **AC3 前端**：`/` → 307 `/install`；`/install` → SPA HTML；其引用的 `/assets/*.js` → 200 且字节数与 vendor 文件一致；`/favicon.ico`、`/themes/default/komari-theme.json` → 200。
4. **AC4 无上游提示**：源码、patch、vendor 产物三处均无 `komari-monitor/komari/releases`。
5. **AC5 幂等**：连续两次 `sync-frontend.sh` 产物哈希一致。
6. **AC6 安装脚本**：静态检查不再指向上游 `latest`。
7. **AC7 追溯**：`git status` 干净；文档存在。
- 回滚：验证失败时按步骤 1-7 各自的回滚点处理；冒烟测试使用独立工作目录（`.build/run`），不污染仓库。

### 9. 提交、建仓、推送

- 动作：
  1. 提交（含 `.trellis/`、`.agents/`、`.dsh/`、`AGENTS.md`、`.gitattributes`、vendor 产物、脚本、文档）。
  2. `gh repo create zhemed/komari --public --description "..."`。
  3. `git remote add origin https://github.com/zhemed/komari.git`。
  4. `git push -u origin komari-1.4.3`。
- 校验：`gh repo view zhemed/komari --json visibility,defaultBranchRef`；`git ls-remote origin` 可见分支。
- 回滚：仓库可 `gh repo delete`（token 含 `delete_repo`）；本地推错可 `git push origin --delete <branch>`。
- **推送前必查**：不得包含凭据（`~/.config/gh/hosts.yml` 不在工作区，无需处理）、不得包含 `.build/`、不得包含真实部署的 `./data`。

## 复核门（`task.py start` 之前）

- `prd.md` 的 8 条需求与 7 条验收标准一一可测。
- `design.md` 中 D1-D8 与实现步骤 1-9 对应，无未决技术未知项。
- 用户已明确批准最终规划摘要。
