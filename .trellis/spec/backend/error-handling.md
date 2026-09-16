# 错误处理（backend）

本文件描述本仓库**实际**的错误传播与返回习惯。核心事实：对外接口几乎全部收敛到 JSON-RPC 2.0 的
错误对象，gin 侧只做包装与映射；handler 自身**不打日志**。

## 1. 三层错误边界

```
业务函数 (database/*, internal/*)   → 返回 error（可能带 %w 包装）
        ↓
RPC handler (web/rpc/jsonrpc/*)     → 返回 (result, *rpc.JsonRpcError)，不写日志
        ↓
传输层 (transport.go / bridge.go)   → 决定 HTTP 状态码与响应体形状
        ↓
logger.GinLogger()                  → 由 4xx/5xx 状态码决定 WARN/ERROR 记录
```

`web/rpc/jsonrpc/` 目录内**没有任何** `logger.*` 调用（唯一出现 logger 的是
`web/rpc/jsonrpc/admin.system_test.go:39` 里的 gorm logger），这正是"handler 不记日志"的证据。

## 2. RPC 错误对象

定义在 `pkg/rpc/errors.go:8-12`：

```go
type JsonRpcError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}
```

- 标准码：`ParseError -32700`、`InvalidRequest -32600`、`MethodNotFound -32601`、
  `InvalidParams -32602`、`InternalError -32603`（`pkg/rpc/errors.go:15-21`）。
- 业务码：`PermissionDenied -32041`、`Unauthenticated -32040`、`NotFound -32044`、
  `AlreadyExists -32045`、`Aborted -32021`、`Unavailable -32051` 等（`pkg/rpc/errors.go:24-36`）。
- 构造用 `rpc.MakeError(code, msg, data)`（`pkg/rpc/errors.go:39-41`）。

**实际使用分布**（`web/` 全目录统计）：`InvalidParams` 104 次、`InternalError` 90 次、
`NotFound` 11 次、`PermissionDenied` 4 次、`InvalidRequest` 4 次、`Unauthenticated` 1 次。
也就是说：**参数错误用 InvalidParams，内部失败用 InternalError**，其余码用得很少。

## 3. RPC handler 的写法

handler 签名固定为 `func(ctx context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError)`
（`pkg/rpc/registry.go:11-12`）。

真实例子 `web/rpc/jsonrpc/admin.client.go:80-103`：

```go
if err != nil {
	return nil, rpc.MakeError(rpc.InternalError, err.Error(), nil)
}
```

既有约定：

- 成功返回 `(result, nil)`，失败返回 `(nil, errObj)`。
- `err.Error()` 常被直接放进 `Message` 或 `Data`（`web/rpc/jsonrpc/admin.client.go:96`、
  `web/rpc/jsonrpc/common.go:275` 的 `rpc.MakeError(rpc.InternalError, "Failed to get public info", err.Error())`）。
  **不要**在这层为了"安全"擅自改写上游错误文案，前端依赖这些 message。
- 参数校验失败统一先返回 `InvalidParams`，例：`adminEditClient` 的
  "Invalid params" / "Invalid or missing UUID"（`web/rpc/jsonrpc/admin.client.go:107-113`）。
- 需要审计的写操作在**成功之后**追加 `auditlog.Log(...)`，用 `auditActor(ctx)` 取 actor/IP
  （`web/rpc/jsonrpc/admin.client.go:71-78`、`:99-101`）。错误路径不写审计。
- handler 内部**不要** `recover()`：panic 交给 gin 中间件（§6）。

## 4. 分发与鉴权错误

`Dispatch` 是所有入口的唯一分发点（`web/rpc/jsonrpc/dispatch.go:23-58`），顺序为
私有站点检查 → 命名空间权限校验 → `rpc.CallWithContext`。它**始终返回**完整响应：

