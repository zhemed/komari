# 排查执行计划（本轮只读）

## 分三条并行工作流（委派给团队成员，各自只读）

1. **服务端链路**（scout-server）
   - `database/clients/traffic.go`：`AccumulateTraffic` 全逻辑、缓存、`InitTrafficTotals`
   - 调用点：agent 上报入口、metricstore 路径
   - 模型/表：`client_traffic_totals`、`clients`（是否已有节点级配置列）
   - 配置：`configs` 键集合；是否有 month/reset/traffic 项
   - 面板百分比来源：后端接口 → 前端组件
   - 是否存在任何归零/重置路径（grep 全仓）

2. **agent 链路**（scout-agent）
   - `monitoring/unit/net.go`（`NetworkSpeed()`）、`monitoring/netstatic/static.go`
   - `utils/utils.go`：`GetLastResetDate`（时区、越界值、边界时刻）
   - `--month-rotate` 的解析/校验/默认值/文档
   - 上报值语义（本周期值 vs 开机计数器）、重置瞬间与进程重启时的行为

3. **metricstore 一致性**（scout-metric）
   - 两级分辨率（60s / 300s）的写入与 rollup 聚合语义
   - 同日求和差异 2.3 倍的代码级原因
   - `trafficAccumulation` / `reportTrafficState` / `reportTrafficValues` 与 `client_traffic_totals` 的关系
   - `migration_legacy_monitoring_to_metric_store_done` 的消费方（旧路径是否仍在写）

## 约束

- 全部**只读**：不改任何文件；不读工作区外路径（生产数据目录、`~/.dsh/**` 等）；不发网络请求。
- 每条结论标注 **实测 / 推断 / 待查** + `文件:行号`。
- 交付五节：结论 → 改动清单（本轮=建议改动点） → 验证（命令与预期） → 风险与未完成 → 建议下一步。

## Lead 复核

- 抽查每条关键结论的 `文件:行号`（至少 3 条自己复跑）。
- 汇总为最终报告；P0 两方案取舍与建议由 Lead 定稿。
