# 构建与版本固定（backend）

本文件记录**本 fork 特有**的构建契约。上游文档不覆盖这些约定，改动构建相关文件前必读。

适用范围：`scripts/`、`web/public/`、`install-komari.sh`、`install-agent.sh`、`install-agent.ps1`、`Dockerfile`、`Dockerfile.agent`、`README.md`（产品视角，构建细节在 `docs/MAINTAINING.md`）。
（本仓库已无 `.github/` 流水线——上游 CI 已于 2026-09-16 移除，见 §2 末尾与 `docs/MAINTAINING.md` §4。）

---

## 1. 不可动摇的契约

### 1.1 默认主题必须存在于 `web/public/defaultTheme/`

`web/public/public.go:18` 是 `//go:embed defaultTheme`。该目录缺失时：

- `go build` 直接编译失败；
- 即使绕过编译，`static()` 也会在 `web/public/public.go:130` panic，报错文案为
  `you may forget to put dist of frontend to web/public/defaultTheme/dist`。

因此 `web/public/defaultTheme/` **是源码的一部分，必须提交进仓库**。
上游默认忽略它（`web/public/.gitignore` 的 `defaultTheme/*`），本 fork 已改为不忽略——
**不要把它改回去**，否则 vendor 产物会被静默排除在提交之外（本仓库首次提交时曾因此漏掉 448 个文件）。

### 1.2 前端源码已在仓库内（`frontend/`），产物哈希是门禁

- 源码：`frontend/`（上游 `komari-web@4a74e8a8` 的快照 + 我们内联的改动，2026-09-16 导入）。
  **不要再引入"克隆上游 + 打补丁"的路径**：`scripts/patches/` 已删除，`sync-frontend.sh` 已由
  `scripts/build-frontend.sh` 取代。
- 溯源与构建参数在 `scripts/frontend-build.env`（上游 repo/commit、`KOMARI_UPDATE_REPO`、
  `KOMARI_FRONTEND_SOURCE_DATE_EPOCH`、`FRONTEND_TREE_SHA256`）。
- 依赖安装一律用 `npm ci`（`frontend/package-lock.json` 已入库；注意 `frontend/.gitignore`
  里上游原本忽略了它，我们已取消忽略），**不要用 `npm install`**。
- `frontend/node_modules/`、`frontend/dist/` 不入库（由 `frontend/.gitignore` 兜住）。

### 1.3 产物必须可复现（`FRONTEND_TREE_SHA256`）

`build-frontend.sh` 对 `web/public/defaultTheme/` 计算规范化目录树哈希
（`tar --sort=name --mtime='@0' --owner=0 --group=0 --numeric-owner | gzip -n | sha256sum`）
并与 `frontend-build.env` 比对，不一致即失败。

这条门禁成立的前提是 `frontend/vite.config.ts` 里 `buildTime` 优先读 `SOURCE_DATE_EPOCH`：
上游原本取 `new Date().toISOString()`，经 `define.__BUILD_TIME__` 注入产物并显示于
`frontend/src/components/Footer.tsx`。该常量每次都不同 → 承载它的 chunk 改名 → 所有引用它的
chunk 连锁改名 → 索引与 Service Worker 的预缓存 revision 同步变化，产物因此不可复现。
脚本把 `SOURCE_DATE_EPOCH` 设为 **上游导入 commit 的提交时间**（`KOMARI_FRONTEND_SOURCE_DATE_EPOCH`）。

- **不要**为了让构建通过而清空或放宽 `FRONTEND_TREE_SHA256`；
  确认变化合理时更新它并在提交信息中说明原因。
- 若哈希再次漂移，先按"归一化后比对全部文件"的方式定位是**内容变化**还是**文件名连锁**，
  不要直接当作噪声忽略。

### 1.4 版本号语义（0.0.x 自有版本线）

`CurrentVersion` 由 `scripts/build-komari.sh` 注入；默认版本**只**写在 `scripts/version.env`
（当前 `0.0.4`），`build-komari.sh` 与 `build-agent.sh` 都 source 它。
面向用户的字面量另有 `install-komari.sh` 的 `REPO_TAG`、`install-agent.sh` 的
`default_agent_version`、`install-agent.ps1` 的 `$DefaultAgentVersion`（后两处有门禁兜底，见 1.6）。
前端 `AdminPanelBar.tsx` 的 `parseSemver` 只取 `x.y.z` 三段且要求严格递增，
因此 `0.0.1-fix1` 这类 tag **永远不会**被判为"可更新"；发新版本必须递增 patch 位（`0.0.2`）。

