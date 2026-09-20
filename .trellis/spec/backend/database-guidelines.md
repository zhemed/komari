# 数据库约定（backend）

本文件描述本仓库**当前实际**的数据持久化约定。分两条独立链路，不要混用：

| 链路 | 存储 | 驱动 | 用途 |
|---|---|---|---|
| 主库 | SQLite `./data/komari.db`（可用 `-d` 改路径） | GORM + `gorm.io/driver/sqlite` | 账号、节点、配置、任务、通知、主题 |
| metric store | **独立** SQLite `./data/metrics.db` | 裸 `database/sql`，**不走 GORM** | 负载/GPU/ping 时序数据，可切 MySQL/PostgreSQL |

主线只支持 SQLite：`database/dbcore/dbcore.go:402-428` 的 switch 只有 `DatabaseTypeSQLite`，
其余走 `default` 返回 `unsupported database type: ... (supported: sqlite)`。
CLI 绑定见 `cmd/root.go:39-40`（`--db-type` / `--database`，默认 `./data/komari.db`）。

## 0. 本仓库自有的表（不是上游的）

- `client_traffic_totals`（`database/models/traffic.go`）：跨重启的流量累计。上游把“总流量”
  显示成 agent 的开机计数器（机器重启即归零），我们改为在服务端累加**重置感知增量**并落库。
  写入走 `database/clients/traffic.go`（内存 + SQLite），由 metricstore 批次写入的钩子驱动
  （`internal/server/metric_store.go` 装配）。改这条链路时注意：
  - 首次见到节点用计数器做**基线**，不重复累加当次增量；
  - 增量可能为 0（计数器没变），此时跳过写库但**不要**把行删掉；
  - 读路径是内存快照（`GetAllTrafficTotals`），重启时由 `InitTrafficTotals()` 重新加载。

## 1. GORM 模型风格（`database/models/*.go`）

**每个字段同时带 `json` 与 `gorm` tag，json 键一律 snake_case 且与列名一致。**

```go
// database/models/models.go:12
UUID  string `json:"uuid,omitempty" gorm:"type:varchar(36);primaryKey"`
// database/models/models.go:13
Token string `json:"token,omitempty" gorm:"type:varchar(255);unique;not null"`
```

既有约定：

- 字符串**显式写长度**：`type:varchar(N)`；长文本 `type:longtext`；大整数 `type:bigint`；百分比 `type:decimal(5,2)`（`database/models/notification.go:21`）。
- 主键：字符串用 `gorm:"primaryKey"`（`database/models/models.go:12`、`:49`）；整型用 `gorm:"primaryKey;autoIncrement"`（`database/models/log.go:6`、`database/models/clipboard.go:6`、`database/models/notification.go:17`、`database/models/pingTask.go:16`）。
- 复合索引写成命名索引 + priority：`index:idx_logs_msg_type_time,priority:1`（`database/models/log.go:10`）与 `priority:2`（`database/models/log.go:11`）。
- 默认值写在 **gorm tag**（数据库层默认），不只靠 Go 零值：`gorm:"default:false"`（`database/models/models.go:35`、`:40`）、`gorm:"type:varchar(20);default:'$'"`（`database/models/models.go:36`）、`gorm:"type:varchar(10);default:'max'"`（`database/models/models.go:42`）、`gorm:"type:boolean;default:false"`（`database/models/notification.go:9`）。
- 外键在父结构体上声明子切片，子表反向引用同款 tag：`Sessions []Session `json:"sessions,omitempty" gorm:"foreignKey:UUID;references:UUID;constraint:OnDelete:CASCADE,OnUpdate:CASCADE"``（`database/models/models.go:55`；同类见 `database/models/pingTask.go:7`、`:9`、`database/models/task.go:9`、`database/models/notification.go:7`）。
- 可空时间用 `*time.Time` + `gorm:"type:timestamp"`（`database/models/models.go:37`、`database/models/task.go:18`）。
- 列名与 json 键不一致时显式 `column:`：`DefaultOn bool `json:"default_on" gorm:"column:all_clients;not null;default:false"``（`database/models/pingTask.go:20`）——改列名时保持 json 键不变，前端契约不破。
- 变长数组字段存 JSON 文本列：`Clients StringArray `json:"clients" gorm:"type:longtext"``（`database/models/pingTask.go:19`、`database/models/task.go:7`、`database/models/notification.go:19`）。
- 结构化配置同样存 JSON 字符串列：`Addition string `json:"addition" gorm:"type:longtext" default:"{}"``（`database/models/oauth.go:5`；通知系统的 `messageSender.go` 已在 0.0.3 删除）、`Data string `json:"data" gorm:"type:longtext" default:"{}"``（`database/models/theme.go:68`）。

