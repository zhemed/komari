# 实施计划（有序清单 + 验证 + 回滚点）

## 阶段 0：准备（只读）
- [ ] 读 `.trellis/spec/backend/database-guidelines.md`、`server-upgrade.md`、`quality-guidelines.md`（提交前必跑命令）
- [ ] 复读归档排查报告 `archive/2026-10/10-23-traffic-month-rotate/research/findings.md`
- **回滚点**：无改动

## 阶段 1：模型与边界计算（可独立验证）
- [ ] `database/models/models.go`：`Client` 增 `TrafficResetDay`、`TrafficResetTimezone`
- [ ] `database/models/traffic.go`：`ClientTrafficTotal` 增 `PeriodUp/PeriodDown/PeriodStartedAt/NeedsRebaseline`
- [ ] 新增 `PeriodWindow(now, resetDay, tz)` + 单测（UTC / Asia/Shanghai / 小月 / 跨年 / 非法值）
- 验证：`go build ./... && go test ./database/...`
- **回滚点**：`git revert` 本阶段提交

## 阶段 2：累加逻辑
- [ ] `database/clients/traffic.go`：扩展 `AccumulateTraffic`（周期累加、跨边界归零、再基线、禁用时零行为差异）
- [ ] `InitTrafficTotals` 加载新列
- [ ] 单测：跨边界、计数器归零、再基线、禁用回归
- 验证：`go test ./database/clients/... -run Traffic -v`
- **回滚点**：本阶段提交

## 阶段 3：配置接口与校验
- [ ] 节点编辑 RPC：接受并校验 `traffic_reset_day` / `traffic_reset_timezone`；非法 ⇒ 报错不落库
- [ ] 值变化 ⇒ `NeedsRebaseline = true` + `auditlog` 记录
- [ ] `getNodesLatestStatus` 追加 `net_period_up/down`（不改变既有字段）
- 验证：`go test ./web/... ./database/...`；手工 curl 校验非法值被拒（**本地实例**）
- **回滚点**：本阶段提交

## 阶段 4：文档
- [ ] `docs/MAINTAINING.md` §3.6：补"周期口径 + agent 切换步骤 + 重启才生效"
- [ ] `.trellis/spec/backend/server-upgrade.md` 或 database spec：补不变量与测试要求
- 验证：`./scripts/check-repo.sh`

## 阶段 5：全量验证与收尾
- [ ] `go build ./... && go vet ./... && go test ./...`
- [ ] `./scripts/check-repo.sh --full`
- [ ] 端到端手工验证（**本地实例**，用测试库）：
      配置 `resetDay=22`、`TZ=Asia/Shanghai` ⇒ 观察周期值在边界归零、累计不回退
- [ ] 提交（消息带 `[task:traffic-period-server-side]`）→ finish → archive → journal

## 发布与回滚（若发版）
- 版本号按发版时的最新号 +1；资产 18 个；镜像 `:latest` 同步；
- 回滚：换回上一个 release 资产或 `komari.backup.<时间戳>`，重启服务端；
- **不删列**（新列被旧二进制忽略）。

## 风险清单
1. 时区数据库（tzdata）缺失 ⇒ `time.LoadLocation` 失败：镜像/静态二进制需确认 tzdata 可用（阶段 1 验证）。
2. 边界计算每次上报都做 ⇒ 影响可忽略，但要在基准测试里确认无锁竞争（阶段 2）。
3. 直接改库不会立刻生效（内存缓存）⇒ 文档与审计日志都要写明。