- 私有站点下未认证访客：`rpc.ErrorResponse(req.ID, rpc.PermissionDenied, "Private site enabled, please login first", nil)`（`web/rpc/jsonrpc/dispatch.go:45-49`）。
- 权限不足：`rpc.ErrorResponse(req.ID, rpc.PermissionDenied, "Permission denied", nil)`（`web/rpc/jsonrpc/dispatch.go:52-55`）。
- 方法不存在：`MethodNotFound`（`pkg/rpc/invoke.go:41-43`）。

敏感方法（2FA）在 `dispatchWithSensitive` 中补二次校验，返回同一错误对象而非 HTTP 401：
`rpc.ErrorResponse(req.ID, rpc.PermissionDenied, err.Error(), nil)`（`web/rpc/jsonrpc/transport.go:50-62`）。

`internal/server` 与 `web` 之间的重启握手用哨兵错误 + `errors.Is`：
`ErrRestartRequested`（`internal/server/runtime.go:34-36`）在 `cmd/server.go:116` 被识别。

## 5. HTTP 状态码与响应体形状

### 5.1 RPC → HTTP 映射（`web/rpc/jsonrpc/bridge.go:113-125`）

| RPC 错误码 | HTTP |
|---|---|
| `InvalidParams` / `InvalidRequest` / `ParseError` | 400 |
| `PermissionDenied` / `Unauthenticated` | 401 |
| `NotFound` | 404 |
| 其它（含 `InternalError`） | 500 |

错误体统一为 `{"status":"error","message":<Message>}`，与 `api.RespondError` 一致
（`web/rpc/jsonrpc/bridge.go:127-132`）。注意 `Data` 字段在这里**不**外泄。

成功体有三种渲染器：`renderStandard`（默认，`{status,message,data}`）、
`WithFlat()`、`WithRaw()`（`web/rpc/jsonrpc/bridge.go:19-26`、`:133-151`）。

### 5.2 请求体损坏

`Bind` 里 JSON 解析失败直接 400：
`{"status":"error","message":"Invalid or missing request body"}`（`web/rpc/jsonrpc/bridge.go:66-70`）。

`/api/rpc2` 直连入口是例外：`servePost` 读体失败回 400 但body 是 **JSON-RPC error 信封**
（`rpc.ErrorResponse(nil, rpc.ParseError, "read body error", err.Error())`，
`web/rpc/jsonrpc/transport.go:121-130`）。

### 5.3 保留的 REST handler

用 `web/api/Common.go:9-32` 的 `api.Response`：

```go
api.RespondError(c, http.StatusInternalServerError, "读取主题目录失败: "+err.Error())
```

真实调用点：`web/api/admin/theme.go:43`、`web/api/admin/theme.go:76-101`、`web/api/admin/2fa.go:15-50`。
约定是**中文前缀 + `err.Error()` 拼接**（theme 系）；`2fa.go` 则用英文短句。同一文件内保持一致。

`api.RequireRole` / `api.RequireSensitive2FA` 这类中间件失败时 `RespondError` + `c.Abort()`
（`web/api/AuthSensitive.go:13-24`，状态码 401）。

## 6. panic 与 recover

启动期装的两个 gin 中间件：`logger.GinLogger(), logger.GinRecovery()`
（`internal/server/runtime.go:88-89`）。

`GinRecovery` 的实际行为（`utils/log/gin.go:34-48`）：`recover()` → 记 `Error("http", "panic recovered", ...)`，
字段含 `error`/`method`/`path` → `c.AbortWithStatus(500)`。**不**回 JSON body，也**不**重抛。

允许 panic 的场合（全仓库仅此几处）：

| 位置 | 场景 |
|---|---|
| `internal/config/config.go:29` | `SetDb` 时 `AutoMigrate(ConfigItem)` 失败——配置库不可用等于不可启动 |
| `web/public/public.go:130` | `defaultTheme` 嵌入目录缺失，构建/部署错误，早期暴露 |
| `utils/geoip/geoip.go:60` | 读取不到 GeoIP 配置 |
| `web/oauth/factory/factory.go:19` | 注册项构造返回 nil |
| `pkg/rpc/registry.go:44` | `MustRegister` 便捷注册失败（仅 `init()` 期使用） |

