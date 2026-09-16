# 目录结构与分层约定（backend）

本文件描述**本仓库当前实际的代码分层**，不是理想架构。新增 Go 文件前先按 §4 的决策表定位。

模块路径：`github.com/komari-monitor/komari`（`go.mod:1`），Go `1.25.0`（`go.mod:3`）。
入口：仓库根 `main.go`（`package main`），不是 `cmd/`。

## 1. 顶层布局

| 目录 | 职责 | 关键锚点 |
|---|---|---|
| `main.go` | 进程入口：装日志 → 打印版本 → `cmd.Execute()` | `main.go:11-20` |
| `cmd/` | cobra 命令与启动流程编排；只做串联与错误上报 | `cmd/root.go:31-36`、`cmd/server.go:34-120` |
| `internal/` | **进程内**基础设施：配置、生命周期阶段、调度器、SQLite 调优、启动迁移、metric store 门面 | `internal/server/app.go:17-30` |
| `pkg/` | 可独立复用的库：RPC 协议内核、metric 存储引擎、JS 运行时、时间工具 | `pkg/rpc/rpc.go:1-5`、`pkg/metric/store.go:20-23` |
| `database/` | 业务数据访问层：GORM 模型 + 各资源的读写函数 | `database/dbcore/dbcore.go:448-460` |
| `web/` | HTTP/RPC 接入层：路由、REST handler、RPC2 方法、WS、静态前端 | `web/router/router.go:18-30` |
| `protocol/` | 与 agent 通信的线协议结构体（v1 上报、v2 JSON-RPC/网络测试） | `protocol/v1/report.go:1-3`、`protocol/v2/jsonrpc.go:1-6` |
| `utils/` | 通用工具：日志、通知、消息发送、GeoIP、续费提醒 | `utils/log/log.go:1-2` |
| `scripts/` | 构建与前端再生成脚本（非 Go 代码） | `scripts/build-komari.sh:1-13` |
| `docs/` | 唯一一份维护说明 `MAINTAINING.md` | `docs/MAINTAINING.md:1-11` |
| `web/public/defaultTheme/` | **vendor 进仓库的前端产物**，被 `//go:embed` 引用，见 `build-and-pinning.md` §1.1 | `web/public/public.go:18-19` |

`.build/`、`bin/` 是本机构建缓存与产物目录，已被忽略（`.gitignore:43-45`），不要往里放源码。

## 2. 分层与依赖方向（实测）

调用链基本单向，从外到内：

```
cmd → internal/server → web/router → web/rpc/jsonrpc + web/api/*
                                   ↓
                                database/* → database/models
                                   ↓
                                internal/metricstore → pkg/metric
```

实测的跨层 import 事实：

- `internal/server/` **会** import `web/*`：`internal/server/runtime.go:27-31` 引入 `web/api`、`web/oauth`、`web/recovery`、`web/router`、`web/security`。即 `internal/server` 是"组装根"，`internal/` 的其它包不应回头依赖它。
- `web/rpc/jsonrpc/*` 依赖 `database/*`、`internal/config`、`pkg/rpc`、`web/agent`（`web/rpc/jsonrpc/common.go:11-20`），**不**直接写 SQL。
- `database/records/records.go:6-8` 依赖 `internal/metricstore`：所有负载/GPU/ping 时序数据走 metric store，不再走 GORM 旧表。
- `pkg/` 基本自洽：`pkg/metric/store.go:17` 只向下用 `internal/sqlitetune`；`pkg/rpc/context.go:10` 只依赖 `database/models`（共享 DTO）。
- `pkg/jsruntime` 里的 `panic` 是 goja 抛 JS 异常的**正确**机制，不要当错误处理反例改掉。
- `utils/notifier/traffic.go:15`、`utils/pingSchedule.go:12`、`utils/renewal/renewal.go:13` 反向依赖 `web/agent` 的在线状态——这是既有耦合，新增 `utils/` 包时不要扩大这类反向依赖。

## 3. 命名与放置约定

### 3.1 RPC2 方法文件按「命名空间.主题」命名

`web/rpc/jsonrpc/` 下 30 个 Go 文件遵循 `namespace.topic.go`：

