# 质量与测试约定（backend）

本文件描述本仓库**实际执行**的质量基线：什么被自动检查、什么没有、提交前必须自己跑什么。
关键前提先说清楚——**本仓库没有 CI 级质量门禁**，见 §4。

## 1. 当前质量现状（实测，不要理想化）

| 项 | 现状 |
|---|---|
| 第一方 `*_test.go` | **77** 个（排除 `.build/`），约 1.6 万行测试代码 |
| 测试目录 | 无 `testdata/`、无独立 `test/` 包；测试与被测代码**同目录同名** |
| lint 配置 | **不存在** `.golangci.yml` / `.golangci.yaml` / `.editorconfig` / `Makefile` / `.gofmt` |
| pre-commit 钩子 | 无 |
| `gofmt` 一致性 | **当前树并非 gofmt-clean**：`gofmt -l` 列出 14 个文件（见 §3） |
| CI 测试 | **本仓库没有 CI**：上游 `.github/workflows/`（10 个 workflow，只做前端构建 + `go build`，**无 `go test` / `go vet`**）已于 2026-09-16 整体移除 |
| README 构建说明 | README 已改成**产品视角短文**（定位/特性/部署/维护）；构建与发布命令在 `docs/MAINTAINING.md` §3，README 只保留 `go build/vet/test` 自检三段 |
| `CONTRIBUTING.md` | 不存在 |

所以：**质量靠自己跑命令 + code review**，不要假设推上去会被拦住。

## 2. 提交前必跑

```bash
go build ./... && go vet ./... && go test ./...
```

这条命令集的唯一第一方出处是 `.trellis/spec/backend/build-and-pinning.md` §2「验证清单」第 5 条
（跨文件引用用章节而非行号，避免行号漂移后失效）。
历史任务把它当验收门禁（`.trellis/tasks/archive/2026-09/09-16-rebase-0.0.1-drop-plugins/prd.md:69`、
`.trellis/tasks/archive/2026-09/09-16-rebase-0.0.1-drop-plugins/implement.md:29`、`:57`）。

改动构建/前端相关文件时，还须跑 `build-and-pinning.md` §2 的完整清单
（`GOPROXY=off GOFLAGS=-mod=mod ./scripts/build-komari.sh`、启动校验版本行、`curl` 关键路径等）。

工具链事实：`go.mod:3` 是 `go 1.25.0`，`docs/MAINTAINING.md:36-37` 要求 Go ≥ 1.25.0 + gcc（CGO 必需）。
⚠️ 上游 CI 有 7 个 workflow 写的是 `go-version: "1.23"`（如 `.github/workflows/build.yml:81`、
`.github/workflows/release.yml:105`——该目录已移除，此处仅作历史说明），与 `go.mod` 不一致；
`docs/MAINTAINING.md` §2 明确裁定**以 `go.mod` 为准**。

## 3. 格式与静态检查

`gofmt` / `goimports` **未被 CI 或脚本强制**，`go vet` 也没有配置或豁免清单。

- 提交的代码**应当**是 `gofmt` 干净的（写新文件前跑一次 `gofmt -w <file>`），`go vet ./...` 必须干净（§2）。
- 但**不要**为了"修格式"而大规模重排历史文件：当前树已有 14 个文件不是 gofmt-clean（根因是 Go 1.19+
  的注释重排规则与末尾空行），属于历史遗留，不属于任何单次改动的范围。`gofmt -l` 实测清单：

  ```
  database/records/records.go                    internal/server/metric_store.go
  internal/migrations/legacy_monitoring.go       pkg/rpc/permission.go
  internal/migrations/legacy_monitoring_test.go  pkg/rpc/principal.go
  internal/server/guides.go                      pkg/rpc/principal_test.go
  web/api/client/ingest.go                       web/migration/migration.go
  web/install/install_test.go                    web/migration/migration_test.go
  web/recovery/recovery.go                       web/rpc/jsonrpc/admin.client.go
  ```

