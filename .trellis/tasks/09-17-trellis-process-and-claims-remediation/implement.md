# 执行计划：流程违规与结论可信性整改

> 状态图例：已完成项标注 `[x]` 与**证据命令**；未完成项在执行阶段逐条打勾。

## Step 0 · 前置（已发生，记录事实）

- [x] 用户判定："严重错误的是你根本没有调用 trellis，现在还在找借口"
- [x] 现状核查：`python3 .trellis/scripts/task.py current --source` → `(none)`；`task.py list` → 0 个任务
      （即本轮确实无任务）
- [x] 未提交的散改文件清点：`git status --porcelain`
      → `docs/MAINTAINING.md`、`internal/metricstore/report_batcher.go`、`internal/metricstore/report_test.go`、
        `.trellis/spec/guides/index.md`、`.trellis/spec/guides/evidence-and-claims-guide.md`

## Step 1 · 规划物与上下文（Phase 1）

- [x] `prd.md` / `design.md` / 本 `implement.md`
- [ ] `task.py add-context ... implement/check`（把指南、聚合规范、MAINTAINING 挂进上下文清单）
- [ ] `task.py validate <name>` 通过
- [ ] `task.py start <name>`（激活）

## Step 2 · 更正落点（Phase 2，本任务的主体）

按落点逐条完成，命令可直接复核：

- [x] `internal/metricstore/report_batcher.go` 注释：删掉"同一节点可能同时走两条通道"与"实测表现为整分钟丢流量"，
      改为带证据边界的表述（通道切换是真实代码路径；没有证据表明曾触发）
- [x] `internal/metricstore/report_test.go` 注释：同上口径
- [x] `docs/MAINTAINING.md` §7：把"已经修掉的一条确因"改为"已修掉但从未被证实触发过的缺陷"，
      并写明这是 0.0.6 的过度断言
- [x] `docs/MAINTAINING.md` §13.3：v1↔v2 切换那句加"该缺陷存在但从未被证实触发过"
- [x] `.trellis/spec/guides/evidence-and-claims-guide.md` 新建 + `guides/index.md` 登记
- [x] `docs/MAINTAINING.md` §3.4 第 1 步：加发布说明措辞门禁
- [x] `.trellis/spec/backend/index.md`：Pre-Development Checklist 指向证据指南
- [x] 归档 PRD `09-17-traffic-persist-accumulator/prd.md`：追加带日期更正块（不重写原文）
- [x] `journal-1.md` Session 12：追加更正指针
- [x] `.build/rel-notes-0.0.6.md`：追加与线上一致的更正段（原文备份到 `rel-notes-0.0.6.published-backup.md`）
- [x] **公开的 GitHub release `0.0.6` 说明**：`gh release edit 0.0.6 -R zhemed/komari --notes-file ...`
      （已确认线上正文含 1 段「更正（2026-09-17」；原文完整保留在上方）
- [x] `git commit` 全部更正（`245bb91`，已推送）（一个提交，说明里写清"整改"性质）

## Step 3 · 验证（Phase 2 收尾）

- [x] 残留特征词复查：`grep -rn "确因\|实测表现为\|双通道上报导致" --include="*.md" --include="*.go" .`
      （排除 `node_modules`/`dist`/`bin`/`.build/tools`）→ 命中项全部属于**更正说明**或
      **指南里的禁用措辞清单**，无未更正的原始断言；归档 PRD 与 journal 的原文条目均已被更正块覆盖
- [x] `go build ./... && go vet ./internal/metricstore/... && go test ./internal/metricstore/... ./web/rpc/jsonrpc/...` → 全绿
- [x] `./scripts/check-repo.sh --full` 十项全绿（含新增门禁脚本改动）
- [x] `gh release view 0.0.6 -R zhemed/komari --json body --jq .body | grep -c "更正（2026-09-17"` = 1

## Step 4 · 检查与收尾（Phase 3）

- [x] 运行 `trellis-check`（gofmt/vet/test/check-repo --full 全绿；仅注释与文档改动，跨层项不适用）（规范符合性、跨层数据流、复用、一致性）
- [x] `trellis-finish-work`：写 journal（本次整改 Session）、`task.py finish`、`task.py archive <name>`
- [ ] 汇报：把 break-loop 五维分析贴给用户（含 E1 根因与 M1~M4 机制状态）

## 回滚点

- 更正只动文档/注释 → 回滚 = `git revert <本任务提交>`；已发布的 0.0.6 notes 需再 `gh release edit` 回旧文本
  （本地副本 `.build/rel-notes-0.0.6.md` 保留原文以便回滚）。
- 0.0.7 的代码修复不属于本任务，不在回滚范围。