### 1.5 插件系统已移除（禁止重新引入）

本 fork 刻意删除了整个插件系统。以下内容**不得**在升级上游代码时被带回来：

- `internal/plugin/` 整包（运行时的 JS 宿主、hook、RPC、市场安装）。
- 后端：插件市场 API 与 `pluginGroup` 路由、插件 RPC（`admin:listPlugins` 等）、
  `/api/plugin/:short/*filepath` 公开路由、`database/models/plugin.go` 及 `PluginConfiguration`
  的 AutoMigrate 注册（**表本身保留**，见下）。
- 前端（补丁 `0003-drop-plugin-system.patch`）：`pages/admin/{plugins,plugin_config,plugin_page}.tsx`、
  `pages/plugin_page.tsx`、`pages/admin/market/plugins.tsx`、`types/plugin.ts`、
  `iconHelper.resolvePluginIcon`、菜单组与路由、`chunkUpload` 的 `"plugin"` purpose。
- `internal/server/runtime.go` 的插件初始化与 `plugin.HTMLInjectHandler(plugin.WrapHandler(...))` 包装
  （现为直接 `Handler: a.engine`）。
- `web/connection/safe_conn.go` 的 WebSocket 帧拦截器（`FrameInterceptor` / `SetFrameInterceptor`）。

**必须保留、不要连带删除**：

- 结构化的通知/插件运行时曾在插件系统移除时被刻意保留，但 0.0.3 已把它们（连带 goja 依赖）
  **整体删除**，仓库里不再有这两个目录，别按旧结论去找。
- 主题系统与主题市场 —— 它们没有版本门禁，不受版本号影响。
- 数据库中的历史插件表（孤儿表，保留即可，不做破坏性迁移）。

### 1.6 agent 发行线契约（0.0.4 起）

上游 agent 只做**源码依赖**，不 fork 到我们仓库（自有仓库只有 `zhemed/komari` 一个）：

- 源码：`agent/`（上游 `komari-agent@1186aafb`，2026-08-07、**1.4.3 同期**的快照 + 我们内联的改动）。
  **不要**把上游 agent main 整条覆盖进来（那会带进 1.5 行为：motd 安全告警注入、文件访问、
  v2-only 协议，见 `docs/MAINTAINING.md` §11.5）；跟进上游请按 §11.3 手工挑改动。
  溯源与构建参数在 `scripts/agent-build.env`；`scripts/patches-agent/` 已删除。
- 构建：`scripts/build-agent.sh` 纯 Go 交叉编译（`CGO_ENABLED=0`），**不需要 zig/gcc**，
  矩阵与上游 `build_all.sh` 一致 = **14** 个平台（排除 windows/arm、darwin/{386,arm}、非 linux 的 loong64）。
- 资产名必须是 `komari-agent-<os>-<arch>[.exe]`：安装脚本、前端生成的命令、agent 自更新的资产匹配
  三者都按这个名字找。
- **自更新必须带资产过滤** `Filters: []string{"^komari-agent-"}`：`go-github-selfupdate` 按
  **后缀**（如 `linux-amd64`）匹配资产，不过滤会让 agent 刷成同 release 里的服务器二进制
  （`komari-linux-amd64`）。实测证据见 `docs/MAINTAINING.md` §11.1，**升级上游时不得去掉**。
- 自更新目标固定 `Repo = "zhemed/komari"`（源码默认 + 构建期 `-X` 双保险）；
  **默认关闭自动更新**（`EnableAutoUpdate` 默认 false，`--enable-auto-update` 才开）。
- 安装脚本是**同一 pin** 的上游 `install.sh` / `install.ps1` 的 vendor + 补丁（因此沿用 `#!/bin/bash`）：默认安装目录 `/opt/komari-agent`
  （**不要**改回 `/opt/komari`，会和服务器的目录撞车），默认装脚本 pin 的版本。
  `build-agent.sh` 会把补丁回放结果与仓库成品**逐字节比对**，不一致即失败。

