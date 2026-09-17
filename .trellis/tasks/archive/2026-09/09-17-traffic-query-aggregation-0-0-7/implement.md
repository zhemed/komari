# 执行计划：流量查询聚合修复

## Step 1 · 判别性实验（先证明记录侧无丢失）

- [x] 60s 桶端点对齐逐分钟对拍：636 个分钟**差值全为 0**，全窗口 +336 B；计数器零回退
- [x] 更正旧观测："05:38 只记到 5.5 KB / 计数器涨 190 KB" 实为混用 60s/300s 分辨率的假象
      （实测两边都是 5,554 B）

## Step 2 · 实现

- [x] `internal/metricstore/legacy_records.go`：新增导出的 `SemanticAggregation`，
      `recordMetricAggregation` 改为复用它（唯一来源）
- [x] `web/rpc/jsonrpc/public.metric.go`：`resolveMetricAggregation` 改为新优先级
- [x] `internal/metricstore/semantic_aggregation_test.go`：真值表
- [x] `web/rpc/jsonrpc/public_metric_test.go`：`TestResolveMetricAggregationPrecedence`

## Step 3 · 验证

- [x] `go build ./...`；`go vet`；`go test ./internal/metricstore/... ./web/rpc/jsonrpc/...`
- [x] 备用端口对拍（复制 data/，不动线上）：线上点值=真实值÷20，修复版逐分钟精确相等；
      `hours=24` 的 300s 桶=该 5 分钟各分钟之和
- [x] 显式覆盖仍生效：`aggregation_by_metric={"traffic.up":"max"}` → 返回 max 语义
- [x] `./scripts/check-repo.sh --full` 全绿

## Step 4 · 发布 0.0.7

- [x] 版本线四处字面量 + 文档标题 → 提交 `fc7d6e8`（**先提交后构建**）
- [x] 构建：服务端 amd64/arm64 静态 + agent 14 平台 + `komari-agent-SHA256SUMS`
- [x] `git tag 0.0.7 && git push`；`gh release create 0.0.7 -R zhemed/komari`（17 资产）
- [x] 镜像：`ghcr.io/zhemed/komari:0.0.7`/`:latest`、`ghcr.io/zhemed/komari-agent:0.0.7`/`:latest`，
      匿名 `docker manifest inspect` 架构数 2/2/3/3
- [x] 本机部署从 release 资产升级；面板同款请求实测 5/5 分钟点值精确一致

## Step 5 · 复盘（本次由用户指出的流程问题）

- [x] 本任务**本应在开工前创建**：实际是事后补登 —— 记录在
      `09-17-trellis-process-and-claims-remediation`，不掩饰。

## Step 6 · 补登验收记录（2026-09-17 复核）

| 验收项 | 命令 | 结果 |
|---|---|---|
| 语义默认唯一来源 | `grep -rn SemanticAggregation internal/metricstore web/rpc/jsonrpc` | 定义 1 处、两处使用（records + queryMetrics） |
| 优先级测试 | `go test ./internal/metricstore/... ./web/rpc/jsonrpc/... -run "Semantic\|ResolveMetricAggregation" -v` | 全 PASS |
| 发布物 | `gh release view 0.0.7 -R zhemed/komari` | 17 资产、非 draft |
| 镜像 | `docker manifest inspect`（匿名） | server 2 架构 / agent 3 架构 |
| 本机部署 | `sha256sum /opt/komari/komari` vs `dist/komari-linux-amd64` | 一致；版本行 0.0.7 (hash fc7d6e8) |
| 端到端 | 面板同款 `public:queryMetrics`（`aggregation=avg`）对拍库中 `sum` | **60/60 个分钟精确一致** |
| 仓库自检 | `./scripts/check-repo.sh` | 全部通过 |

> 补登事实：这些验收在发布当时（本任务被创建之前）已经跑过一遍，本次为留痕重跑，结果一致。
