# 日志约定（backend）

全进程只有**一个**运行时日志出口：`utils/log`（包名 `logger`）。不要直接使用 `log`、`fmt.Println`
或裸 `slog`。

## 1. 包与导入

- 目录是 `utils/log`，**包名是 `logger`**（`utils/log/log.go:1-2`：
  "Package logger provides the server's single runtime logging surface."）。
- 因此导入一律带别名，仓库内 **37 处导入全部**写成：
  `logger "github.com/komari-monitor/komari/utils/log"`（无一处例外）。新增文件照写。

## 2. 初始化

`logger.Setup(level slog.Level)`（`utils/log/log.go:127-132`）做三件事：
装 `ConsoleHandler` 到 `os.Stdout`、`slog.SetDefault`、把 MySQL driver 的日志接到同一出口
（`_ = mysql.SetLogger(mysqlDriverLogger{})`，实现见 `utils/log/log.go:136-140`，固定走 `ErrorArgs("mysql", ...)`）。

全仓库**只有** `main.go` 调用它（`main.go:12-16`）：

```go
if utils.VersionHash == "unknown" {
	logger.Setup(slog.LevelDebug)
} else {
	logger.Setup(slog.LevelInfo)
}
```

即：**未注入版本 hash 的本地构建自动降到 DEBUG**，正式构建是 INFO。

未调用 `Setup` 时 `logger()` 会退回 `slog.Default()`（`utils/log/log.go:142-147`）——测试里替换
`defaultLogger` 是既有手法（`utils/log/log_test.go:21-24`）。

## 3. API 形状

```go
func Info(component, message string, args ...any)          // utils/log/log.go:156
func Debug / Warn / Error(component, message string, args ...any)  // :153-164
func Debugf / Infof / Warnf / Errorf(component, format string, args ...any)  // :166-169
func InfoArgs / WarnArgs / ErrorArgs(component string, args ...any)         // :171-173
func Fatalf(component, format string, args ...any)         // :175-178
func DebugContext / InfoContext / WarnContext / ErrorContext(ctx, component, message, args...)  // :180-194
```

**第一个参数永远是模块名（component），不是格式化字符串。**

两种尾部参数风格语义不同，不要混用：

| 写法 | 尾部参数去哪 | 输出 |
|---|---|---|
| `logger.Info("metricstore", "store initialized", "driver", "sqlite", "points", 12)` | 变成结构化字段 | `... store initialized driver=sqlite points=12` |
| `logger.ErrorArgs("notifier", "Failed to send offline notification:", err)` | 用 `fmt.Sprint` **拼进 message**（`utils/log/log.go:171-173`） | `... Failed to send offline notification:<err>` |

`*f` 变体同理走 `fmt.Sprintf` 进 message（`utils/log/log.go:166-169`），不产生字段。

## 4. 输出格式

`ConsoleHandler.Handle`（`utils/log/log.go:34-70`）实际输出：

```
2026/07/21 10:51:52 [INFO/SERVER] Starting server on 0.0.0.0:25774 ...
```

（格式示例写在 `utils/log/log.go:17-18` 的注释里。）

- 时间戳：`record.Time.Local().Format("2006/01/02 15:04:05")`（`utils/log/log.go:62`）——**本地时区**。
- component 被 `strings.ToUpper`（`utils/log/log.go:46`），所以写 `"server"` 显示成 `SERVER`。
  既有取值本身是小写（§5），**不要**自己传大写。
- 日志级别名由 `levelName` 决定（`utils/log/log.go:87-98`）：
  `<= Debug` → `DEBUG`，`< Warn` → `INFO`，`< Error` → `WARN`，否则 `ERROR`。
  颜色由 `levelColor` 决定（`:100-111`）：青/绿/黄/红。
- 值里含空格、tab、CR、LF 或引号时会被 `%q` 包起来（`formatValue`，`utils/log/log.go:113-124`）。
- 级别门限：`Enabled` 就是 `level >= h.level`（`utils/log/log.go:30-32`）。
- 输出目标是 **stdout**（`utils/log/log.go:128`），不是 stderr。

`utils/log/log_test.go:20-41` 把上面这些当契约断言（`[INFO/METRICSTORE]`、`driver=sqlite`、`points=12`）。

## 5. 模块名（component）的实际取值

