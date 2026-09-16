# 执行计划：0.0.1 基线 + 移除插件系统

对应任务：`.trellis/tasks/09-16-rebase-0.0.1-drop-plugins`（需求见 `prd.md`，设计见 `design.md`）。

## Gate 0（前置）

- 必须已获得用户对最终规划摘要的**明确批准**，再执行 `task.py start` 进入本计划。
- 环境：本机 `go1.26.6` / `node v22.23.2` / `npm 10.9.8` / `gcc 11.4.0`；沙箱下 npm/GOPATH/GOCACHE 需重定向到 `.build/`（用户自身 shell 无此限制）。
- 起点：当前 HEAD 为上一任务的收尾提交，工作区干净，`origin` = `zhemed/komari`。

## 实施清单（按序，每步带门禁）

### 1. 版本号落到 0.0.1

- 动作：`scripts/build-komari.sh` 默认 `KOMARI_VERSION=0.0.1`；`install-komari.sh` 默认 `REPO_TAG=0.0.1`。
- 门禁：`bash -n` 通过；`command grep -rn "1\.4\.3" scripts/*.sh install-komari.sh` 仅剩注释/说明性文字（无实际版本注入值）。
- 回滚：`git checkout -- scripts/build-komari.sh install-komari.sh`。

### 2. 后端移除插件系统（分批编译）

按 `design.md` D2 的表逐项删除，**每批后跑一次 `go build ./...`**，避免一次性删完后再面对几十个错误：

1. `web/router/` 的两处路由 + `internal/server/runtime.go` 的插件包装与 cleanup。
2. `web/api/admin/{plugin.go,plugin_market.go,plugin_market_test.go}`、`web/api/public/plugin.go`、`web/rpc/jsonrpc/admin.plugin*.go`。
3. `web/api/admin/archive_upload.go` 的插件分支。
4. `database/models/plugin.go` + `database/dbcore/dbcore.go:460` 的注册项。
5. `internal/plugin/` 整包删除（最后删，确保无引用后再删）。

- 门禁：`go build ./...` 成功；`go vet ./...` 干净；`go test ./...` 全绿。
- 附加检查：`grep -rn "internal/plugin" --include=*.go .` 无结果；`/api/plugin/` 不再出现于路由文件。
- 回滚：`git checkout -- .` 或按文件 `git checkout`；本步不产生数据副作用。

### 3. 前端补丁 0003

- 动作：在 `.build/komari-web`（已应用 0001/0002 的 pin 检出）上按 D3 的 8 项修改，`git diff > scripts/patches/0003-drop-plugin-system.patch`。
- 门禁：
  - 在**干净 pin 检出**上 `git apply --check` 通过；
  - `npm run build`（含 `tsc -b`）通过——这是前端删除是否干净的唯一硬证据；
  - 产物中不再出现 `market/plugins`、`plugin_page`、`admin/plugins`。
- 回滚：删除补丁文件；`.build/` 为可再生目录。

### 4. 重新生成 vendor + 哈希重校准

- 动作：`npm_config_cache=.build/npm-cache ./scripts/sync-frontend.sh`，先清空 `FRONTEND_TREE_SHA256` 跑两次取一致值，再回填并第三次验证门禁。
- 门禁：两次独立再生成哈希**完全一致**；回填后脚本报"哈希校验通过"。
- 回滚：`git checkout -- scripts/frontend-pin.env web/public/defaultTheme`。

### 5. 文档与规范

- 动作：`git mv docs/MAINTAINING-1.4.3.md docs/MAINTAINING.md` 并重写为 0.0.1 叙述（1.4.3 仅作为代码来源）；更新 `README.md` 引用路径与文案；更新 `.trellis/spec/backend/build-and-pinning.md`（新增"插件系统已移除"约束 + 版本号 + 补丁清单）。
- 门禁：`grep -rn "MAINTAINING-1.4.3" .` 无残留（排除 `.git/`）；文档中所有 commit/命令与实测一致。
- 回滚：`git checkout -- README.md .trellis/spec/ docs/`。

### 6. 端到端验收（AC1-AC8）

1. **AC1**：`./scripts/build-komari.sh` 后启动，日志必须是 `Komari Monitor 0.0.1 (hash: <git rev-parse HEAD>)`。
2. **AC2**：`go build ./...`、`go vet ./...`、`go test ./...`。
3. **AC3**：`sync-frontend.sh` 已构建；`grep -rl "market/plugins\|plugin_page" web/public/defaultTheme/` 无结果。
4. **AC4**：`/api/plugin/x` 返回 404；`grep -rn "internal/plugin"` 无结果。
5. **AC5**：`/install`、`/assets/*.js`、`/favicon.ico`、`/themes/default/komari-theme.json` 均 200；主题市场端点（`/api/admin/theme/market/catalog`）仍存在（未登录时按既有鉴权行为响应即可）。
6. **AC6（升级安全，真实场景）**：用**旧的 1.4.3 数据目录**启动新二进制——需先准备：用当前已推送的 1.4.3 二进制在临时目录跑一次生成 `./data` 与 `system_version=1.4.3-*`，再换 `0.0.1` 二进制启动同一目录，断言出现 `data/backup/upgrade-*.zip` 且日志含 `[upgrade-backup] ... (from "1.4.3-..." to "0.0.1-...")`；再次启动**不得**再产生新备份。
7. **AC7**：步骤 4 已完成两次一致；确认 `FRONTEND_TREE_SHA256` 为最终值。
8. **AC8**：提交推送后 `git status` 干净、`git ls-remote` 与本地上限一致。
- 回滚：验收失败按各步骤回滚点处理；服务冒烟统一在 `.build/run` 独立目录进行，不污染仓库与真实数据。

### 7. 提交与推送

- 动作：单个提交包含版本号、后端删除、补丁 0003、vendored 产物、文档与 spec；随后 `git push origin komari-1.4.3`。
- 门禁（推送前）：`git status -sb` 干净；`git diff --cached --name-only | grep -E '^(\.build/|bin/|data/)'` 为空。
- 回滚：`git revert HEAD` + 重新推送；远端历史保留证据。

## 复核门（`task.py start` 之前）

- PRD 的 R1-R8 与 AC1-AC8 一一可测，无阻塞性未决问题。
- `design.md` D1-D7 与上式步骤 1-7 对应；两处小决策（traffic_report CTA、备份白名单 plugin 条目）已给出建议并被批准。
- 用户已明确批准最终规划摘要。