（历史）`pkg/jsruntime/**` 曾用 `panic(vm.NewGoError(...))` 抛 goja 的 JS 异常；该包已在 0.0.3
删除，仓库里现在没有任何"用 panic 跨 JS 边界"的场景。

其它受控 recover 点（panic 不跨边界）：

- `internal/scheduler/scheduler.go:203-212`：`safeRun` 隔离单个 cron job 的 panic，记 `Errorf("scheduler", "corn job %s panic: %v", ...)`。
- `internal/server/reload.go:59-67`：`dispatch` 隔离单个配置 reload handler 的 panic，记 `Errorf("reload", ...)`。
- `web/api/client/report_v2.go:211-220`：通知回调丢弃 panic（`defer func() { _ = recover() }()`）。

## 7. 启动期错误：Initialize 返回 error，GetDBInstance 直接退出

`database/dbcore/dbcore.go` 提供两个入口，语义不同：

- `Initialize() error`（`:312-317`）：返回错误，供启动生命周期与测试/CLI 使用。
- `GetDBInstance() *gorm.DB`（`:322-326`）：失败即 `logger.Fatalf("dbcore", ...)` 退出进程。

启动路径因此**只**用 `Initialize()`，并把阶段名打进错误（`cmd/server.go:36-51`：

```go
logger.Fatalf("server", "server startup failed at %q: %v", "bootstrap", err)
```

阶段名取值：`bootstrap`、`detect-first-run-install`、`run-first-run-install`、
`database-migration-detection-recovery`、`run-database-migration`、`metric-store-recovery`（`cmd/server.go:38-111`）。
`logger.Fatalf` 全仓库只有 9 处（`cmd/server.go` 8 处 + `database/dbcore/dbcore.go:324`），
**新增可恢复失败不要用 Fatalf**。

## 8. 包装与判定

- 从底层往上传递时用 `fmt.Errorf("...: %w", err)`，前缀写清动作：`database/dbcore/dbcore.go:410`、
  `:422`、`:431`、`:462`、`internal/sqlitetune/connector.go:71`、`:110`、`internal/server/bootstrap.go:17-30`。
- 判定用 `errors.Is` / `errors.As`：`internal/server/runtime.go:215`（`metricstore.ErrCompactInProgress`）、
  `web/rpc/jsonrpc/admin.misc.go:190`（`metricstore.ErrStructureUpgradeRequired`）、
  `web/rpc/jsonrpc/transport.go:105`（`errors.As` 到 `*json.SyntaxError` / `*json.UnmarshalTypeError`）。
- 哨兵错误定义在包内：`internal/metricstore/store.go:26-30`、`internal/server/runtime.go:34-36`、
  `web/upload/handler.go:122` 使用的 `ErrNotFound`。

## 9. 敏感信息的脱敏

数据库连接错误**不允许**原样外泄或落日志：`internal/metricstore/redact.go:13-22` 的
`RedactConnectionError(message, dsn)` 会抹掉 DSN 本体、`password=` 形式与 `user:pass@host` 中的密码。
写入配置或日志前调用它，不要自己写正则。

## 10. 常见错误

- 在 RPC handler 里 `logger.Error(...)` 之后再返回错误 → 与 `GinLogger` 的 5xx 记录重复；handler 不记日志。
- 把 `InternalError` 当成"参数不对"的兜底 → 前端无法区分；参数问题一律 `InvalidParams`。
- 忘写 `api.RequireSensitive2FA()` 却让 handler 直接执行敏感动作 → 见 `web/router/router.go:124`、`:140` 的正确写法。
- 模仿已删除的 goja 桥那样用 `panic` 传业务错误 → 没有 JS 边界，会直接 500 且丢上下文；业务错误用返回 error。
- 新增可恢复失败却用 `logger.Fatalf` → 进程直接退出，绕过 §7 的阶段化错误上报。
