# 设计：服务端算流量周期

## 1. 边界与不变量

**不变量**

1. `client_traffic_totals` 的 **累计语义不变**（跨重启只增）——面板与限额进度继续读它。
2. 周期值只是**附加**产物：`period = Σ delta`（自 `max(周期起点, 启用时刻)` 起）。
3. 任何"下降"一律**不产生负增量**，但**只有**跨周期边界才归零周期值；计数器归零（重启/换机）不归零周期值。
4. 配置非法 ⇒ 拒绝保存（不静默降级）。

**改动边界**：只碰 `database/models`、`database/clients`、`web/rpc/jsonrpc`、`internal/metricstore`（仅当需要传边界）
与文档；**不动前端**（用户裁决：面板不改）。

## 2. 数据模型（全部为追加，AutoMigrate 安全）

`models.Client`（`database/models/models.go`）新增：

| 字段 | 类型 | 语义 |
|---|---|---|
| `TrafficResetDay` | int | `0` = 禁用；`1..31` = 每月重置日 |
| `TrafficResetTimezone` | string | IANA 名称（如 `Asia/Shanghai`）；空 = `UTC`（**显式默认，不用服务器本地时区**） |

`models.ClientTrafficTotal`（`database/models/traffic.go`）新增：

| 字段 | 类型 | 语义 |
|---|---|---|
| `PeriodUp` / `PeriodDown` | int64 | 本周期用量（字节） |
| `PeriodStartedAt` | \*time.Time | 本周期起点（UTC 时刻） |
| `NeedsRebaseline` | bool | 下次上报只更新基线、不累加（口径切换用） |

## 3. 周期边界计算

新函数（放 `database/clients/traffic.go` 或独立 `period.go`）：

```
PeriodWindow(now time.Time, resetDay int, tzName string) (start, next time.Time, err error)
```

- `tzName` 空 ⇒ `UTC`；非法 IANA ⇒ 返回错误（配置保存时就要拦）。
- `resetDay` 不在 `0..31` ⇒ 返回错误（`0` 由调用方解释为"禁用"，不进此函数）。
- 语义与 agent 既有实现对齐（`agent/utils/utils.go` 的 `getActualResetDate`）：
  当月有该日 ⇒ 该日 00:00（配置时区）；当月无该日（如 31 日遇小月）⇒ **顺延次月 1 日 00:00**。
- 返回值都转成 UTC 存储，展示/日志用配置时区。

## 4. 累加流程（`AccumulateTraffic` 扩展）

```
首次建行：baseline = 上报值；period = 0；PeriodStartedAt = max(boundary(now), now)
常规：     cumulative += delta（不变）
          if 配置启用 且 PeriodStartedAt < 当前周期边界:  period = 0；PeriodStartedAt = 该边界；写审计日志
          if NeedsRebaseline:                period 不变；NeedsRebaseline = false
          period += delta
```

- **边界判定口径（实现修正）**：不能用 `PeriodWindow(now).next` 判"是否跨边界"——
  该窗口按定义一定包含 `now`，`next` 永远晚于 `now`，会导致周期永不归零。
  实现改为比较"已记录的 `PeriodStartedAt` 是否还是 `now` 所在周期的起点"：
  `PeriodStartedAt < PeriodWindow(now).start` ⇒ 至少跨过一个完整边界 ⇒ 归零并把起点前移到该边界。
- **老节点首次启用（行已存在、`PeriodStartedAt IS NULL`）**：把起点补成 `PeriodWindow(now).start`
  （当前周期的真实边界），**不置 `resetNow`** ⇒ 不写重置审计事件；周期值仍从 0 开始累加。
  这里绝不能写零值时间（`0001-01-01`）：下次上报会把它判成"很久以前的起点"而误报一次重置。
- `PeriodWindow(now).start` 按定义不晚于 `now`（顺延出来的未来边界是 `next`），
  所以"首次建行"的 `max(boundary, now)` 恒等于 `now`，即**周期从启用时刻起算**。