## 4. 测试布局与写法

### 4.1 位置：同目录同名

`x.go` + `x_test.go` 放在一起，例：`pkg/metric/store.go` / `pkg/metric/store_test.go`、
`utils/log/gin.go` / `utils/log/gin_test.go`、`web/security/cors.go` / `web/security/cors_test.go`、
`internal/sqlitetune/connector.go` / `internal/sqlitetune/connector_test.go`。

唯一"无同名源码"的测试文件是 `web/api/public/test_db_test.go`——它是测试基建（只有 `TestMain`），
不是某个函数的测试。

测试最富的包（2026-09-16 实测，全仓 77 个 `*_test.go`）：`pkg/metric`（19 个）、
`web/rpc/jsonrpc`（8 个）、`agent/monitoring/unit`（5 个）、`internal/metricstore`（4 个）、
`agent/server`（4 个）、`web/api/admin`（3 个）。前端与 agent 源码 vendor 进本仓库后，
`agent/` 下的测试同样归我们维护（其中 3 个依赖外网，见 `docs/MAINTAINING.md` §7）。

**大量目录没有测试**（55 个），包括 `cmd/`、`internal/config/`、`database/accounts/`、
`database/clients/`、`database/auditlog/`、`web/router/`、`web/upload/`。
给这些地方加代码时，"有测试"不是既有惯例；反过来，`pkg/metric`、`web/rpc/jsonrpc`、
`internal/metricstore` 这几个包改动时应当补测试（现有测试密度最高）。

### 4.2 断言：默认手写，testify 是例外

- 主流写法是标准库 `testing` + `t.Fatalf`（全仓约 1592 处）/ `t.Fatal` / `t.Errorf`（37 处）。
- `github.com/stretchr/testify` 虽是直接依赖（`go.mod:19`，v1.11.1），但**只有 3 个测试文件**用了它：
  `utils/notifier/traffic_report_test.go:7`、`web/api/public/login_test.go:13`、
  `database/notification/traffic_report_test.go:6-7`。**非测试源码 0 处导入。**
- 结论：新测试默认手写 `t.Fatalf`；不要引入 testify 作为新约定。

### 4.3 白盒同包测试是常态

77 个测试文件中 **75 个用被测同包**（`package jsonrpc`、`package metricstore` …），
未导出函数被直接测试，例：`internal/server/app_test.go:16` 测 `retryMetricStoreConnection`、
`internal/metricstore/metricstore_test.go:36` 测 `buildMetricConfig`、
`web/public/public_test.go:72` 测 `replaceHTMLLanguage`。

只有 2 个外部测试包：`utils/geoip/geoip_test.go:1`（`package geoip_test`）、
`pkg/metric/example_test.go:1`（`package metric_test`，因为它要写 `Example`）。

### 4.4 表驱动 + 子测试

```go
tests := []struct {
	name  string
	count int
	want  []int
}{
	{name: "empty", count: 0, want: []int{}},
	{name: "latest only", count: 1, want: []int{4}},
	{name: "even selection", count: 3, want: []int{0, 2, 4}},
}

for _, test := range tests {
	t.Run(test.name, func(t *testing.T) {
		if got := sampleEvenly(input, test.count); !reflect.DeepEqual(got, test.want) {
			t.Fatalf("sampleEvenly(%v, %d) = %v, want %v", input, test.count, got, test.want)
		}
	})
}
```

（`web/rpc/jsonrpc/common_record_test.go:8-28`。）同形见 `web/security/cors_test.go:18-45`、
`pkg/rpc/principal_test.go:6-30`；map 形式的表见 `internal/migrations/timestamp_test.go:35-47`、
`web/rpc/jsonrpc/public.audit_test.go:11-21`。

### 4.5 HTTP 层测试