`TableName()` 全仓库**只有三处**（都在 configs 相关）：`internal/config/config.go:20-22` → `"configs"`、
`internal/migrations/migrations.go:46-48`、`:79-81`（`legacyModelConfig` / `legacyConfig`）→ `"configs"`。
`database/models/` 下**没有** `TableName()` 覆盖，表名全部交给 GORM 默认复数化
（`User`→`users`、`PingTask`→`ping_tasks`），所以手写 SQL 时必须自己对齐这个命名。

唯一的自定义 driver 类型是 `StringArray`（`database/models/models.go:114-138`）：
`Scan` 接受 nil/[]byte/string 三态并把空值规范化为空切片（`:116-134`），`Value` 直接 `json.Marshal`（`:136-138`）。

## 2. AutoMigrate 注册（`database/dbcore/dbcore.go`）

三批调用，**容错强度不同**：

| 批次 | 位置 | 行为 |
|---|---|---|
| 主批次 | `database/dbcore/dbcore.go:448-460` | 失败即 `return fmt.Errorf("failed to create tables: %w", err)`（`:461-463`） |
| Session | `database/dbcore/dbcore.go:464-468` | 失败只记 `Errorf`，理由注释 "it may already exist" |
| Task / TaskResult | `database/dbcore/dbcore.go:469-474` | 同上 |

主批次清单：`User`、`Client`、`Log`、`Clipboard`、`LoadNotification`、`OfflineNotification`、
`TrafficReportNotification`、`PingTask`、`OidcProvider`、`MessageSenderProvider`、`ThemeConfiguration`。

**`Record` / `GPURecord` / `PingRecord` 已不在注册列表**，原因写在 `database/dbcore/dbcore.go:441-446`：
负载/GPU/ping 历史数据运行期全部走 metric store，旧表不再建表、不再写入；
结构体保留作为 metric store 的读写 DTO 与旧表导入 DTO。

`configs` 表**不在这里**迁移，而是由 `config.SetDb` 自己 `AutoMigrate(&ConfigItem{})`
（`internal/config/config.go:26-31`，失败直接 panic），唯一调用点是 `database/dbcore/dbcore.go:433`。

新增一张表 → 建 `database/models/<name>.go` + 在 `database/dbcore/dbcore.go:448-460` 登记（见 `directory-structure.md` §4）。

## 3. 启动顺序（`doInitialize`，`database/dbcore/dbcore.go:341-477`）

顺序是**契约**，改动前先理解依赖：

1. `./data/backup.zip` 存在则先做恢复快照 + 解压（`database/dbcore/dbcore.go:345-387`）。
2. 采集 `dbFileExistedAtStartup`（`:389-394`），必须在恢复之后、`gorm.Open` 之前。
3. `gorm.Open`（`logConfig` 带 `NowFunc: time.Now().UTC()`，`:396-399`）。
4. `migrations.Run(...)`（`:430-432`）——**早于 AutoMigrate**。
5. `config.SetDb(instance)`（`:433`）。
6. `backupOnVersionUpgrade()`（`:437`）——**晚于 SetDb、早于 AutoMigrate**。
7. `AutoMigrate` 三批（`:448-474`）。

## 4. 配置键值存储（`internal/config`）

单表 `configs`，KV + JSON 值：

```go
type ConfigItem struct {
	Key   string `gorm:"primaryKey;column:key;type:text"`
	Value string `gorm:"column:value;type:text"` // 存 JSON 字符串
}
func (ConfigItem) TableName() string { return "configs" }
```

（`internal/config/config.go:15-22`。注意**故意没有 json tag**，值本身就是 JSON 文本。）

API 与语义：