按调用点统计，既有取值与量级如下——**新增日志请复用已有名字，不要造新词**：

| component | 出现次数 | 典型位置 |
|---|---|---|
| `server` | 34 | `internal/server/runtime.go:109`、`:124`、`cmd/server.go:38-120` |
| `notifier` | 19 | `utils/notifier/traffic_report.go:28`、`utils/notifier/offline.go:115` |
| `metricstore` | 19 | `internal/metricstore/*` |
| `dbcore` | 18 | `database/dbcore/dbcore.go:252`、`:262`、`:324` |
| `migration` | 16 | `internal/migrations/migrations.go:180`、`:420` |
| `geoip` | 13 | `utils/geoip/mmdb.go:179` |
| `client-api` | 9 | `web/api/client/report.go:102`、`:158` |
| `oauth` | 8 | `web/oauth/factory/factory.go:22` |
| `config` | 6 | `internal/config/config.go:108`、`:125`、`:202` |
| `scheduler` | 2 | `internal/scheduler/scheduler.go:206` |
| `clients` | 2 | `database/clients/*` |
| `audit` | 2 | `database/auditlog/log.go:21`、`:34` |
| `terminal` / `reload` / `install` / `database` / `admin-api` | 各 1 | `web/api/terminal/request.go:49`、`internal/server/reload.go:63`、`web/install/install.go:105`、`web/api/admin/archive_upload.go:42` |

由 `utils/log` 自己使用的固定组件名（不要在外面复用）：
`gin`（`utils/log/gin.go:25-29`）、`http`（`utils/log/gin.go:38`）、`gorm`（`utils/log/gorm.go:41-73`）、`mysql`（`utils/log/log.go:139`）。

## 6. 各级别的实际用法

- **Info**：生命周期里程碑与"改了就值得知道"的状态变更。
  `logger.Infof("server", "Starting server on %s ...", a.listenAddr)`（`internal/server/runtime.go:109`）、
  `logger.Infof("dbcore", "[upgrade-backup] ./data backed up to %s before upgrade (from %q to %q)", ...)`（`database/dbcore/dbcore.go:262`）。
- **Warn**：可继续、但说明配置或数据有问题的分支。典型是配置 marshal/unmarshal 失败：
  `logger.Warn("config", "unmarshal config failed", "key", fi.key, "error", err)`（`internal/config/config.go:202`，
  同文件 `:108`、`:125`、`:207`、`:213`、`:230`）。
- **Error**：功能失败。
  `logger.Errorf("server", "Failed to get OIDC provider config: %v", err)`（`internal/server/runtime.go:64`）、
  `logger.Errorf("server", "cleanup %q failed: %v", cleanup.name, err)`（`:168`）、
  `logger.ErrorArgs("client-api", "Failed to read request body:", err)`（`web/api/client/report.go:102`）。
- **Debug**：**当前全仓库 0 处**（唯一的使用者 `pkg/jsruntime` 已在 0.0.3 删除）。
  需要排查问题时可以临时加，但不要把它当常规输出——真要留就得顺手更新这张表。
- **Fatalf**：仅启动期不可恢复失败。全仓库 **9 处**：`cmd/server.go:38`、`:44`、`:50`、`:63`、`:76`、
  `:89`、`:111`、`:120` + `database/dbcore/dbcore.go:324`。
  它记 ERROR 后直接 `os.Exit(1)`（`utils/log/log.go:175-178`）。可恢复失败一律不用。

## 7. 结构化字段 vs 拼接

需要机器可读的上下文时用键值对形式，键用小写：

```go
logger.Error("http", "panic recovered", "error", fmt.Sprint(err), "method", c.Request.Method, "path", c.Request.URL.Path)  // utils/log/gin.go:38-42
```

只有"这是一句人话 + 一个 err"时才用 `ErrorArgs` / `Errorf`（例：`internal/scheduler/scheduler.go:206`
`logger.Errorf("scheduler", "corn job %s panic: %v", name, r)`）。
既有代码以拼接风格为主（`ErrorArgs`/`Errorf`/`Infof` 占多数），**新代码跟随所在文件的既有风格**。

注意 `logArgs` 只做一件事：把 `component` 塞到字段列表最前面（`utils/log/log.go:149-151`），
所以 `logger.Info("c", "m", "k", v)` 里的 `"k", v` 必须是成对的，否则输出会变形。