标准形态：`gin.SetMode(gin.TestMode)` → `gin.New()` → 挂路由/中间件 → `ServeHTTP` → 断言 `Code` 与 `Header()`：

```go
gin.SetMode(gin.TestMode)
router := gin.New()
router.Use(noStoreAPIResponses())
router.NoRoute(guideNoRoute("/install", "Not found in install mode", nil))

apiResponse := httptest.NewRecorder()
router.ServeHTTP(apiResponse, httptest.NewRequest(http.MethodGet, "/api/unknown", nil))
if apiResponse.Code != http.StatusNotFound { ... }
```

（`internal/server/app_test.go:31-53`。）

- `httptest.NewRecorder()` / `httptest.NewRequest` 出现在 8 个测试文件，含 `utils/log/gin_test.go:24`、`web/security/cors_test.go:66`、`web/install/install_test.go:44`、`web/public/public_test.go:105`、`web/recovery/recovery_test.go:18`、`web/migration/migration_test.go:92`。`web/connection/safe_conn_test.go:14` 额外用 `httptest.NewServer` 起真实 WS。
- 中间件测试把被测中间件挂到最小路由上，helper 抽在测试文件内：`web/security/cors_test.go:147`（`setupCORSRouter`）、`web/security/cors_test.go:185`（`performCORSRequest`）。
- 端到端测试用"真实 handler 集合"而非整机：`web/install/install_test.go:19` 的 `setupInstallRouter(t)` 返回 `(*gin.Engine, *gorm.DB, *Controller)`；`web/public/public_test.go:89-101` 先 `config.SetDb(db)` 再 `StaticRestricted(...)`。

### 4.6 数据库测试：内存或 `t.TempDir()`，不连真实外部库

- 共享内存 DSN 用**测试名**做库名，避免子测试串库（`internal/migrations/timestamp_test.go:64`、`database/models/time_test.go:16`；`internal/migrations/migrations_test.go:18` 是同形的 helper）：

  ```go
  dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
  ```

- 内存库必须收敛连接数，否则多连接看到的不是同一个库：`sqlDB.SetMaxOpenConns(1)`（`internal/migrations/migrations_test.go:27`、
  `web/api/public/test_db_test.go:16-18`、`web/security/cors_test.go:173`）。
- 需要真实文件时用 `t.TempDir()`（全仓 71 处）+ `?mode=rwc`，例：`web/install/install_test.go:21`、`web/migration/migration_test.go:21`、`internal/migrations/legacy_monitoring_test.go:22`。
- metric store 测试走本包的构造函数：`metric.Open(ctx, metric.SQLite(":memory:"))`（`pkg/metric/restructure_test.go:15`、`internal/metricstore/maintenance_test.go:14`），必要时带 `WithMaxOpenConns(1)`（`internal/metricstore/metricstore_test.go:230`）或 `WithAutoMigrate(false)`（`pkg/metric/restructure_test.go:306`）。
- **需要外部数据库的用例必须可跳过**，不能成为默认依赖：`pkg/metric/postgres_integration_test.go:16-18`（`METRIC_POSTGRES_DSN` 未设 → `t.Skip`）、`:29-31`（`METRIC_MYSQL_DSN`）、`:39-41`（`METRIC_MARIADB_DSN`）、`pkg/metric/restructure_test.go:897-899`（`KOMARI_METRIC_RESTRUCTURE_DUMP`）。
- 全仓**没有 `//go:build` 测试标签**，跳过一律用运行期 `t.Skip`。

### 4.7 测试夹具：文件内私有 helper + `t.Cleanup`

没有 `testutil` 公共包。既有 helper 一律是所在测试文件里的私有函数，配 `t.Helper()` + `t.Fatalf`：