- `GetAs[T](key, defaul ...any) (T, error)`（`internal/config/config.go:35-71`）：缺 key 且给了默认值时**顺带写库**并返回（`:42-54`）；读时先直接 `json.Unmarshal`，失败再走反射转换（`:59-69`）。
- `GetMany(keys map[string]any)`（`internal/config/config.go:76-130`）：`keys` 是 `map[key]defaultValue`；默认值 `nil` 表示不存在时不写库，非 nil 则批量 upsert（`:99-127`）。
- `GetManyAs[T]()`（`internal/config/config.go:135-235`）：**用结构体字段的 json tag 作为配置键，`default` tag 作为默认值**；只有带 `default` tag 的字段才会在缺失时写库（`:204-220`），没有 `default` 且缺失则保持零值不写（`:221`）。
- `Set` / `SetMany`（`internal/config/config.go:374-407`、`:409-458`）：都用 `clause.OnConflict{Columns: [{Name:"key"}], DoUpdates: AssignmentColumns(["value"])}` 做 upsert（`:396-399`、`:448-451`），成功后 `publishEvent(oldVal, newVal)` 广播（`:405`、`:456`）。
- 订阅：`config.Subscribe(func(ConfigEvent))`（`internal/config/config.go:529-534`），事件里用 `IsChanged` / `IsChangedT[T]` 判定（`:465-519`）。**订阅回调在独立 goroutine 执行**（`publishEvent` 里 `go sub(event)`，`:542`）。

`internal/config/settings.go` 是 KV 的结构化视图：`Settings` 字段带 `json` + `default` tag
（`internal/config/settings.go:5-42`），键名常量集中在 `internal/config/settings.go:44-78`。加载点是 `internal/server/bootstrap.go:28`
的 `config.GetManyAs[config.Settings]()`。

**新增配置项的正确写法**：在 `Settings` 加字段（`json` tag = 键名，`default` tag = 默认值），
再在同文件常量区加 `XxxKey`，不要散落字符串字面量。

## 5. SQLite 调优

### 5.1 统一走 `internal/sqlitetune`

`Options`（`internal/sqlitetune/connector.go:31-42`）声明全部 PRAGMA；
`Open(dsn, options)` 通过 `sqlite3.SQLiteDriver{ConnectHook: ...}` 对**每个物理连接**下发
（`internal/sqlitetune/connector.go:46-58`、`:107-114`）。`PRAGMA journal_mode = WAL` 恒定下发
（`internal/sqlitetune/connector.go:153`），其余按 Options 追加（`:148-177`）。`normalize` 会拒绝
`CacheSizeKB <= 0`、`WALAutoCheckpoint <= 0`、`JournalSizeLimitBytes < -1`（`:116-146`）。

### 5.2 主库实测取值（`database/dbcore/dbcore.go:179-184` 常量 + `:296-307`）

| 项 | 值 |
|---|---|
| BusyTimeout | 5s |
| CacheSizeKB | 8192（8 MB） |
| MMapSizeBytes | 0（显式禁用 mmap） |
| TempStoreMemory | false |
| CacheSpill | true |
| WALAutoCheckpoint | 256 |
| JournalSizeLimitBytes | 1 MB |
| Synchronous | `NORMAL` |

### 5.3 DSN 与连接池

- `buildSQLiteDSN`（`database/dbcore/dbcore.go:274-294`）：参数固定 `_busy_timeout=%d&_txlock=immediate`；
  `file:` 前缀原样追加，`:memory:` 归一为 `file::memory:?cache=shared&...`。
  注释解释了 `_txlock=immediate` 的用途（`database/dbcore/dbcore.go:404-406`）。
- 池子被**硬约束为 1**：`SetMaxOpenConns(1)` / `SetMaxIdleConns(1)` / 不限时（`database/dbcore/dbcore.go:414-417`），
  理由注释："SQLite has one writer. Keeping exactly one durable connection also keeps
  connection-local cache and WAL limits stable for the main DB."
- 启动时 `PRAGMA wal_checkpoint(TRUNCATE)`（`database/dbcore/dbcore.go:424-426`）。

### 5.4 空间回收（`database/dbcore/maintenance.go`）

`StorageSize()` 统计 db/-wal/-shm 三文件（`database/dbcore/maintenance.go:17-24`）；
`ReclaimSpace(ctx)` 在 `maintenanceMu` 下按 checkpoint → `VACUUM` → checkpoint 执行（`:26-46`），
checkpoint 会检查 busy 标志并报错而非静默继续（`:48-60`）。非 SQLite 直接返回错误（`:28-30`）。

## 6. 版本升级自动备份（`backupOnVersionUpgrade`）

版本标识存在配置库而非裸文件：`const SystemVersionKey = "system_version"`
（`database/dbcore/dbcore.go:174-177`），注入点是
`dbcore.SetVersionID(utils.CurrentVersion + "-" + utils.VersionHash)`（`internal/server/bootstrap.go:20`），
早于 `dbcore.Initialize()`。

触发规则（doc comment `database/dbcore/dbcore.go:209-222`，实现 `:223-265`）：

