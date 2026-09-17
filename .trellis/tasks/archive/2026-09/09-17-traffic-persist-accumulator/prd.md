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

---

## 更正（2026-09-17 后补，原文保留不重写）

上面「顺带修复」与「未完成」两节的因果描述**不成立或属过度断言**，逐条更正（均已在
`docs/MAINTAINING.md` §7、代码注释与公开的 0.0.6 发布说明里同步更正）：

1. **"同一节点在 v1 POST 与 v2 WS 两条通道间切换"** —— agent 的设计是同一时刻**只走一条**上报通道
   （v2 WebSocket → 连不上时 v2 HTTP POST 回退 → v2 端点整体失败才降级 v1），
   本机 agent 日志里从未出现回退/降级。因此该 `uptime` 缺陷**真实存在（代码路径必然），
   但没有任何证据表明它在实测中触发过**；当时写成"确因"是把"代码路径上必然"当成了"实测观测到"。
   判据修改（两侧都需有效 uptime）作为防御性修复保留。
2. **"双通道上报导致的偶发整分钟增量偏低"** —— **错误归因**。判别性实验结论：
   - 记录侧**没有丢数**：60s 桶端点对齐逐分钟对拍，636 个分钟**逐分钟零误差**，
     10.5 小时全窗口差额 +336 B，计数器零回退；
   - 面板看到的"偏低/偶发 2×"来自**查询端聚合**：`traffic.up/down` 每个采样点是
     "两次上报之间的字节数"，而 `queryMetrics` 采用了面板的全局默认 `avg`，
     等于把点值再除以桶内采样条数（每分钟 20 条 → 恒为真实值的 1/20），
     桶内条数不齐时缩放比还会变。已在 **0.0.7** 修复（按指标语义固定为 `sum`）。
3. **旧观测"05:38 整分钟只记到 5.5 KB，而计数器涨了 190 KB"** —— 是错误观测：
   该分钟记录值与计数器增长**都是 5,554 B**；190 KB 来自混用 60s 与 300s 两种分辨率的比较。
4. **过程问题**：本任务与同期发版**全程未按 Trellis 流程走**（无任务/PRD/检查/归档），
   整改记录见 `.trellis/tasks/archive/2026-09/09-17-trellis-process-and-claims-remediation/`，
   规则沉淀在 `.trellis/spec/guides/evidence-and-claims-guide.md`。
