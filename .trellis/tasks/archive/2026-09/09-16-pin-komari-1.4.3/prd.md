# 锁定并维护 komari 1.4.3 分叉基线

## Goal

我们只维护 1.4.3 这一个版本：把它固化为**可离线复现、可长期自持**的自有基线，消除与上游 1.5.x 的隐式耦合（升级提示、写死上游的安装脚本、构建期依赖外部前端仓库），并把基线推送到自有公开仓库 `zhemed/komari`。

## 背景与已确认事实（带证据锚点）

### 仓库与身份

- 工作区 `/root/komari` 原为空目录；已浅克隆上游 tag `1.4.3`。
- 后端基线：`komari-monitor/komari` tag `1.4.3` → commit `bf6b45ec3abfc56bba5e9223650a47a72f665371`。
- 自有主干分支：`komari-1.4.3`；原 `origin` 已改名 `upstream`（`--single-branch --depth 1`，fetch refspec 锁在 `refs/tags/1.4.3`）。
- 上游 `1.5.0` / `1.5.0-fix1` 已发布 → "只维护 1.4.3" 意味着放弃上游后续特性与潜在的后续安全修复，需自行决定是否 backport。
- 自有账号与仓库：`gh auth status` → ✓ Logged in to github.com as **zhemed**（token 含 `repo` 权限）；`github.com/zhemed` 存在（8 个公开仓库，含 `zhemed/komari-theme-ink`）；**`zhemed/komari` 尚不存在**，需创建。
- 常驻无头浏览器 **未登录** github.com（`/settings/profile` → 302 到 `/login`，首页显示 Sign in/Sign up）→ 更新检查是浏览器端未认证请求，因此**仓库必须公开**该功能才可用（已选定公开）。

### 构建链（关键约束，全部实测）

- 后端仓库**单独无法构建**：`web/public/public.go:18` 为 `//go:embed defaultTheme`，而克隆产物无 `web/public/defaultTheme/`；缺失时 `static()` 在 `web/public/public.go:130` 直接 panic（提示 `you may forget to put dist of frontend to web/public/defaultTheme/dist`）。
- 前端是独立仓库：`.github/actions/build-frontend/action.yml:2`（"Clone komari-web, build the default theme"）、`:37-38`（`npm install` + `npm run build`）、`:41-48`（拷贝 `dist/*`、`komari-theme.json`、`preview.png` 到 `web/public/defaultTheme/`）。
- **tag 不锁前端版本**：`.github/workflows/release.yml:26-30` 仅在 tag 形如 `X.Y.Z-fixN` 时才回指前端 ref；普通 tag（如 `1.4.3`）`frontend_ref` 为空 → `action.yml:34` 退化为克隆 komari-web **默认分支**（现为 `radix`）。上游 1.4.3 二进制所用前端因此**不确定可复现**。
- 前端同名 tag 可用作确定性 pin：`komari-monitor/komari-web` tag `1.4.3` → commit `4a74e8a81e2e4b1c3da8ad795f9523151efb6b56`；该 commit 内**提交了 `package-lock.json`（497,428 字节）** → 可用 `npm ci` 确定复现。
- 版本注入（`.github/workflows/release.yml:124`）：`go build -trimpath -ldflags="-s -w -X github.com/komari-monitor/komari/utils.CurrentVersion=${VERSION} -X github.com/komari-monitor/komari/utils.VersionHash=${VERSION_HASH}"`；默认值见 `utils/version.go:4-5`（`0.0.1` / `unknown`）。
- 工具链现状：本机 `go1.26.6`、`node v22.23.2`、`npm 10.9.8`、`gcc 11.4.0`（cgo 必需，`mattn/go-sqlite3`）。`go.mod:3` 要求 `go 1.25.0`，而 `.github/workflows/release.yml:105` 用 `go-version: "1.23"`（不一致，以 `go.mod` 为准）。
- 跨平台交叉编译在上游依赖 zig（`.github/workflows/release.yml:107-115`），本地构建不涉及。

### 已完成的可行性验证（本机实测，可复现）

- 前端 `npm install`（802 包）+ `npm run build` 成功：`dist` 6.7MB，PWA 预缓存 446 条。
- 注入主题后 `go build` 成功：27 秒，产出 38,095,288 字节二进制。
- 冒烟结果：启动日志 `Komari Monitor 1.4.3 (hash: bf6b45ec3abfc56bba5e9223650a47a72f665371)`；`/` → 307 到 `/install`；`/install` → SPA HTML；`/assets/entry-index-DqVZ412e.js` → 200 / 696,601B；`/favicon.ico` → 200 / 40,832B；`/themes/default/komari-theme.json` → 200 / 8,702B。

### 与上游的耦合风险点（实测确认）

