# 指标查询与聚合约定

> 规范描述**代码现状**，不是理想状态。锚点会随代码漂移，改到相关代码时顺手更新。

---

## 0. 一句话契约

**"每次上报一个字节数"的指标（`traffic.up/down`）在查询时必须求和，不能取平均；
累积计数器（`net.total.up/down`）取桶内最后一个值。**
这条约定写在服务端，不依赖任何客户端（含面板）传什么聚合参数。

---

## 1. 指标的两类语义

指标名常量：`internal/metricstore/metrics.go:5-31`。

| 指标 | 类型 | 每个采样点的含义 | 正确聚合 |
|---|---|---|---|
| `traffic.up` / `traffic.down` | gauge / bytes | 两次上报之间新增的字节数（上报间隔约 3s） | **sum** |
| `net.total.up` / `net.total.down` | counter / bytes | 开机以来的累积字节数 | **last** |
| 其余（`cpu.usage`、`memory.used`、`net.in.rate`…） | gauge | 瞬时值 / 速率 | 客户端偏好（默认 `avg`） |

`traffic.*` 为什么不能用 avg：它的采样点是"量"而不是"水平"。取平均会得到
"每次上报的平均字节数"，等于把该桶的总量再除以桶内采样条数（实测 20 条/分钟），
于是**同一个桶的点值与采样条数挂钩**——条数不齐的桶（未封桶、agent 回退窗口）
缩放比就变了，同一张图上相邻点被除以不同的数，表现为"个别分钟偏低/偶发 2×"。
（完整实测与定位过程见 `docs/MAINTAINING.md` §7。）

---

## 2. 唯一来源：`metricstore.SemanticAggregation`

`internal/metricstore/legacy_records.go:150`：

```go
func SemanticAggregation(metricName string) (metric.Aggregation, bool)
```

- 返回该指标的语义默认聚合；第二个返回值为 `false` 表示"没有语义默认，交给调用方偏好"。
- `recordMetricAggregation`（同文件 `:161`）是 records 路径的包装：有语义默认用它，否则 `avg`。
- **要加新指标时先想清楚它属于"量"还是"水平"**：量的指标请在这里登记，不要在各个查询入口各写一份。

---

## 3. 查询端优先级（`queryMetrics`）

`web/rpc/jsonrpc/public.metric.go:743` `resolveMetricAggregation`：

```
按指标显式指定（aggregation_by_metric / algorithm_by_metric）
  > 指标语义默认（metricstore.SemanticAggregation）
  > 全局指定（aggregation / algorithm）
  > avg
```

语义默认排在全局之前是**有意**的：全局那个是图表偏好（面板默认 `avg`，见
`frontend/src/pages/instance/LoadChart.tsx` 的 `komari-instance-metric-aggregation`），
不是针对某个指标的意图；让它压过语义默认就会把 `traffic.*` 算错。
确实要覆盖语义默认时，用 `aggregation_by_metric: {"traffic.up": "max"}` 这种**按指标**形式。

回归测试：
- `internal/metricstore/semantic_aggregation_test.go`（语义默认真值表）
- `web/rpc/jsonrpc/public_metric_test.go` 的 `TestResolveMetricAggregationPrecedence`（优先级）

---

## 4. 分辨率与桶值的语义

- 分辨率只有两档：60s 与 300s（`metric_resolutions` 表；`internal/metricstore/compaction.go` 负责 60s → 300s 折叠）。
- **`sum` 聚合下点值 = 该桶内的字节总量**，所以同一个指标在 1 小时视图（60s 桶）与
  24 小时视图（300s 桶）上的量级天然差 5 倍。这是"每桶字节数"的固有含义，不是 bug；
  想在两种视图间保持同一量纲，得让客户端按响应的 `interval_seconds` 归一化成速率（当前前端**不**归一化）。
- 面板请求参数：`max_points=700`、`aggregation="avg"`、`fill_empty=true`（`LoadChart.tsx`）。

---

## 5. 排查用的实测命令

同一分辨率下"记录值 vs 原始计数器增长"逐桶对拍（端点对齐：用相邻桶的 `last_val` 差，
不要用同桶 `first_val`，更**不要**混着 60s 与 300s 比——历史上就是这么比出过假结论）：

```bash
python3 - <<'EOF'
import sqlite3
m = sqlite3.connect('file:/opt/komari/data/metrics.db?mode=ro', uri=True)
b = list(m.execute("""select mm.bucket_milli, mm.last_val, mm.count from metric_rollups mm
                      join metric_series s on s.id=mm.series_id
                      where s.metric_name='net.total.up' and mm.resolution_id=1 order by mm.bucket_milli"""))
rec = dict(m.execute("""select mm.bucket_milli, mm.sum from metric_rollups mm
                        join metric_series s on s.id=mm.series_id
                        where s.metric_name='traffic.up' and mm.resolution_id=1"""))
bad = [(b[i][0], b[i][1]-b[i-1][1], rec.get(b[i][0], 0)) for i in range(1, len(b))
       if (rec.get(b[i][0], 0) or 0) != (b[i][1]-b[i-1][1])]
print(f"分钟数={len(b)-1} 不一致={len(bad)}")
EOF
```

预期输出 `不一致=0`（2026-09-17 实测：636 个分钟全为 0，10.5 小时全窗口只差 336 B）。

按面板同款参数复查聚合是否正确：

```bash
curl -s -X POST http://127.0.0.1:25774/api/rpc2 -H 'Content-Type: application/json' -d '{
  "jsonrpc":"2.0","method":"public:queryMetrics","params":{
    "metric_keys":["traffic.up"],"entity_id":"<uuid>","hours":1,"max_points":700,
    "aggregation":"avg","fill_empty":true},"id":1}'
```

`avg` 请求下返回的点值应当**等于**库里对应分钟的 `sum`（服务端按语义纠正）；
若点值 ≈ 真实值 ÷ 采样条数，说明语义默认没生效（检查优先级是否被改回全局优先）。
