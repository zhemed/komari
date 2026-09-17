# 0.0.7 流量查询聚合修复（traffic.up/down 必须求和）

> **补登说明**：本任务的工作已于 2026-09-17 完成并发布 0.0.7，但当时**没有按 Trellis 建任务**
> （见 `09-17-trellis-process-and-claims-remediation`）。本文件是事后补登的规划物，
> 内容与实际提交一致，验收项均附复核命令。

## Goal

修掉"面板流量图个别分钟偏低 / 偶发 2×"：根因在**查询端聚合语义**——
`metricstore` 的 `traffic.up/down` 每个采样点是"两次上报之间的字节数"，
而 `queryMetrics` 直接采用客户端的全局聚合偏好（面板默认 `avg`），
取平均等于把点值再除以桶内采样条数。修复后请求端不必改动，服务端按指标语义纠正。

## Requirements

- R1 `traffic.up/down` 的查询聚合固定为 `sum`；`net.total.up/down` 固定为 `last`；
  其余指标（CPU/内存/速率）继续跟随客户端偏好。
- R2 聚合规则只能有**一个来源**：与 records 路径既有的 `recordMetricAggregation` 约定合并为
  `metricstore.SemanticAggregation`。
- R3 覆盖优先级明确且可控：按指标显式指定 > 语义默认 > 全局指定 > `avg`；
  需要覆盖语义默认时用 `aggregation_by_metric`。
- R4 前端与 agent **不改**（前端已内嵌进服务器二进制，能不动就不动）。
- R5 不得再声称"数据丢失"：修复前先证明记录侧无丢失（判别性实验）。

## Acceptance Criteria

- [x] `metricstore.SemanticAggregation` 存在且被 records 路径与 queryMetrics 共用
      （`grep -rn "SemanticAggregation" internal/metricstore web/rpc/jsonrpc`）
- [x] `resolveMetricAggregation` 优先级实现为 R3
      （`web/rpc/jsonrpc/public.metric.go`；测试 `TestResolveMetricAggregationPrecedence`）
- [x] 语义默认真值表测试：`internal/metricstore/semantic_aggregation_test.go`
- [x] 记录侧无丢失的判别性实验：60s 桶端点对齐逐分钟对拍 636/636 分钟零误差、
      全窗口差额 +336 B（见 `docs/MAINTAINING.md` §7 与本任务 implement 记录）
- [x] 面板同款参数下修复版点值**逐分钟精确等于**库中真实量（备份数据 + 备用端口实例对拍）
- [x] `go build ./... && go vet && go test`（metricstore / jsonrpc）全绿
- [x] `./scripts/check-repo.sh --full` 十项全绿
- [x] 发布 0.0.7：17 个 release 资产 + ghcr 两个镜像 `:0.0.7`/`:latest`（匿名校验可拉取）
- [x] 本机部署从 release 资产升级，二进制 sha256 与资产一致，面板同款请求实测 5/5 分钟精确一致

## Notes

- 相关规范：`.trellis/spec/backend/metric-query-aggregation.md`（本任务新增）。
- 已知未做（记录在案，非本任务范围）：60s 与 300s 两档桶的点值量级天然差 5 倍；
  若要同量纲需前端按 `interval_seconds` 归一化成速率。
- 提交：`48dca2e`（修复+测试+文档）、`fc7d6e8`（版本线 0.0.7）、`2109504`（发版脚本门禁）。