- `nextBoundary` 用 `PeriodWindow(now, ...)` 现算（每次上报算一次，成本可忽略；不做长时间预计算，避免时区/闰月状态）。
- 禁用配置（`resetDay = 0`）⇒ 周期字段保持为空（内存与库都清空），行为与今天完全一致（**回归安全**）。
- **配置三态必须分开（`periodConfigState`）**：`disabled`（`resetDay = 0`／节点不存在）⇒ 不记账且清空周期列；
  `enabled` ⇒ 正常记账；`invalid`（库里是越界重置日或非法时区，正常保存路径会拦住、只可能来自外部改库）
  ⇒ **只跳过本次周期记账，保留库中现值**（内存也保留现值，面板不会读到 0），留 WARN。
  把 `invalid` 混同于 `disabled` 会在外部改坏配置时顺手清空该节点已有的周期数据——记账失败不该销毁数据。
- **内存缓存**：`InitTrafficTotals` 读入新列；所有变更同时写内存与 DB。直接改库必须**重启服务端**才生效。

## 5. agent 口径切换（关键风险）

切换 `--month-rotate` 时，agent 上报值会从"周期值"跳到"开机计数器"（通常**变大**），
若按普通增量计算会灌入一大笔假流量。

- 对策：**保存节点流量配置时无条件置 `NeedsRebaseline = true`**；下一次上报只更新基线、`delta = 0`。
- 运维步骤写进文档：① 在服务端配置重置日/时区 → ② 把 agent 的 `--month-rotate` 设为 `0`（此时配置保存已置再基线）
  → ③ 观察一次上报后累计值继续正常增长。
- 不做"按阈值猜测跳变"（脆弱）；宁可要求一次显式配置保存。

## 6. 接口

- **写入**：节点编辑 RPC 接受 `traffic_reset_day`、`traffic_reset_timezone`；校验失败返回明确错误；
  请求里**带了这两个字段就无条件**置 `NeedsRebaseline` + 审计日志（`auditlog`）——与 §5 一致，
  即使新旧值相同也置位（切换 agent 口径只需点一次保存，不依赖"值是否变化"；
  代价是每次保存都会丢掉一次上报的增量，最多损失一个上报周期）。
- **读取**：`getNodesLatestStatus` **追加** `net_period_up` / `net_period_down`（可选字段）；
  `net_total_up/down` 语义与取值**不变**（面板因此不需要改动）。
- 兼容：老客户端忽略新字段；新字段缺失时前端行为不变。

## 7. 测试计划

| 用例 | 断言 |
|---|---|
| 边界计算：UTC / Asia/Shanghai / 小月 31 日 / 跨年 | 返回的 UTC 起点与预期一致 |
| 非法配置：`-1`、`32`、`Not/AZone` | 返回错误，且**不落库** |
| 跨边界累加 | 到点后 `period` 归零、`cumulative` 不回退 |
| 计数器归零（模拟 agent 重启） | `period` **不**归零、`cumulative` 不回退 |
| 再基线 | 切换后首次上报 `delta = 0`，`cumulative` 不跳变 |
| 禁用配置回归 | 与改动前行为一致（周期字段不动） |

## 8. 迁移与回滚

- **迁移**：GORM AutoMigrate 追加列，无需手工 SQL；旧行新列为零值（`resetDay = 0` ⇒ 行为不变）。
- **回滚**：换回旧二进制即可（新列被忽略）；**不需要**删列（删列才是破坏性操作）。
- 若曾直接改库：重启服务端，让 `InitTrafficTotals` 重新加载。

## 9. 未决/后续

- 面板展示周期值（用户本轮明确不改，留待后续任务）；
- agent 侧 `--month-rotate` 是否最终移除（当前保留，但文档标注"与服务端周期二选一，勿同时用"）；
- 历史累计值无法反推各周期用量（已在排查报告说明）。
