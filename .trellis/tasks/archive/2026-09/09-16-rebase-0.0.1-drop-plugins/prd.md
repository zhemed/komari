# 以 0.0.1 为自有基线并彻底移除插件系统

## Goal

我们不再自称任何上游版本：**项目版本全面改为自有的 `0.0.1`**，并**彻底移除插件系统**，
使代码库与上游的版本语义、插件生态彻底解耦，从此按自己的节奏维护。

## 背景与已确认事实（带证据锚点）

### 决策链（用户已明确）

1. 1.4.3 作为我们项目的 **0.0.1 基线**，从 0.0.1 开始自己维护。
2. 插件兼容问题（0.0.1 会让所有 `>=1.x` 的插件被拒载）→ 用户选择 **彻底移除整个插件系统**。

### 版本号被谁消费（实测）

| 位置 | 用途 | 0.0.1 下的结果 |
|---|---|---|
| `main.go:18` | 启动日志 | 显示 0.0.1（期望） |
| `web/rpc/jsonrpc/common.go:461`、`web/rpc/jsonrpc/public.go:84` | 前端展示（页脚/后台/关于） | 显示 0.0.1（期望） |
| `pkg/jsruntime/process/process.go:65` | JS 运行时 `versions.komari` | 随插件系统一起失去主要消费者 |
| `internal/server/bootstrap.go:26` → `database/dbcore/dbcore.go:196` | `versionID = CurrentVersion+"-"+VersionHash`，用于**升级检测与自动备份**（`dbcore.go:233`、`:258`） | 1.4.3→0.0.1 会触发一次全量 `./data` 备份 |
| `internal/plugin/version.go:33,39` | 插件版本约束校验 | **随插件系统移除而消失**（这正是移除的收益） |

- `web/migration/` **不引用** `CurrentVersion`（已核实）→ 版本切换不影响数据库迁移。
- `utils/version.go:4-5` 的默认值本就是 `0.0.1` / `unknown`，0.0.1 正好回到该占位值语义（但我们显式注入）。

### 插件系统完整耦合图谱（实测）

**后端（待删）**