| 用途 | helper |
|---|---|
| 包级 DB 引导（全仓唯一 `TestMain`） | `web/api/public/test_db_test.go:11` |
| 路由装配 | `web/install/install_test.go:19` `setupInstallRouter`、`web/migration/migration_test.go:19` `setupConfigDB`、`web/security/cors_test.go:147` `setupCORSRouter` |
| DB 夹具 | `internal/migrations/migrations_test.go:15` `openTestDB`、`internal/migrations/timestamp_test.go:62` `openTimestampMigrationDB` |
| 指标夹具 | `pkg/metric/rollup_test.go:18` `newRollupStore`、`internal/metricstore/report_test.go:15` `useReportTestStore` |
| 其他 | `web/backup/restore_test.go:10` `writeTestArchive`、`web/connection/safe_conn_test.go:13` `wsEchoServer`、`database/dbcore/sqlite_tuning_test.go:91` `sqlitePragmaInt` |

清理统一 `t.Cleanup`（41 处）：`web/install/install_test.go:31` 关 `sqlDB`、`web/security/cors_test.go:175-177`、
`utils/log/gin_test.go:18` 还原被替换的全局 logger。**"替换全局状态 + `t.Cleanup` 还原"是既有手法**，
用这些地方就要照抄：`utils/log/gin_test.go:15-18`（`defaultLogger`）、
`web/api/client/uploadBasicInfo_test.go:46-50`（`geoip.CurrentProvider`）、
`web/rpc/jsonrpc/admin.system_test.go:36-40`（`gormtests.DummyDialector{}` + `DryRun: true` 断言生成的 SQL）。

### 4.8 时间相关的测试必须确定性

- 存储层断言 UTC：`database/models/time_test.go:17-19` 传 `NowFunc: func() time.Time { return time.Now().UTC() }`，并用固定时刻 `time.Date(2026, 7, 17, 1, 2, 3, 123456789, time.UTC)`（`:28`）断言纳秒不丢（`:37-38`）。
- 日历语义测试替换 `time.Local` 并还原：`pkg/timeutil/timeutil_test.go:6-11`。
- 迁移测试用 `t.Setenv("TZ", ...)` 覆盖时区分支：`internal/migrations/timestamp_test.go:14`、`:29`。
- HTTP 层用固定时刻 + `time.Now().UTC().Truncate(time.Millisecond)`（`web/rpc/jsonrpc/public_metric_test.go:65` 等），不要依赖真实"现在"。
- **没有引入 clock 抽象或时间 mock 库**：全仓 `NowFunc|nowFunc|clock.Clock` 只有 2 处命中（`database/dbcore/dbcore.go:398` 的生产代码与 `database/models/time_test.go:19`）。

## 5. 前端约束（容易踩，与后端同仓库）

前端源码**已在本仓库**（`frontend/`，上游快照 + 我们内联的改动，2026-09-16 导入），
由 `scripts/build-frontend.sh` 在本地构建；`.build/`、`frontend/node_modules/` 与 `frontend/dist/`
都是生成物，**不要**当源码改。因此：

- **改前端行为要改补丁，不要改 `.build/komari-web/`**（会被 `git clean -xfdq` + 重新检出覆盖）。
  现有 3 个补丁：`0001-update-check-repo.patch`、`0002-reproducible-build-time.patch`、
  `0003-drop-plugin-system.patch`。
- TS 编译选项（`.build/komari-web/tsconfig.app.json`）是**严格模式**：
  `"strict": true`（`:19`）、`"noUnusedLocals": true`（`:20`）、`"noUnusedParameters": true`（`:21`）、
  `"noFallthroughCasesInSwitch": true`（`:23`）、`"verbatimModuleSyntax": true`（`:13`）、
  `"noEmit": true`（`:15`，只做类型检查）。
  → **未使用的局部变量/参数会直接编译失败**，`type` 导入必须显式 `import type`。
- 构建前端的硬性前置见 `build-and-pinning.md` §1：`npm ci`（不是 `npm install`）、
  `FRONTEND_TREE_SHA256` 必须匹配、产物不得再出现 `komari-monitor/komari/releases`。

