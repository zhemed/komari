# 流程违规与结论可信性整改（未调用 Trellis + 未验证结论进入公开文档）

## Goal

整改由用户当场指出的两类严重错误：

1. **流程违规**：2026-09-17 这一轮（流量噪声排查 → 修复 → 0.0.6/0.0.7 发版）**全程未调用 Trellis**：
   开工/续接没有 `trellis-start`/`trellis-continue`，没有 `task.py create`，没有 PRD / design / implement，
   没有 `trellis-check`，没有 `trellis-finish-work`；只在事后补了 `journal` 与 spec 文件。
   并且在被指出后，还把这件事解释成"小改动走直接改路线"（= 找借口）。
2. **结论可信性**：把**未经验证的因果结论**写成"已定位的原因"，并进入**公开**发布说明与维护文档。

交付：全落点带日期更正 + 证据标准指南 + 发版措辞门禁 + 可机械复核的验收命令。

## Requirements

- R1 记录事实，不粉饰：任务内必须写明"未调用 Trellis"的具体缺口清单与时间点，不得用"直接改路线"之类的措辞淡化。
- R2 错误结论**全落点**更正，且每条都是**带日期的更正说明**，不重写历史：
  - `docs/MAINTAINING.md` §7 / §3.4 / §13
  - `internal/metricstore/report_batcher.go`、`internal/metricstore/report_test.go` 的注释
  - `.trellis/tasks/archive/2026-09/09-17-traffic-persist-accumulator/prd.md`（已归档任务）
  - `.trellis/workspace/komari/journal-1.md`（Session 12 正文 + 索引标题行）
  - **已发布的 GitHub release `0.0.6` 说明**（公开可见，用 `gh release edit` 追加更正段）
  - `.build/rel-notes-0.0.6.md`（与线上一致的本地副本）
- R3 预防机制落到仓库里，而不是口头承诺：
  - 新增 `.trellis/spec/guides/evidence-and-claims-guide.md`（观测 vs 推断、判别性实验、全落点更正清单）
  - 在 `.trellis/spec/guides/index.md`、`.trellis/spec/backend/index.md` 登记
  - 在 `docs/MAINTAINING.md` §3.4 第 1 步加"发布说明措辞门禁"
- R4 机械可复核：给出**一条命令**能列出仍带错误特征词的落点，且执行后为空（白名单除外）。
- R5 本任务自身按 Trellis 流程走完：prd/design/implement → context → validate → start → 执行 →
  `trellis-check` → `trellis-finish-work`（归档）。

## Acceptance Criteria

- [ ] `grep -rn "确因\|实测表现为\|双通道上报导致" --include="*.md" --include="*.go" .`（排除 node_modules/dist/bin）
      在更正后**只剩**带日期更正说明的条目或白名单条目
- [ ] `gh release view 0.0.6 -R zhemed/komari --json body` 含更正段（日期 + 指向 §7）
- [ ] 归档 PRD 与 journal Session 12 均有指向更正的说明；journal 新增一条整改 Session
- [ ] `./scripts/check-repo.sh --full` 全绿
- [ ] `internal/metricstore` 与 `web/rpc/jsonrpc` 的 `go test` 通过（注释改动不得打破构建）
- [ ] 本任务 `task.py validate` 通过，最终 `archive` 到 `.trellis/tasks/archive/2026-09/`

## Notes

- 用户原话（判定）："严重错误的是你根本没有调用 trellis，现在还在找借口"。
  本 PRD 把这句话作为事实记录，不辩解。
- 更正原则来自 `.trellis/spec/guides/evidence-and-claims-guide.md`：**不重写历史**，只加日期与指针。
- 0.0.7 的代码修复本身（聚合语义）另见任务 `09-17-traffic-query-aggregation-0-0-7`；本任务不重复其内容。
