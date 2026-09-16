# 构建与版本固定（backend）

本文件记录**本 fork 特有**的构建契约。上游文档不覆盖这些约定，改动构建相关文件前必读。

适用范围：`scripts/`、`web/public/`、`install-komari.sh`、`Dockerfile`。
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

### 1.2 前端版本以 `scripts/frontend-pin.env` 为唯一事实来源

- `KOMARI_WEB_COMMIT` 固定到具体 commit（当前 `4a74e8a8…`，即 komari-web tag 1.4.3）。
- **不要改为分支名或 `latest`**：上游 CI（`.github/actions/build-frontend/action.yml:34`，
  **该目录已从本仓库移除**）在普通 tag 下会退化为克隆默认分支，这正是上游同 tag 二进制
  不可复现的原因。
- 依赖安装一律用 `npm ci`（仓库内已提交 `package-lock.json`），**不要用 `npm install`**。

### 1.3 产物必须可复现（`FRONTEND_TREE_SHA256`）

`sync-frontend.sh` 对 `web/public/defaultTheme/` 计算规范化目录树哈希
（`tar --sort=name --mtime='@0' … | gzip -n | sha256sum`）并与 `frontend-pin.env` 比对，不一致即失败。

这条门禁成立的前提是 `scripts/patches/0002-reproducible-build-time.patch`：
上游 `vite.config.ts:53` 取 `new Date().toISOString()`，经 `define.__BUILD_TIME__`
（`vite.config.ts:125-126`）注入产物并显示于 `src/components/Footer.tsx:26`。该常量每次都不同
→ 承载它的 chunk 改名 → 所有引用它的 chunk 连锁改名 → 索引与 Service Worker 的预缓存 revision
同步变化，产物因此不可复现。补丁让 `buildTime` 优先读 `SOURCE_DATE_EPOCH`，
脚本把它设为 **pin commit 的提交时间**。

- **不要**为了让构建通过而清空或放宽 `FRONTEND_TREE_SHA256`；
  确认变化合理时更新它并在提交信息中说明原因。
- 若哈希再次漂移，先按"归一化后比对全部文件"的方式定位是**内容变化**还是**文件名连锁**，
  不要直接当作噪声忽略。

### 1.4 版本号语义（0.0.x 自有版本线）

`CurrentVersion` 由 `scripts/build-komari.sh` 注入，默认 `0.0.2`。
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

- `pkg/jsruntime/` —— 被 `utils/messageSender/javascript` 使用，**不是插件专用**。
- 主题系统与主题市场 —— 它们没有版本门禁，不受版本号影响。
- 数据库中的历史插件表（孤儿表，保留即可，不做破坏性迁移）。

## 2. 构建与验证命令

```bash
./scripts/build-komari.sh                 # 输出 bin/komari（仅需 Go，动态链接 glibc）
./scripts/sync-frontend.sh                # 重新生成前端产物（需网络 + Node）
KOMARI_VERSION=0.0.2 ./scripts/build-komari.sh
KOMARI_STATIC=1 ./scripts/build-komari.sh                      # 发布用：linux/amd64 静态（需 zig）
KOMARI_STATIC=1 KOMARI_GOARCH=arm64 ./scripts/build-komari.sh  # 发布用：linux/arm64 静态
```

**静态构建是发布的前提**：`Dockerfile` 基于 `alpine:3.21`（musl），glibc 动态二进制在其中
无法运行；glibc 静态虽能链接，但 `getaddrinfo`/NSS 依赖宿主共享库，不作为发布形态。
zig 缺失时 `KOMARI_STATIC=1` 必须**明确报错**，不得静默退化为动态链接。

改构建相关文件后的最小验证：

1. `GOPROXY=off GOFLAGS=-mod=mod ./scripts/build-komari.sh` —— 必须成功（证明 vendor 生效、无需网络）。
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
