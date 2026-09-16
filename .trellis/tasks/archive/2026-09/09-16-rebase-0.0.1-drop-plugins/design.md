# 技术设计：0.0.1 基线 + 移除插件系统

对应任务：`.trellis/tasks/09-16-rebase-0.0.1-drop-plugins`（需求见 `prd.md`）。

## 1. 边界与原则

- **只删不加**：本次不做功能替代、不做接口过渡层、不保留"隐藏开关"。插件相关代码整块删除，删完即无。
- **不动上游核心**：监控、告警、节点、主题、通知、备份主流程零改动。
- **可回滚**：所有改动落在单个提交里（连同 vendored 产物），`git revert` 即可整体回退到 1.4.3 基线的 0.0.1 之前状态。
- **不删数据**：数据库表保留，只删代码对它的引用。

## 2. 关键设计

### D1 版本号：0.0.1 全面落地

- `scripts/build-komari.sh` 的 `KOMARI_VERSION` 默认值 `1.4.3` → `0.0.1`。
- **`utils/version.go` 的默认值本来就是 `0.0.1`**，与注入值一致，故无需改 Go 源码，减少与上游的差异面。
- `install-komari.sh` 的 `REPO_TAG` 默认 `1.4.3` → `0.0.1`。
- 版本标识 `versionID = "0.0.1-" + VersionHash`（`internal/server/bootstrap.go:26` 未改）。

### D2 后端移除：整包删除 + 悬空引用清理

| 目标 | 处理 |
|---|---|
| `internal/plugin/`（21 文件） | 整目录删除 |
| `web/api/admin/plugin.go`、`plugin_market.go`、`plugin_market_test.go` | 删除 |
| `web/api/public/plugin.go` | 删除（连同 `/api/plugin/:short/*filepath` 路由） |
| `web/rpc/jsonrpc/admin.plugin.go`、`admin.plugin_test.go` | 删除；**实现时先确认 RPC 注册方式**（init 自注册则删文件即可，显式列表则同步删条目） |
| `database/models/plugin.go` | 删除；同步移除 `database/dbcore/dbcore.go:460` 的 `&models.PluginConfiguration{}`（**表不删**，AutoMigrate 不再管理它） |
| `web/router/` `pluginGroup` 块（210-227） | 整块删除；`gorm`/`jsonRpc` 等导入如因此闲置一并清理 |
| `internal/server/runtime.go` | 删 `plugin` import 与 `:101-106`（`Init`/`LoadAll`/cleanup）、`:117-119` 的包装，HTTP 处理链改为 `Handler: a.engine` |
| `web/api/admin/archive_upload.go` | 删 `:9` import 与 `:57` 的 `finalizePluginUpload`、以及按 purpose 分发到它的分支 |

**必须保留**：`pkg/jsruntime/`（`utils/messageSender/javascript` 依赖）、主题与其市场、`web/api/admin/backup_whitelist` 中除 plugin 目录外的条目。

### D3 前端移除：补丁 `0003-drop-plugin-system.patch`

在 pin 住的 `komari-web@4a74e8a8` 检出上（已应用 0001/0002）继续修改，用 `git diff` 生成，保证可重放：

1. 删除页面文件：`pages/admin/market/plugins.tsx`、`pages/admin/plugins.tsx`、`pages/admin/plugin_config.tsx`、`pages/admin/plugin_page.tsx`、`pages/plugin_page.tsx`。
2. `src/routes.ts`：删除 `plugin/:short/*filepath`、`plugins`、`plugins/config`、`plugin-page`、`market/plugins` 五处路由及相应的 `lazy(...)` 导入。
3. `src/config/menuConfig.json`：删除插件菜单组（`/admin/plugins`、`/admin/market/plugins`、`/admin/plugins/config`）。
4. `src/components/admin/AdminPanelBar.tsx`：删除插件菜单渲染、插件注入页处理与 `resolvePluginIcon` 相关分支（保留 `iconMap` 与主题逻辑）。
5. `src/utils/iconHelper.ts`：删除 `resolvePluginIcon`。
6. `src/types/plugin.ts`：删除。
7. `src/lib/chunkUpload.ts`：`UploadPurpose` 去掉 `"plugin"`（保留 `backup` / `theme`）。
8. `src/pages/admin/notification/traffic_report.tsx:256-257`：删除指向 `/admin/market/plugins` 的 CTA（按 PRD Technical Notes 的建议处理）。
9. i18n 键**保留**（未使用的键无害；删除会显著放大补丁体积且无功能收益）。

生成的补丁必须在**干净的 pin 检出**上 `git apply --check` 通过；`tsc -b`（`npm run build` 的一部分）必须通过，这是前端删除是否干净的硬门禁。

### D4 保留 JS 运行时（明确不删）

`pkg/jsruntime` 的唯一非插件消费者是 `utils/messageSender/javascript/javascript.go`。删它会连带破坏"JavaScript 消息发送器"这一既有通知能力，属于 R4 禁止的回归。

### D5 数据与升级行为

- **表**：`plugin_configurations` 等历史表保留（孤儿），不写迁移、不 DROP。
- **升级备份**：`dbcore.go:233` 的规则是"标识不同即备份"，故 `1.4.3-<hash>` → `0.0.1-<hash>` 首次启动会 zip 整个 `./data`（`dbcore.go:258`）。这是**既有安全机制**，本次不修改；验收时按 AC6 显式验证一次并记录。
- **配置中的 `system_version`** 会被改写为 `0.0.1-<hash>`，属预期。

### D6 可复现门禁重校准

补丁集变化 → vendor 产物哈希变化，`FRONTEND_TREE_SHA256` 必须重新校准；校准前必须连续两次独立再生成得到同一哈希（与上一任务相同的判定标准），否则视为失败而不是"更新一下哈希值"。

### D7 文档与规范

- `docs/MAINTAINING-1.4.3.md` → `docs/MAINTAINING.md`：标题与内容改为"0.0.1 自有基线"叙述；1.4.3 仅作为**代码来源**记录（`bf6b45ec` + `komari-web@4a74e8a8`）。
- `.trellis/spec/backend/build-and-pinning.md`：新增"插件系统已移除"的明文约束（禁止重新引入 `internal/plugin`、插件市场、插件 RPC 与前端插件页面），并更新版本号与补丁清单。
- `README.md` 的 fork 说明改为 0.0.1 基线。

## 3. 权衡

| 选择 | 收益 | 代价 |
|---|---|---|
| 彻底删除插件系统（约 6k 行） | 维护面最小；插件版本门禁问题**从根上消失**；与上游插件生态彻底解耦 | 永久失去插件能力，"流量定期报告"等上游插件无法使用 |
| 不删表、不写迁移 | 零数据风险，回滚干净 | 数据库留有孤儿表 |
| i18n 键保留 | 补丁最小、可重放性最好 | 存在未使用的翻译键 |
| 0.0.1 走 ldflags 注入而非改 Go 默认值 | 与上游差异更小（默认值本就是 0.0.1） | 版本号来源集中在构建脚本，需在文档中强调 |

## 4. 回滚

- **整体回滚**：`git revert <本次提交>` 即可恢复插件系统与 1.4.3 版本号（vendored 产物也在同一提交内，故一并恢复）。
- **仅回滚版本号**：`KOMARI_VERSION=1.4.3 ./scripts/build-komari.sh` 可临时构建旧版本号二进制（插件代码已删，不影响该开关用途）。
- **部署侧回滚**：升级前 `dbcore` 自动生成的 `data/backup/upgrade-*.zip` 即为回滚素材。