| 条件 | 行为 |
|---|---|
| `versionID` 为空 | 跳过（测试场景） |
| 配置无版本 且 启动前无库文件 | 全新安装：只写版本，**不备份**（`:237-241`） |
| 配置无版本 但 启动前已有库文件 | 从无版本标记的旧版升级：**备份** |
| 配置有版本且与当前不同 | **备份**（`:232-235` 的反向分支） |
| 配置有版本且与当前相同 | 直接 return（`:232-235`） |

备份动作：先 `PRAGMA wal_checkpoint(TRUNCATE)` 保证 `-wal` 已落主文件（`:243-248`），
再把 `./data` 打包为 `./data/backup/upgrade-{UTC时间戳}.zip`，排除 `backup.zip` 与 `backup/` 目录本身
（`:250-262`），最后 `writeVersionMarker()` 写回版本（`:264-272`）。

**备份失败只记 `Errorf`，不阻断启动**（`:252-253`、`:259-260`）——这是刻意选择，不要改成 fatal。

## 7. 启动迁移与"不删表"的真实边界

`internal/migrations/migrations.go:92-137` 的 `Run` 在 AutoMigrate **之前**执行一次性数据修补，
顺序：`migrateLegacyTimestampColumns` → （旧宽表 configs 存在时）`migrateLegacyOidcConfig`、
`migrateLegacyMessageSenderConfig` → `migrateLegacyClientInfo` → `migrateLegacyLoadNotification`
→ `migrateLegacyPingAllClientsExpansion` → （旧宽表时）`migrateLegacyConfigToItems`
→ `migrateDeprecatedMetricRetentionConfig` → `migrateRemovedCompatibilityConfig` → `markTimestampMigrationDone`。

**准确的现状表述**（不要写成"永不 DROP"）：运行期不删表；需要退场的表**优先改名保留**，
仅当数据已被迁移进新结构或必须重建同名表时才 DROP。

- 改名保留的正例：`db.Migrator().RenameTable("client_infos", "client_infos_backup")`（`internal/migrations/migrations.go:315`）+ 日志 `Data migration completed, old table has been backed up as client_infos_backup`（`:318`）。
- 重建同名表的反例：旧宽表 `configs` → KV 的迁移会 `DropTable("configs")` 后重建（`internal/migrations/migrations.go:328`），在事务内先落数据；`LoadNotification` 旧形状也会被重建（`internal/migrations/migrations.go:178-184`）。
- 旧监控 4 表 `records / records_long_term / gpu_records / ping_records`（`internal/migrations/legacy_monitoring.go:27`）在启动阶段**不删**，只在管理员显式执行升级向导后才删（`internal/migrations/legacy_monitoring.go:178-195`，drop 实现 `:666-677`）。
- 兼容项清理删的是**配置行**而非表：`migrateDeprecatedMetricRetentionConfig` 删 `metric_retention_days`（`internal/migrations/migrations.go:139-144`）、`migrateRemovedCompatibilityConfig` 删 `nezha_compat_enabled` / `nezha_compat_listen` / `low_resource_mode`（`:146-155`）。
- 时间戳迁移带幂等标记，避免重复执行：`timestampUTCMigrationKey = "internal_timestamp_utc_migrated"`（`internal/migrations/timestamp.go:18`，写入 `:213-238`）。

### 7.1 孤儿表保留

移除插件系统时**没有**删除数据库里的插件表；`build-and-pinning.md` §1.5 明确写了
"数据库中的历史插件表（孤儿表，保留即可，不做破坏性迁移）"。新增模型时不要顺手 `DropTable` 任何已有表。

## 8. 时序数据的真实落点（metric store）

- 运行期唯一写入路径是 metric store：`database/records/records.go` 全部函数转发到 `internal/metricstore`，文件头注释说明旧表只在启动一次性迁移时作为数据源读取（`database/records/records.go:10-16`）。
- 默认目标库 `./data/metrics.db`（`internal/metricstore/config.go:31` 的 `default:"./data/metrics.db"`），**不使用 GORM**：`pkg/metric/migrations.go:34-75` 手写 `CREATE TABLE IF NOT EXISTS`，`Migrate` 里 SQLite 额外 `PRAGMA optimize`（`:27-30`）。
- 配置键集中在 `internal/metricstore/config.go:44-56`（`metric_db_driver`、`metric_db_dsn`、`metric_table_prefix`、`metric_max_open_conns`、`metric_max_idle_conns`、`metric_rollup_*_retention_*`、`metric_migration_target`），通过 `config.GetManyAs[MetricStoreConfig]()` 读取。
- SQLite 分支会被改写为 `file:./data/metrics.db?mode=rwc&_txlock=immediate`（`internal/metricstore/config.go:96`），并强制单连接；`cache=shared` 被刻意剥离（`internal/metricstore/config.go:87-100` 及 `stripSharedCache`）。
- 表名 = `metric_` 前缀 + definitions/labels/series/resolutions/rollups/store_state（`pkg/metric/store.go:113-121`；建表语句见 `pkg/metric/migrations.go:39-74`）。
- 清空类 API 语义是"删数据不删定义"：`DeleteAllRecords`（`internal/metricstore/deletion.go:18`）、`DeleteAllPingRecords`（`:35`）。

