# 排查记录：按月重置流量

证据等级：**实测** = 本次读代码/命令确认；**推断** = 由代码推得、未运行验证；**待查** = 需生产只读数据。

## 1. 链路（实测）

agent（`--month-rotate != 0`）
→ `agent/monitoring/unit/net.go:224-246`：上报 `GetTotalTrafficBetween(GetLastResetDate(...), now)`
→ 网络层 `agent/monitoring/netstatic/static.go:418-438`：按时间窗累加每日记录 ⇒ **本周期值**
→ 服务端 `web/api/client/ingest.go` → `metricstore.WriteReport`
→ `internal/metricstore/report_batcher.go:423-436`：`TotalUp < prev` ⇒ 判 `counterReset` ⇒ delta=0（该次）
→ `internal/metricstore/report_batcher.go:459-466`：把本次上报值写回 state（后续恢复增量）
→ `internal/server/metric_store.go:91` → `clients.AccumulateTraffic`
→ `database/clients/traffic.go:49-91`：只在**首次**建行时用上报值做基线，之后**只加 delta**
→ `web/rpc/jsonrpc/common.go:363-366`：面板 `net_total_up/down` 取持久累计
→ `frontend/src/components/Node.tsx:361-378`：百分比 = (up+down)/limit

**断点**：服务端没有任何"周期"概念；agent 的"周期值"被当成"单调计数器"。

## 2. 服务端确实无归零路径（实测）

- `grep -rn 'AccumulateTraffic' --include='*.go'` 仅 3 处（定义 + 1 调用 + 注释）。
- `database/clients/traffic.go` 只有 Init/Accumulate/Get/GetAll/Delete。
- `configs` 无 reset/month 键；`clients` 模型无 month_rotate 列（`database/models/models.go:41-42` 只有 traffic_limit/type）。
- 唯一 month_rotate 落在 agent：`agent/cmd/flags/flag.go:19`；前端只有 i18n 键 `nodeTable.monthRotate`（未接后端）。

## 3. 时区与越界（实测）

- `agent/utils/utils.go:9-36`：`GetLastResetDate` 用传入 `time.Now()` 的 `Location()`（agent 本机时区）。
- `getActualResetDate`（同文件 39-58）：到期日 00:00；小月无该日则顺延次月 1 日。
- 越界（<1 或 >31）：直接 `return currentDate` ⇒ 区间 `[now,now]` ⇒ 恒 0，无告警。

## 4. 面板口径（实测）

- 设计文档 `docs/MAINTAINING.md` §3.6 明确：`client_traffic_totals` 是**跨重启累计**，面板"总流量"与限额进度读它。
- 因此"数字永不下降"是既有设计，不是 bug；与"按月重置"语义冲突的是**产品口径**。

## 5. metricstore 两级分辨率（实测 + 待查）

- rollup 桶保存 count/sum/sumSq/min/max/first/last（`pkg/metric/rollup.go` 全文；`mergeRollupValues`）。
- `SemanticAggregation`（`internal/metricstore/legacy_records.go:140-158`）显式规定：
  `traffic.up/down` = **AggSum**（注释记录了"默认 avg 会把值缩小"的实测），`net.total.*` = AggLast。
- 若两级都按 sum 求和，理论应相等 ⇒ 2.3× 差异只可能来自**覆盖不全**或**查询侧聚合/区间差异**。
  - 验证 SQL（只读，需用户在实例执行）：
    1. `SELECT resolution_milli, COUNT(*), SUM(sum) FROM <prefix>rollups WHERE metric_id=<traffic.up> AND bucket_start BETWEEN <T0> AND <T1> GROUP BY resolution_milli;`
    2. 同上补 `MIN(bucket_start), MAX(bucket_start)`，检查 60s 层覆盖是否从当天 0 点开始；
    3. `SELECT COUNT(DISTINCT bucket_start) FROM ...` 与理论桶数对比，找缺口。
  - **待查**：60s 层是否因 compaction 滞后/起点缺失导致覆盖不足。

## 6. 流量虚高（2.12×）

用户裁决：测量有误，**移出范围**。若日后要查：同日 `SUM`(traffic.up/down) vs `client_traffic_totals` 日增量的对照方法见 §5 SQL。

## 7. 两方案对比（供决策）

| 维度 | A 服务端检测周期回滚 | B 服务端算周期（推荐） |
|---|---|---|
| agent 改动 | 无（保留周期值上报） | 需把 `--month-rotate` 设 0（回到单调计数器） |
| 服务端改动 | 配置 + 检测/去抖 + 基线重设 | 配置 + 周期累加列 + 输出 |
| 语义清晰度 | 靠"猜"（下降 + 时间窗） | 明确（周期边界由服务端定义） |
| 误判风险 | 重启/换机/抖动 | 无（计数器单调） |
| 配置成本 | **同样需要节点级重置日**（否则无法区分重启） | 同上 |
| 存量兼容 | 老 agent 直接可用 | 需一次配置切换窗口 |

结论：两者都需要节点级配置；**B 语义最干净**，A 仅在"不能动 agent 配置"时选。