## 8. 框架日志的接管

三个框架的日志**已经**被接进同一出口，不要额外再包一层：

- **Gin 访问日志** `logger.GinLogger()`（`utils/log/gin.go:12-32`）：
  message 形如 `<status> <METHOD> <path> | <clientIP> | <duration>`，
  并按状态码选级别：`>= 500` → `Error("gin", ...)`，`>= 400` → `Warn("gin", ...)`，否则 `Info`。
  `c.Errors` 非空时追加到 message。
  **query string 被刻意丢弃**（注释 `utils/log/gin.go:10-11`），并有回归测试守着：
  `utils/log/gin_test.go:32-34` 断言日志里不得出现 `token=secret`。
- **panic 恢复** `logger.GinRecovery()`（`utils/log/gin.go:34-48`）：见 `error-handling.md` §6。
- **GORM** `logger.NewGormLogger()`（`utils/log/gorm.go:22-31`）：
  `SlowThreshold` 200ms、`IgnoreRecordNotFoundError` true、`LogLevel: gormlogger.Warn`
  ——**成功查询默认不打**（注释 `utils/log/gorm.go:26-28`）。`Trace`（`:57-74`）按 失败/慢查询/Info 级别
  分别输出 `query failed` / `slow query` / `query completed`，字段含 `elapsed`、`rows`、`sql`、`source`。
  要看每一条 SQL 得显式 `LogMode(gormlogger.Info)`。

装配点在 `internal/server/runtime.go:88-89`：

```go
r := gin.New()
r.Use(logger.GinLogger(), logger.GinRecovery())
```

`gin.New()` 而非 `gin.Default()` —— 因为默认中间件已被上面两个替换。

## 9. 什么不该记

- **URL query**：`GinLogger` 已经全部丢弃；不要在 handler 里把 `c.Request.URL.RawQuery` 打进日志。
- **凭据 / DSN**：数据库连接串不能原样外泄。`internal/metricstore/redact.go:13-22` 提供
  `RedactConnectionError(message, dsn)`，会抹掉 DSN 本体、`password=` 形式与 `user:pass@host` 里的密码；
  `internal/metricstore/store_migration.go:289-316` 是同一意图的另一处实现。
  记连接失败前先过脱敏。
- **token / session / 密码**：`models.Client.Token`、`User.Passwd`、`TwoFactor`、`session_token`
  cookie 都不进日志。RPC 层已把敏感字段清空后再返回（`web/rpc/jsonrpc/common.go:248-251`），日志层不要补记。
- **业务错误不要重复记**：`web/rpc/jsonrpc/` 里 **0 处** `logger.*` 调用——RPC handler 只返回
  `*rpc.JsonRpcError`，由 `GinLogger` 按最终 HTTP 状态码记一次。在 handler 里再 `logger.Error` 就是重复。
- **安全审计事件走另一个通道**：用户可见的审计记录用 `database/auditlog`（`auditlog.Log(ip, uuid, message, msgType)`，
  `database/auditlog/log.go:11-27`），它写进 `logs` 表、保留 30 天（`:29-35`），
  与 `utils/log` 的 stdout 日志是两件事。调用点例：`web/rpc/jsonrpc/admin.client.go:100`、
  `internal/server/runtime.go:137`、`:154`。

## 10. 常见错误

- 忘了第一个参数是 component，写成 `logger.Infof("starting on %s", addr)` → 输出里 `starting on %s` 变成模块名。
- 用 `logger.Info("server", "msg", "key")` 只给键不给值 → 输出出现悬空 `key=`。
- 新增 `logger.Debug(...)` 指望排查时能看到 → 正式构建是 INFO，看不到；要用 Info 或加临时字段。
- 在 RPC handler 里 `logger.Error` + 返回错误 → 与 `GinLogger` 的 5xx 记录重复（§9）。
- 把 `Fatalf` 用在请求处理路径 → 直接杀进程，见 §6。
- 自己 `fmt.Sprintf` 拼好整句再传 `logger.Info` → 丢失结构化字段，排查时无法按 key 检索。
- 直接 `slog.Info(...)` → 绕过 `ConsoleHandler`，格式与其余日志不一致（`utils/log/log.go:34-70` 的 component 大写、
  级别着色全部失效）。
