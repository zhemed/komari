# 流量历史跨重启保存（服务端持久累计）

## 问题（用户报告）

“重启后流量历史就不能保存了”。查证：面板卡片“总流量”直接展示 agent 上报的
**开机以来计数器**（`/proc/net/dev`），机器一重启必然归零——上游一直如此；
agent 自带的落盘统计 `net_static.json` 只在 `--month-rotate` 非 0 时才启用（我们默认没开），
服务端 metric store 的增量（`traffic.up/down`）虽然跨重启保留，但只用于后台 24 小时流量图。

## 方案（用户选定：服务端做持久累计）

1. 新增 `client_traffic_totals` 表（`database/models/traffic.go`）+ 仓储层
   `database/clients/traffic.go`（内存快照 + SQLite，启动加载）；
2. 数据源 = metric store 已有的**重置感知增量**，经
   `internal/metricstore/report_batcher.go` 的钩子回传（metricstore 不依赖 database/*）；
3. 首次见到节点用当前计数器做基线（不重复累加当次增量）；
4. 面板 `getNodesLatestStatus` 改读累计值，卡片“总流量”与流量阈值进度随之跨重启可用；
5. 删除节点时清理累计行。

**不改 agent**，因此已部署的所有节点立刻生效，不需要重装。

## 验收

- [x] 单测：基线语义 / 计数器归零后不回退 / 重启后重新加载 / 删除清理 / 钩子参数正确
- [x] 实测：累计与 metric store 增量之和相差 18KB（≈一个 3 秒上报周期）
- [x] 实测：重启 agent、重启服务端期间累计值单调不减；面板数值与库中累计一致
- [x] `./scripts/check-repo.sh --full` 十项全绿
- [x] 文档：MAINTAINING §3.6（设计与语义）、§7（残留噪声）、README、数据库规范 §0

## 顺带修复（实测抓到的真 bug）

v2 协议的报告没有 `uptime` 字段（服务端读到 0），而 agent 重启判定用
`report.Uptime < values.uptime`；同一节点在 v1 POST 与 v2 WS 两条通道间切换时会被误判成
“agent 重启”并把该条上报的流量增量清零。判据改为“两次上报都带有效 uptime”，
加回归测试 `TestWriteReportKeepsTrafficWhenUptimeMissing`。

## 未完成（记录在 MAINTAINING §7）

双通道上报导致的**偶发整分钟增量偏低**仍未完全消失（逐桶：多数 0.9~1.2，个别 0.0x；
样本 min/max 显示增量扎堆）。需要给上报流加临时日志才能定性；
后续方向是在“服务端在 v2 活跃时忽略 v1 指标上报”与“agent 只走一条通道”之间选一个。
