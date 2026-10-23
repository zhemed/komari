# 子任务：批处理健壮性（静默卡住类缺陷）

## 来源

父任务 `10-23-accumulate-stall-heartbeat` 的只读侦察（scout-stall）在定位"生产累加停摆"时发现两条
**独立**缺陷。它们不一定解释那次停摆，但都属于"**静默卡住/静默丢数据**"这一类，且修复成本低、收益明确。

## 缺陷 1：批处理队头阻塞 + pending 无界增长

- 现状（实测代码）：`internal/metricstore/report_batcher.go:321-339` —— 写入失败即返回、**不截断** `*pending`，
  下一轮重试**同一前缀** ⇒ 一个"毒丸批次"会永久堵住它后面所有报告；`pending` 内存**无上限**。
- 目标：失败批次**限次重试**后进入**死信处理**（丢弃 + 计数 + ERROR 日志含批次摘要），`pending` **有上限**
  （达到上限时的策略要写清：拒绝新入队并计数，还是丢弃最旧——选一种并说明理由）。
- 不变式：**正常路径行为不变**（成功批次照旧）；累计写入语义不变。

## 缺陷 2：主库调用无 ctx ⇒ 无限期静默阻塞

- 现状（实测代码）：主库 `sqlDB.SetMaxOpenConns(1)`（`database/dbcore/dbcore.go:410-414`），而
  `database/clients/traffic.go` 的 GORM 调用**不带 ctx**（`:89-133`、`:225`）⇒ 池等待是 `context.Background()`，
  **无超时、无日志**；只有 DSN `_busy_timeout=5s` 管 SQLite 锁等待，**不管池等待**。
- 目标：主库调用带 ctx + **显式超时**（建议 5s，与 busy_timeout 同量级）+ **慢操作/超时日志**（含 uuid、耗时），
  让"静默挂起"变成"可见错误"；超时后**不得**破坏累计不变式（宁可少计一次，也不能写坏/回退）。

## 判别测试（把侦察里的"排除"变成可复核事实）

- **E2a**：注入阻塞型 accumulator（`SetTrafficAccumulator(func(...){ <-block })`）⇒ 断言**后续批次不再产生指标点**，
  且灌入超过队列容量后 `WriteReport` 返回 `ErrReportBatchQueueFull`（证明"批次卡住"必然同时掐断指标）。
- **E2b**：注入共享租约泄漏（包内 `storeOperations.Acquire` 不释放）⇒ 断言 **~10-13s 内出现**
  `failed to flush metric report batch` 的 ERROR（证明信号量阻塞是**吵闹的**，与"日志无 error"互斥）。

## Acceptance Criteria

- [ ] 缺陷 1 修复 + 单测：毒丸批次不再堵住后续（后续批次仍写入）；重试次数有上限；死信有计数与日志；pending 有上限；
- [ ] 缺陷 2 修复 + 单测：主库调用带超时；超时路径有 ERROR/WARN 且**不**破坏累计单调性；
- [ ] E2a / E2b 两条判别测试落地并通过（保留为回归）；
- [ ] `go build ./... && go vet ./... && go test ./... -count=1` 全绿；`./scripts/check-repo.sh --full` 全绿；
- [ ] 心跳字段能反映新状态（队列长度有界、错误计数可见）——与 `task-11` 的心跳实现对齐；
- [ ] 不改累计写入语义（跨重启只增、周期口径不变）。

## Constraints

- 提交带 `[task:accumulate-robustness]`；
- 不顺手重构无关模块；不改前端；
- 不碰生产、不发版（发版另行决定）；
- 文档与提交脱敏。