- `internal/plugin/` 整包：21 个文件 / 5414 行。
- 市场与 API：`web/api/admin/plugin.go`、`web/api/admin/plugin_market.go`、`plugin_market_test.go`、`web/api/public/plugin.go`。
- RPC：`web/rpc/jsonrpc/admin.plugin.go`、`admin.plugin_test.go`。
- 模型：`database/models/plugin.go`（`Plugin`/`PluginPermissions`/`PluginPage`/`PluginConfiguration`），注册处 `database/dbcore/dbcore.go:460` 的 `&models.PluginConfiguration{}`。
- 路由：`web/router/` 中第 40 行的公开路由 `/api/plugin/:short/*filepath`；第 210-227 行的 `pluginGroup` 整块（list/enabled/logs/market/*/delete/configuration/静态文件）。
- 服务运行时：`internal/server/runtime.go:22,101-106,117-119` 的 `plugin.Init` / `plugin.LoadAll` / `plugin.CloseAll` / `plugin.HTMLInjectHandler(plugin.WrapHandler(engine))`。
- 备份上传分支：`web/api/admin/archive_upload.go:9,57` 的 `finalizePluginUpload`（走 `plugin.InstallZip`）。

**前端（补丁 0003 待删）**

- 页面：`pages/admin/market/plugins.tsx`、`pages/admin/plugins.tsx`、`pages/admin/plugin_config.tsx`、`pages/admin/plugin_page.tsx`、`pages/plugin_page.tsx`。
- 路由：`src/routes.ts:24-25,92-106,116-118`。
- 菜单：`src/config/menuConfig.json:85-101`（`/admin/plugins`、`/admin/market/plugins`、`/admin/plugins/config`）。
- 类型与工具：`src/types/plugin.ts`、`src/utils/iconHelper.ts` 的 `resolvePluginIcon`、`src/components/admin/AdminPanelBar.tsx` 的插件菜单/注入页处理。
- 上传：`src/lib/chunkUpload.ts:5` 的 `UploadPurpose` 含 `"plugin"`。
- 失效链接：`src/pages/admin/notification/traffic_report.tsx:256-257` 指向 `/admin/market/plugins`，移除后必然 404。

**必须保留（不能连带删除）**

- `pkg/jsruntime/` —— `utils/messageSender/javascript/javascript.go` 在用它，**不是插件专用**。
- 主题系统：`web/api/admin/theme.go`、`theme_market.go`、前端主题页面与 `market/themes.tsx`（用户只要求移除插件）。
- 主题**没有** komari 版本门禁（已核实 `theme_market.go` 与 `market/themes.tsx` 均无版本判定），不受 0.0.1 影响。

## Requirements

- **R1 版本基线全面 0.0.1**：构建注入 `CurrentVersion=0.0.1`；日志、RPC 上报、页脚/后台展示、JS runtime `versions.komari`、`install-komari.sh` 的 `REPO_TAG`、文档与 spec 全部为 0.0.1。**不保留任何对外可见的 1.4.3 版本号**（1.4.3 只作为"代码来源"记录在文档里）。
- **R2 后端移除插件系统**：按上方图谱删除插件包、市场、RPC、公开路由、模型与 AutoMigrate 注册、备份上传的插件分支，并把 `internal/server/runtime.go` 的 HTTP 处理链还原为不带插件包装的形式。
- **R3 前端移除插件系统**：以补丁 `0003` 删除插件页面/路由/菜单/类型/上传 purpose，处理 `traffic_report` 的失效链接，然后重新生成 vendor 产物。
- **R4 保留能力不得回归**：主题系统（含主题市场）、JS 消息发送器、默认主题、备份/恢复主流程、监控核心功能全部照常。
- **R5 数据与升级安全**：不删数据库表（遗留表保留为孤儿，不做破坏性迁移）；明确接受"1.4.3→0.0.1 首次启动触发一次 `./data` 升级备份"；备份/恢复仍可用。
- **R6 构建链保持可复现**：`sync-frontend.sh` 的哈希门禁在新补丁集下重新校准，且两次独立再生成哈希一致。
- **R7 文档与规范同步**：维护文档改名为不再绑定 1.4.3 的标题并重写；`.trellis/spec/backend/build-and-pinning.md` 更新，明确"插件系统已移除，不要重新引入上游插件相关代码"。
- **R8 交付**：提交并推送到 `zhemed/komari`，工作区干净。

## Acceptance Criteria

- **AC1 版本**：启动日志为 `Komari Monitor 0.0.1 (hash: <后端 commit>)`；前端页脚/后台显示 0.0.1。
- **AC2 后端质量门禁**：`go build ./...`、`go vet ./...`、`go test ./...` 全部通过（删除包后无悬挂引用）。
- **AC3 前端质量门禁**：`npm run build` 成功；产物中不含插件页面与市场入口；后台菜单无任何插件项。
- **AC4 残留检查**：`grep -rn "internal/plugin"` 无结果；`/api/plugin/*` 返回 404；源码中除 i18n 未使用键外无插件功能代码。
- **AC5 主题不回归**：`/themes/default/komari-theme.json`、`/favicon.ico`、`/install`、`/assets/*` 全部 200；主题市场相关端点仍在。
- **AC6 升级安全**：在带 `system_version=1.4.3-*` 的数据目录上启动，产生一次 `data/backup/upgrade-*.zip` 并写入新版本标记；再次启动不重复备份。
- **AC7 可复现**：`sync-frontend.sh` 连续两次哈希一致，`FRONTEND_TREE_SHA256` 已更新为新值。
- **AC8 交付**：全部提交已推送，`git status` 干净，`gh repo view` 可见新提交。

## Technical Notes

- **删除即收益**：移除后可去掉约 5.4k 行插件运行时 + 市场/RPC/API 代码，且**插件版本门禁问题彻底消失**，这正是选择"移除"而非"打补丁绕过"的核心理由。
- **JS 运行时保留**：`pkg/jsruntime` 仍被 JavaScript 消息发送器使用，不可删。
- **重命名维护文档**：`docs/MAINTAINING-1.4.3.md` → `docs/MAINTAINING.md`（同步更新 `README.md` 与 spec 里的链接），因为项目已不叫 1.4.3。
- **两处小决策的处理建议**（不阻塞，若你不同意可在批准时指出）：
  1. `traffic_report.tsx` 的插件市场引导链接 → **删除该 CTA**（保留页面其它内容），避免 404。
  2. 备份白名单/恢复流程中的 plugin 目录条目 → **一并移除**，与插件系统移除保持一致。

## Out of Scope

- 不移除主题系统与主题市场。
- 不删除数据库中的历史插件表（保留为孤儿，避免破坏性迁移）。
- 不做前端视觉重设计，不改监控/告警/节点等核心功能。
- 不重新实现任何插件能力（包括把"流量定期报告"内置回来）。
- 不为 0.0.1 发布 GitHub Release（如需让 `install-komari.sh` 真正可用，另行决定）。