- `admin.client.go`、`admin.ping.go`、`admin.notification.go`、`admin.xtermjs.go`
- `public.metric.go`、`public.audit.go`
- `common.record.go`
- 无主题的聚合文件直接叫 `common.go`、`client.go`
- 测试沿用同名前缀加 `_test.go`：`admin.database_test.go`、`public.audit_test.go`

方法名一律 `group:name`，由 `RegisterWithGroupAndMeta` 拼接（`web/rpc/jsonrpc/register.go:16-27`）。
新增一个 admin 类方法 → 放进最贴切的 `admin.<topic>.go`，在该文件 `init()` 里注册。

### 3.2 每个 RPC 文件顶部写一行职责注释

既有惯例是文件首行注释给出该文件的来源与边界，例如
`web/rpc/jsonrpc/admin.client.go:14-16` 说明"承载原 `web/api/admin/client.go` 的业务逻辑"。
新增文件照此写。

### 3.3 `internal/` vs `pkg/` 的判定

- 只在**本进程**内使用、会读配置或全局状态 → `internal/`（例：`internal/config`、`internal/scheduler`）。
- 自包含、可单独测试、不读全局配置 → `pkg/`（例：`pkg/metric`、`pkg/timeutil`）。
- `pkg/jsruntime` 是显式例外：它被 `utils/messageSender/javascript` 使用，**不是插件专用**，移除插件系统时刻意保留（`build-and-pinning.md` §1.5）。

### 3.4 模型只放 `database/models/`

GORM 结构体集中在 `database/models/`（13 个文件）。RPC handler 里出现的匿名 `struct` 只用于**参数绑定与响应拼装**（如 `web/rpc/jsonrpc/common.go:218-220` 的 `params`），不落库。

### 3.5 嵌入资源用 `data/theme/` 占位目录

`web/api/admin/data/theme/`、`web/install/data/theme/`、`web/recovery/data/theme/` 等是**为了让 `go:embed` 目标目录在 git 中存在**的占位，不是业务数据。

## 4. 新增文件该放哪里

| 你要加的东西 | 放这里 | 参照 |
|---|---|---|
| 一个新的 JSON 接口 | `web/router/router.go` 里 `jsonRpc.Bind(...)` + 一个 `web/rpc/jsonrpc/<ns>.<topic>.go` 方法 | `web/router/router.go:44-51` |
| 二进制/流/重定向/特殊鉴权接口 | `web/api/{admin,client,public}/` 保留为 REST handler | `web/router/router.go:85-101` |
| 一个新的数据库读写函数 | `database/<资源包>/`，用 `dbcore.GetDBInstance()` | `database/tasks/tasks.go:3-10` |
| 一张新表 | `database/models/<name>.go` + 在 `dbcore.go` 的 `AutoMigrate` 列表登记 | `database/dbcore/dbcore.go:448-460` |
| 新的配置项 | `internal/config/settings.go` 的结构体字段（`json` + `default` tag） | 见 `database-guidelines.md` §4 |
| 可复用的算法/存储引擎 | `pkg/<name>/`，自带 `*_test.go` | `pkg/metric/`、`pkg/timeutil/` |
| 启动期一次性数据修补 | `internal/migrations/`（不删表，见 `database-guidelines.md`） | `internal/migrations/migrations.go:92-93` |
| 后台周期任务 | `internal/server/runtime.go` 的 `registerScheduledWork()` | `internal/server/runtime.go:175-198` |

## 5. 常见错误

- **不要**新建第二个 `package main`：`main.go` 是唯一入口，子命令写进 `cmd/`。
- **不要**在 `web/rpc/jsonrpc/` 里直接 `db.Create(...)`：业务读写落在 `database/` 对应包，handler 只做参数装配、权限判断与审计。
- **不要**把新 Go 文件放进 `web/public/defaultTheme/`——那是前端产物目录，会被 `scripts/build-frontend.sh` 整体替换。
- **不要**把 `docs/MAINTAINING.md` 的信息复制进本文件：构建/pin 契约以 `build-and-pinning.md` 为准，本文件只管"代码放哪"。

## 6. 自检

改完目录结构后：

```bash
go build ./... && go vet ./... && go test ./...
```

新增 RPC 方法还应确认 `rpc.help` 能列出它（元数据由 `ensureMeta` 保证，见 `pkg/rpc/registry.go:35-37`）。