- **后台更新提示**：`komari-web/src/components/admin/AdminPanelBar.tsx:263` 硬编码 `https://api.github.com/repos/komari-monitor/komari/releases?per_page=100`，与本机版本比较后展示"可更新"（`:255-296`、`:433`）。前端**无任何配置机制**可覆盖该 URL（全仓 `import.meta.env` 仅 `src/utils/themeConfiguration.ts:40` 的 `BASE_URL` 一处）→ 必须改源码或构建期注入。
- **版本比较只取三段**：`AdminPanelBar.tsx` 的 `parseSemver` 用 `^(\d+)\.(\d+)\.(\d+)`，`isNewerVersion` 要求三段严格递增 → 形如 `1.4.3-fix1` 的 tag **永远不会**被判为"更新"。这决定了我们将来的发版号规则（见 Technical Notes）。
- **安装脚本写死上游**：`install-komari.sh:36` `REPO="komari-monitor/komari"`，`:291` 稳定版走 `releases/latest/download/...` → 用它安装会装上 1.5.x，而非我们的 1.4.3。
- **后端无自升级逻辑**：全仓检索 `releases|api.github.com|checkUpdate` 未见核心自更新代码（仅 `install-komari.sh` 与主题/插件市场的 `web/api/admin/theme.go:386` 访问 GitHub）→ 部署不会被程序自己覆盖。
- **插件/主题兼容**：`internal/plugin/version.go:33-39` 以 `utils.CurrentVersion` 校验插件声明的 komari 约束 → 1.4.3 下，市场里要求更高版本的新插件会被拒绝加载。

### 环境备注（沙箱限制，非仓库问题）

- 本会话文件策略为 workspace-write，`/root/.npm`、`/root/go` 不可写 → npm/GOPATH/GOCACHE 已重定向到 `/root/komari/.build/`（767MB）。用户自己的 shell 无此限制。

## Requirements

- **R1 后端基线固定**：停留在 `komari-monitor/komari@1.4.3`，主干分支 `komari-1.4.3`，不追踪上游 `main`。
- **R2 前端基线固定并 vendor**：默认主题产物必须来自 `komari-web@4a74e8a8`（tag 1.4.3），并提交进本仓库，使后端在**无网络、无 Node** 环境下仅用 Go 工具链即可 `go build`。
- **R3 可再生成**：提供再生成脚本，从 pin 的前端 commit 重新产出 vendor 目录，并做哈希校验（区分"能构建"与"可复现"）。
- **R4 版本标识**：构建注入 `CurrentVersion=1.4.3` 与后端 `VersionHash`，与上游口径一致（`release.yml:124`），使前端版本比较与插件兼容检查行为不变。
- **R5 更新提示去上游化**：后台不得再提示升级到上游 1.5.x；检查目标改为公开仓库 `zhemed/komari`，且地址可从构建期覆盖。
- **R6 自有基线推送**：创建公开仓库 `zhemed/komari`，把 `komari-1.4.3` 分支推上去；远端配置为 `origin`=自有仓库、`upstream`=上游（只读参考）。
- **R7 安装脚本去上游化**：`install-komari.sh` 不得再隐式安装上游 `latest`（1.5.x），需指向自有发布或强制指定 1.4.3。
- **R8 pin 可追溯 + 入场即干净**：pin 事实（后端 commit、前端 commit、工具链、构建/再生成命令）写入仓库文档；`.trellis/`、`AGENTS.md` 等 Trellis 产物一并提交；提交后工作区干净。

## Acceptance Criteria

- **AC1（离线可构建）**：在无网络、无 Node 的环境下，仅用 Go 工具链 `go build` 成功产出二进制（主题来自 vendor）；失败即证明 vendor 未生效。
- **AC2（版本正确）**：启动日志打印 `Komari Monitor 1.4.3 (hash: <后端 commit>)`。
- **AC3（前端可用）**：`/` → 307 到 `/install`；`/install` 返回 SPA HTML；其引用的 `/assets/*.js` 返回 200 且字节数与 vendor 产物一致；`/favicon.ico`、`/themes/default/komari-theme.json` 均 200。
- **AC4（无上游提示）**：源码与构建产物中均不再出现 `api.github.com/repos/komari-monitor/komari/releases`；更新检查指向 `zhemed/komari`。
- **AC5（再生成幂等）**：再生成脚本可重复执行，在 `package-lock.json` 锁定不变的前提下产出相同哈希；哈希不符时明确失败。
- **AC6（安装脚本安全）**：使用改造后的 `install-komari.sh` 不会安装 1.5.x（指向自有仓库或明确锁定 1.4.3）。
- **AC7（可追溯 + 干净）**：仓库内存在 pin 文档，记录后端 commit、前端 commit、Go 版本、ldflags；`git status` 干净，`zhemed/komari` 上可见 `komari-1.4.3` 分支。

## Technical Notes

- **发版号规则**：维持 `CurrentVersion=1.4.3`。将来若发布自有补丁版，**必须递增三段中的 patch 位**（如 `1.4.4`）才会被前端判为"更新"；`1.4.3-fixN` 形式在 `AdminPanelBar.tsx` 的 `isNewerVersion` 下恒为"非更新"（只影响提示，不影响功能）。
- **主题目录契约**：`web/public/defaultTheme/dist/index.html` 必须存在，否则 `web/public/public.go:130` panic。
- **不 fork 前端**：以固定 commit + patch 文件方式复用 `komari-web`，不引入第二个需要长期维护的 fork。

## Out of Scope

- 不升级到 1.5.0，不合并上游后续特性。
- 不改动后端核心业务逻辑（监控、指标、告警、插件运行时），不做数据库 schema/migration 变更。
- 除 R5/R7 的地址与提示改造外，不做前端功能改造。
- 不搭建 CI/CD、不做多平台交叉编译流水线。
- 不实现上游安全修复的自动 backport 机制（仅记录为运维决策）。