## 6. 本仓库特有禁止事项

以下都是本 fork 与上游的差异点，违反会直接破坏可构建性（详细理由见 `build-and-pinning.md`）：

1. **不要重新引入插件系统**。`internal/plugin/`、插件市场、插件 RPC、`/api/plugin/*`、
   前端插件页面均已删除，升级上游代码时若被带回必须剔除。**保留物只有**：主题系统与主题市场、
   数据库里的历史插件表（孤儿表）。`pkg/jsruntime/` 与 `utils/messageSender/` 已在 0.0.3
   随通知系统一并删除，不要再按"插件专用例外"的旧结论去找它们。
2. **不要把 `web/public/defaultTheme/` 排除出 git**。它被 `//go:embed` 引用
   （`web/public/public.go:18`），缺失时 `web/public/public.go:130` panic、`go build` 直接失败。
   当前有 **431** 个受版本控制的文件；`web/public/.gitignore:6` 的 `# defaultTheme/*` 保持注释状态。
   首次提交曾因忽略它漏掉 448 个文件。改前端后用 `scripts/build-frontend.sh` 重建：它整体替换该目录，
   并校验目录树哈希与 `scripts/frontend-build.env` 一致。
3. **不要 `git push upstream`**，也不要扩大 `upstream` 的 fetch refspec（当前只取到 tag 1.4.3）。
4. **不要把 `install-komari.sh` 的下载路径改回 `releases/latest`**（会装到上游 1.5.x）。
5. **不要把 Go 依赖说成"vendor"**：仓库根**没有** `vendor/` 目录，`.gitignore:26` 的 `# vendor/` 是注释掉的；
   Go 依赖走 module cache。本仓库说的 "vendored" **专指前端产物** `web/public/defaultTheme/`。
6. **不要用 `npm install`** 替代 `npm ci`（`scripts/build-frontend.sh`；`frontend/package-lock.json` 已入库）。
   注意上游 CI（`.github/actions/build-frontend/action.yml:37`，该目录已移除）用的是 `npm install`，**不要照抄**。
7. **不要改版本号语义**：发版必须递增 patch 位（`0.0.1` → `0.0.2`），带后缀的 tag 前端永远识别不到更新。

## 7. Code review 关注点

按本仓库的实际薄弱环节排序：

1. **跨层数据流**：RPC 方法返回的结构体字段名（json tag）会直接变成前端契约，改 tag 等于改 API。参照 `web/rpc/jsonrpc/common.go:321-346` 那种显式 `recordLike` 结构。
2. **权限与敏感操作**：新增写接口是否挂了 `api.RequireRole` / `api.RequireSensitive2FA()`，路由登记处见 `web/router/router.go:82`、`:124`、`:140`。
3. **错误码选择**：`InvalidParams` vs `InternalError`（`error-handling.md` §2）。
4. **是否重复记日志**：RPC handler 不该有 `logger.*`（`logging-guidelines.md` §9）。
5. **数据库破坏性操作**：`internal/migrations/` 里的 `DropTable` / 新 `AutoMigrate` 登记（`database-guidelines.md` §2、§7）。
6. **测试是否真的在测**：本仓库没有 CI 拦截，"测试通过"必须是本地实跑的结果。
7. **是否顺手改了与任务无关的文件**——见 §8。

## 8. 已知遗留缺陷（别当成新问题，但也别扩散）

- ~~`web/public/.gitignore:4` 引用的 `docs/MAINTAINING-1.4.3.md` 已不存在~~ —— **已于 2026-09-16 修正**为 `docs/MAINTAINING.md`。教训：重命名文档时必须全仓 grep 引用（含 `.gitignore` 这类非 Markdown 文件），不能只查 Markdown。
- 14 个文件未过 `gofmt -l`（§3）。
- `.trellis/spec/backend/index.md` 的语言声明已改为中文（2026-09-16），与已填写规范一致。