## 2. 构建与验证命令

```bash
./scripts/build-komari.sh                 # 输出 bin/komari（仅需 Go，动态链接 glibc）
./scripts/build-frontend.sh                # 重新构建前端产物（改了 frontend/ 才需要，需 Node + 网络）
KOMARI_VERSION=0.0.2 ./scripts/build-komari.sh
KOMARI_STATIC=1 ./scripts/build-komari.sh                      # 发布用：linux/amd64 静态（需 zig）
KOMARI_STATIC=1 KOMARI_GOARCH=arm64 ./scripts/build-komari.sh  # 发布用：linux/arm64 静态
./scripts/build-agent.sh                    # agent：14 个平台 → dist/agent/（源码在 agent/，纯 Go，无需 zig）
./scripts/build-agent.sh --only linux/amd64 # agent：单平台 + 两道门禁校验（改 agent 补丁后必跑）
./scripts/build-agent-image.sh              # agent 镜像：本地单平台构建（--push 才推 ghcr）
./scripts/build-server-image.sh             # 服务器镜像：同上（跨架构需 QEMU/binfmt，见 MAINTAINING §12）
```

**静态构建是发布的前提**：`Dockerfile` 基于 `alpine:3.21`（musl），glibc 动态二进制在其中
无法运行；glibc 静态虽能链接，但 `getaddrinfo`/NSS 依赖宿主共享库，不作为发布形态。
zig 缺失时 `KOMARI_STATIC=1` 必须**明确报错**，不得静默退化为动态链接。

改构建相关文件后的最小验证（也可以直接跑 `./scripts/check-repo.sh --full`，
它把下面这些机械检查打包在一起，见 `docs/MAINTAINING.md` §2.1）：

1. `GOPROXY=off GOFLAGS=-mod=mod ./scripts/build-komari.sh` —— 在**模块缓存已预热**的机器上必须成功。
   ⚠️ 本仓库**没有 `vendor/` 目录**（实测：`GOMODCACHE=<空目录> GOPROXY=off` 会报
   `module lookup disabled by GOPROXY=off`），所以它验证的是"依赖已在本地"，不是"依赖随仓库下发"。
   完全离线的冷机器需要先联网 `go mod download`。
2. 启动二进制，日志须含 `Komari Monitor 0.0.1 (hash: <git rev-parse HEAD>)`。
3. `curl` 校验 `/install`、`/assets/*.js`、`/favicon.ico`、`/themes/default/komari-theme.json` 均 200，
   且 JS 字节数与 `web/public/defaultTheme/dist/` 下同名文件一致。
4. `grep -r "komari-monitor/komari/releases" web/public/defaultTheme/` 必须无结果。
5. `go build ./... && go vet ./... && go test ./...` 全绿。
6. 发布产物另需 `KOMARI_STATIC=1 ./scripts/build-komari.sh`，并用 `file` 确认输出为
   `statically linked`。

**本仓库没有 CI**：上游 `.github/workflows`（10 个 workflow）只做前端构建 + `go build`，
且会从前端默认分支构建、向 `ghcr.io/komari-monitor` 推镜像，因此已整体移除。
上述验证必须**本地手动执行**，发布同样手动（见 `docs/MAINTAINING.md` §3.4）。

## 3. 禁止事项

- 不要 `git push upstream`（`upstream` 仅作只读参考）。
- 不要扩大 `upstream` 的 fetch refspec 让它自动跟踪上游分支：当前只能取到 tag 1.4.3
  是防止误引入 1.5.x 的安全默认。需要上游修复时显式 `git fetch upstream <ref>` 后 cherry-pick。
- 不要把 `install-komari.sh` 的下载路径改回 `releases/latest`（那会安装上游 1.5.x）。
- 不要把 vendored 产物标记为生成物排除出 git——本仓库的可离线构建完全依赖它。
- 不要重新引入插件系统（见 1.5）。
- 不要去掉 agent 自更新的资产过滤 `Filters`（见 1.6；去掉会让节点被刷成服务器二进制）。
- 不要把 agent 版本号改回上游的 `1.x` 线，也不要把 agent 源码 fork 成本仓库的 vendored 目录——
  我们只 pin + 打补丁（见 1.6）。