**改动 metric store 的 schema 要走受限升级向导重建设计，不要在启动时静默改**
（`pkg/metric/migrations.go:13-14` 明确写了 "Existing installations are rebuilt explicitly by
the administrator guide, rather than being changed during startup"）。

## 9. 时间约定

- 主库 GORM 全局 `NowFunc` 强制 UTC：`NowFunc: func() time.Time { return time.Now().UTC() }`（`database/dbcore/dbcore.go:398`）。业务写入点也显式 `.UTC()`：`database/auditlog/log.go:18`、`database/accounts/sessions.go:36` 等。
- metric store 全部以 **UTC 毫秒整数**存储：`timeMillis` / `fromMillis`（`pkg/metric/time_millis.go:5-7`），列名带 `_milli` 后缀（`resolution_milli`、`bucket_milli`、`created_at_milli`，`pkg/metric/migrations.go:43`、`:56`、`:59`）。
- 展示/日历计算用**系统本地时区**，集中在 `pkg/timeutil`（`pkg/timeutil/timeutil.go:8-31` 的 `SameSystemDate` / `FormatSystemDate` / `SystemDateDistance`）。`internal/migrations/timestamp.go:198-199` 的注释点明了这条分工：旧自定义类型的时区逻辑仅是迁移适配，运行期用系统本地时区。
- 备份文件名用 UTC 时间戳：`time.Now().UTC().Format("20060102-150405")`（`database/dbcore/dbcore.go:255`、`:353`）。

**写新代码时**：入库前 `.UTC()`；只做"今天/昨天"这类日历判断时用 `pkg/timeutil`，不要自己 `time.Local` 算。

## 10. 常见错误

- 在主库上开多个写入连接 → 与 `SetMaxOpenConns(1)` 的设计冲突，触发 `SQLITE_BUSY`。
- 忘记在 `AutoMigrate` 列表登记新模型 → 表永远不存在，只在运行时炸。
- 用 `GetAs` 读一个本该有默认值的键而不给默认值 → 多实例/首次读时行为不一致（默认值写库只在给了 `default` 时发生）。
- 把 `Record` / `GPURecord` / `PingRecord` 重新加回 `AutoMigrate` → 与 metric store 双写，见 `database/dbcore/dbcore.go:441-446`。
- 在 `internal/migrations/` 里 `DropTable` 一张还有数据的表 → 违反 §7 的现状；退场表优先 `RenameTable` 成 `*_backup`。
- 直接外泄数据库连接错误字符串 → 走 `internal/metricstore/redact.go:13-22` 的 `RedactConnectionError`。
- 把时间当本地时区写进主库或 metric store → 后续跨时区查询与 `SystemDateDistance` 都会错位。

---

## 附：外部工具读取 SQLite 的两个坑（2026-09-16 实测）

本仓库的 `./data/komari.db` 跑在 WAL 模式（`internal/sqlitetune/connector.go:153` 的
`PRAGMA journal_mode = WAL`），排查线上问题时容易踩到：

1. **外部 sqlite3 读到"旧"数据**：WAL 模式下，只在 `-wal` 里、尚未 checkpoint 的写入对
   新连接是可见的（SQLite 保证），所以正常情况下外部读不会落后。但升级重启过程中出现过
   `komari.db-wal` 被 unlink、进程仍持有其 fd 的偶发状态，此时外部读会停在升级前——
   **先重启一次服务端再判断**，别急着下"数据丢了"的结论（详见 `docs/MAINTAINING.md` §7）。
2. **不要用 `cp` 直接拷 `komari.db` 当备份**：WAL 里的写入可能不在主文件里。
   面板的备份走 `VACUUM INTO`（`web/api/admin/download.go:119`）拿到一致性快照。
