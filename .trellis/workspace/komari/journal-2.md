# Journal - komari (Part 2)

> Continuation from `journal-1.md` (archived at ~2000 lines)
> Started: 2026-09-20

---



## Session 61: 升级弹窗按钮去重并发布 0.0.21（含生产升级）
<!-- trellis-session: v=2 fp=0b3eb5013c940731 -->

**Date**: 2026-09-20
**Task**: 升级弹窗按钮去重并发布 0.0.21（含生产升级）
**Branch**: `main`

### Summary

用户发现弹窗列表首项的「安装此版本」与底部按钮指向同版。核实确认（都来自 latestRelease），按用户选择删除列表里的按钮、只留底部，发 0.0.21 并升级生产。顺带修掉两处既有问题：MAINTAINING 版本表仍写 0.0.18（两次升级漏改）、自检第 1 项匹配写法形同虚设（并踩到 backtick 被 shell 命令替换的坑）。

### Main Changes

- AdminPanelBar.tsx 删除列表每行按钮；保留底部按钮两种形态与全部状态变量用法
- 版本 → 0.0.21；前端哈希 f4785c5b…；release 18 资产；镜像 :0.0.21/:latest
- MAINTAINING §1 显式声明当前版本；check-repo 第 1 项对准该行（backtick 已转义）
- 生产升级到 0.0.21（备份 komari.backup.20260920_031334）

### Git Commits

| Hash | Message |
|------|---------|
| `3977f5a` | fix(docs+check): MAINTAINING 显式声明当前版本，自检第 1 项对准它 [task:dedupe-upgrade-dialog-buttons] |

### Testing

- [OK] 生产 version=0.0.21 且二进制 sha256 与 release 资产一致；产物无 install_version 调用点；面板 200；agent online；日志 0 错误
- [OK] check-repo.sh --full 全绿；release digest 与本地产物逐一一致

### Status

[OK] **Completed**

### Next Steps

- 可选：清掉语言包里已无用的 upgrade.install_version 键；清理历史备份与恢复分支


## Session 62: 修 README 过期版本号（0.0.18→0.0.21）+ 自检纳入文档版本声明
<!-- trellis-session: v=2 fp=e7e12a356b144cf2 -->

**Date**: 2026-09-26
**Task**: 修 README 过期版本号（0.0.18→0.0.21）+ 自检纳入文档版本声明
**Branch**: `main`

### Summary

用户问 README 里的「当前 0.0.18」是否最新——核查真源全是 0.0.21，README 是陈旧文案（三次升版漏同步）。根因是自检第 1 项只查 MAINTAINING、从不查 README。已修 README，并把两处文档版本声明都纳入自检，带判别性验证。

### Main Changes

- README：「（当前 0.0.18）」→「（当前 0.0.21）」
- check-repo 第 1 项：MAINTAINING + README 双查，任一与 version.env 不一致即报红

### Git Commits

| Hash | Message |
|------|---------|
| `dd653ce` | chore(task): archive 09-26-fix-readme-stale-version |

### Testing

- [OK] 判别性验证：README 改回 0.0.18 → 报红并指出不一致；还原 → 通过
- [OK] check-repo.sh --full 全绿

### Status

[OK] **Completed**

### Next Steps

- 可选：语言包里 upgrade.install_version 已无用键；清理历史备份与恢复分支


## Session 63: 扫描残留 Trellis 任务 + 验证闸门恢复
<!-- trellis-session: v=2 fp=a31ada065a872bb2 -->

**Date**: 2026-10-23
**Task**: 扫描残留 Trellis 任务 + 验证闸门恢复
**Branch**: `main`

### Summary

用户要求扫描项目残留任务并顺便验证修好的 Trellis 闸门。闸门放行侧确认恢复（普通命令/grep/team_task_create/spawn_teammate/task.py start 全部通过，上一轮会被拦）；拦截侧未验证。只读扫描结论：活动区仅 2 个 in_progress 且均未提交、归档区 65 条零坏账、悬空指针 0、孤立目录 0。已按用户裁决提交两个任务目录留痕，归档 traffic 排查任务，本轮 audit 任务收尾归档。

### Main Changes

- 提交 traffic 任务目录（含结论与裁决：方案 B / 面板不改）并归档
- 提交 audit 任务目录，勾选验收并记录扫描结果与闸门验证表

### Git Commits

| Hash | Message |
|------|---------|
| `c86de3c` | chore(task): 提交残留扫描任务并归档 traffic 排查任务 [task:leftover-task-audit] |

### Testing

- [OK] 闸门：普通命令 / grep / team_task_create / spawn_teammate / task.py start 全部通过（实测）
- [OK] 扫描：归档 65/65 一致、缺件 0、重复 0、悬空 0（Lead 独立复核归档数、重复 name、未跟踪项）

### Status

[OK] **Completed**

### Next Steps

- 实现方案 B：新建任务，写 design.md + implement.md（节点级重置日 + 时区；agent 回单调计数器；面板不动）


## Session 64: 实现方案 B：服务端算流量周期（阶段 1-3）
<!-- trellis-session: v=2 fp=5b9700eb5b344517 -->

**Date**: 2026-10-23
**Task**: 实现方案 B：服务端算流量周期（阶段 1-3）
**Branch**: `main`

### Summary

按用户裁决实现方案 B：节点级重置日+时区、服务端周期边界与周期累计，累计语义与面板口径不变。两条写入线并行（代码 = worker-core，文档 = worker-docs），Lead 复核后两次打回：① 既有行首次启用会把 period_started_at 写成零值⇒下轮产生虚假重置事件（已修 + 红绿测试）；② 库里非法配置会清空周期值（改三态：invalid 保留现值 + WARN）。附带 tzdata 兜底 import。

### Main Changes

- models/period.go/traffic.go/period_config.go/client.go/admin.client.go/common.go/main.go + 文档两处
- 输出追加 net_period_up/down（net_total_* 未动，面板不改）

### Git Commits

| Hash | Message |
|------|---------|
| `323f907` | feat(traffic): 服务端计算流量周期（节点级重置日 + 时区）[task:traffic-period-server-side] |

### Testing

- [OK] database/clients 20 用例 + jsonrpc 契约用例全绿；两条新用例先在旧行为上 FAIL 再 PASS
- [OK] go build/vet/test 全绿；check-repo.sh --full 15 项全绿；Lead 复跑关键 3 条测试

### Status

[OK] **Completed**

### Next Steps

- 阶段 5：无 tzdata 容器验证 + 跨真实重置日观察（需授权起本地实例）；决定是否发版


## Session 65: 方案 B 上线 0.0.22：发布、镜像缺陷修复、生产切换、WARN 去重
<!-- trellis-session: v=2 fp=5ddb249ec3c2ec36 -->

**Date**: 2026-10-23
**Task**: 方案 B 上线 0.0.22：发布、镜像缺陷修复、生产切换、WARN 去重
**Branch**: `main`

### Summary

整条弧线完成：① 方案 B（服务端算流量周期）实现 + 两轮打回 + 阶段 5 验证（tzdata 判别性 + 跨边界 L1 e2e + 再基线 + 非法配置降级）→ 发 0.0.22 并升级生产；② 发版中发现并修复严重既存缺陷：多架构镜像 arm64/armv7 腿装的是 amd64 二进制（ARG 默认值覆盖 buildx 注入，0.0.21 及更早都中招），修好后逐平台载荷 5/5 与 release 资产一致；③ 生产切换：重置日 22/UTC + agent 显式钉 --month-rotate 0；期间踩到 WAL 孤儿化（外部 sqlite 写库引发），按文档 §7 优雅重启解决，累计无回退；④ WARN 去重（节点+指纹，有界并发安全）。

### Main Changes

- 0.0.22 发布：tag=39c5860、18 资产、镜像 :0.0.22/:latest 已重推为正确载荷
- Dockerfile/Dockerfile.agent 去 ARG 默认值（95085a3）；文档补 §12 载荷验收与 §3.4.1 两坑（137ae29）
- WARN 去重（5521d62，database/clients 4 文件）
- 生产：clients 两列 22/UTC、agent unit 钉 --month-rotate 0、服务端优雅重启一次

### Git Commits

| Hash | Message |
|------|---------|
| `7e4be68` | chore(task): archive 10-23-traffic-period-verify-release |

### Testing

- [OK] 生产验收：period_started_at=2026-10-22T00:00:00Z、period 随上报增长、累计单调不回退（64.98GB→65.06GB）、0 错误、agent online
- [OK] 镜像逐平台载荷 5/5 == release 资产（负例对照旧 index 全红）

### Status

[OK] **Completed**

### Next Steps

- 跨 2026-11-22 00:00Z 边界观察一次周期归零（唯一未做验证）；决定是否发 0.0.23 带上 WARN 去重、是否升级 0.0.7 的 agent


## Session 66: 0.0.23 发布（WARN 去重）+ 收尾决策
<!-- trellis-session: v=2 fp=703c12fe87a05e0c -->

**Date**: 2026-10-23
**Task**: 0.0.23 发布（WARN 去重）+ 收尾决策
**Branch**: `main`

### Summary

按用户裁决完成收尾：推送 6 个提交、发布 0.0.23（WARN 去重）、并把生产升到 0.0.23；用户决定不升级 agent（仍 0.0.7）、不设跨边界提醒。0.0.23 验收：18 资产、逐平台载荷 5/5（含 ELF e_machine 判据）、生产 0.0.23 + digest 一致、0 错误、agent online、累计不回退且周期继续增长。

### Main Changes

- 版本字面量 0.0.22→0.0.23（含 README/MAINTAINING 四处口径）；rel-notes-0.0.23.md
- 镜像 :0.0.23/:latest 重推，server index 4817dbd2…、agent index d261e841…
- 生产升级（回滚点 komari.backup.20261023_074704 + /tmp/komari.rollback.0.0.22 + upgrade-20261023-074710.zip）

### Git Commits

| Hash | Message |
|------|---------|
| `267acbe` | chore(task): archive 10-23-post-0-0-22-decisions |

### Testing

- [OK] 逐平台载荷 5/5 == release 资产（amd64 3e00 / arm64 b700 / armv7 2800）
- [OK] 生产验收：0.0.23 + da0139c、digest 3f9fc6b3… 一致、0 错误、周期 22s 内 +25,432、累计 +362,988,906 不回退

### Status

[OK] **Completed**

### Next Steps

- 唯一未做：跨 2026-11-22 00:00Z 边界观察；可选：把 agent 校验和资产 name 规则写进 §3.4


## Session 67: 面板/限额口径切到周期值 + 流水线心跳与健壮性 → 0.0.24
<!-- trellis-session: v=2 fp=db1a9781bb3bbde9 -->

**Date**: 2026-10-23
**Task**: 面板/限额口径切到周期值 + 流水线心跳与健壮性 → 0.0.24
**Branch**: `main`

### Summary

把用户可见的用量口径从累计切到服务端周期值（卡片/详情/进度条/表格），累计保留为「累计（历史）」；限额判定同口径；未配置周期节点回落累计并逐位零回归（可复跑 npm run test:period 69/69）。另交付：报告流水线心跳+看门狗（traffic_pipeline，10 字段）、批处理健壮性（限次重试+死信+pending 上限、主库 5s 超时与慢操作日志、读库失败不再误清周期列）、E2a/E2b 判别测试。发布 0.0.24（18 资产、逐平台载荷 5/5）并升级生产与本地测试实例 komari-test。

### Main Changes

- 前端 13 文件 + 产物重建（f672d9f）；口径回归脚本 60ca273；文档 6e1fde7/bcc76af/3c656db
- 心跳+看门狗 f440b70；健壮性 a34ce8e；发布 e29ba43（tag 0.0.24）
- 本地长期容器测试实例（komari-test + agent，127.0.0.1 上，数据 /root/komari-test/data）

### Git Commits

| Hash | Message |
|------|---------|
| `2330f80` | chore(task): archive 10-23-period-value-panel-cutover |

### Testing

- [OK] npm run test:period 69/69（Lead 复跑）；go build/vet/test 全绿；check-repo --full 全绿
- [OK] 生产 0.0.24：digest 一致、0 错误、agent online、累计不回退且周期继续增长

### Status

[OK] **Completed**

### Next Steps

- 待验证：跨真实周期边界（2026-11-22 00:00Z）观察周期归零；可选：把渲染层 DOM 断言搬进仓库、把 test:period 接入 check-repo


## Session 68: 面板口径加固：渲染层断言入库 + 接入自检第 16 项
<!-- trellis-session: v=2 fp=fc50e88d88de3cf4 -->

**Date**: 2026-10-23
**Task**: 面板口径加固：渲染层断言入库 + 接入自检第 16 项
**Branch**: `main`

### Summary

用户批准两项：① 把渲染层 SSR/DOM 断言从 /tmp 搬进仓库（frontend/script/period-panel-dom.mjs，渲染真组件 Node.tsx，89 项断言，零新依赖——用 vite 既有 esbuild 打包到临时目录）；② 把口径回归接入 scripts/check-repo.sh 新增第 16 项（逻辑 69 + 渲染 89，解析不出计数即报红；node 缺失/过旧/缺 esbuild 时打印跳过行、不判失败不静默通过）。红→绿判别性验证两次（逻辑与渲染各改坏一条断言 ⇒ 自检报红，还原后 byte-identical 且全绿）；跳过语义三种情况均实测。

### Main Changes

- frontend/script/period-panel-dom.mjs（+397）+ period-panel-dom-entry.tsx（+68）+ package.json script
- scripts/check-repo.sh 第 16 项（+53）+ docs/MAINTAINING.md 一行

### Git Commits

| Hash | Message |
|------|---------|
| `edc174e` | chore(task): archive 10-23-period-panel-hardening |

### Testing

- [OK] npm run test:period-dom 89/89；npm run test:period 69/69；check-repo.sh 与 --full、selftest 全绿（Lead 复跑）

### Status

[OK] **Completed**

### Next Steps

- 可选：浏览器交互层（悬停气泡/移动端/暗色）用常驻 bw 做一次真实页面快照断言；esbuild 若不再被 vite 依赖需提升为 devDependency 并同步 lock


## Session 69: ink 主题本地小样：首页节点卡片按 ping 任务分行（延迟+丢包+历史条，未部署）
<!-- trellis-session: v=2 fp=ffe57a564bfc7dee -->

**Date**: 2026-10-25
**Task**: ink 主题本地小样：首页节点卡片按 ping 任务分行（延迟+丢包+历史条，未部署）
**Branch**: `main`

### Summary

主仓任务 10-25-komari-theme-region-format 的本地小样：用户要求把「三网」行与延迟/丢包两聚合卡改成按 ping 任务分行（任务名+延迟 ms+丢包%+该任务 20 格延迟历史点阵），先看小样不部署。实现：useNodePingStats 增加每任务 loss/lossApproximate/history（metric+legacy 双路径，缓存 v10）、useNodePingDisplay 新增 NodePingRegionRow 行数据、NodeCard 用行列表替换旧版式；保留点击弹窗/离线态/色觉友好斜杠纹。type-check、eslint、build-only 通过；用 Playwright(chromium-1243 executablePath，环境无 1248 headless-shell) 渲染 mock 数据截图核对 comfortable/compact/cvf 三档。未部署、未改服务器。

### Main Changes

- themes/ink/src 3 文件：NodeCard.vue、useNodePingDisplay.ts、useNodePingStats.ts（+207/−111）

### Git Commits

| Hash | Message |
|------|---------|
| `8688155` | feat(theme): 首页节点卡片按 ping 任务分行显示延迟/丢包/历史条（本地小样，未部署） [task:10-25-region-rows-prototype] |

### Testing

- [OK] bun run type-check 通过；bunx eslint 3 文件通过；bun run build-only 通过（4.3s）
- [OK] Playwright 本地渲染截图：3 行任务名 + 延迟 + 丢包 + 每行历史点阵，comfortable/compact/cvf 均正常

### Status

[OK] **Completed**

### Next Steps

- 待用户审阅截图：确认版式后决定是否更新视觉快照、bump manifest 与部署（部署需另行授权）


## Session 70: 发布 ink 主题 0.0.2：按 ping 任务分行版式（构建 + 部署，未 push）
<!-- trellis-session: v=2 fp=b91596a99ea56fa3 -->

**Date**: 2026-10-25
**Task**: 发布 ink 主题 0.0.2：按 ping 任务分行版式（构建 + 部署，未 push）
**Branch**: `main`

### Summary

按主仓任务 10-25-theme-release-versioning 的用户授权（选择「发主题 0.0.2 并部署」）：manifest 0.0.1→0.0.2、同步 VERSION.md → 提交 570b6ec → ./build.sh --skip-server --skip-install 构建出 ops-dist/theme/ink-build-570b6ec.zip（sha256 1bc49901…，包内 manifest 0.0.2，zip 名与 HEAD 一致）→ 本地 annotated tag theme-ink/v0.0.2 → deploy-theme.sh --dry-run → 正式部署到唯一服务器（备份 theme-ink-0.0.1_20261025031052.tgz，759 条目；解包 767，替换后 759 文件）。未重建 Komari 服务端/镜像/容器，未 push。

### Main Changes

- themes/ink/komari-theme.json version 0.0.1→0.0.2；ops-docs/VERSION.md 当前版本同步 0.0.2；主题产物 ops-dist/theme/ink-build-570b6ec.zip（+sha256、BUILD-INFO）

### Git Commits

| Hash | Message |
|------|---------|
| `570b6ec` | chore(theme): 发布 ink 主题 0.0.2（首页节点卡片按 ping 任务分行） [task:10-25-theme-002-release] |

### Testing

- [OK] [OK] build.sh --skip-server --skip-install：type-check + vite build 通过；包内 manifest=0.0.2；sidecar sha256 一致
- [OK] [OK] dry-run 计划正常；正式部署完成：上传→远端 sha256 一致→备份 759 条目→解包 767→manifest 0.0.2
- [OK] [OK] 部署后只读核对：主题入口 index-08OsQRET.js → index-CwsnpWBc.js；容器 Restarts=0、StartedAt 未变；/api/version 仍 0.0.24+own.4；站点 / 与 /admin 均 200；真实页面 3 行版式正常

### Status

[OK] **Completed**

### Next Steps

- 后续（不在本任务）：更新 3 张视觉快照；评估防「后台更新/市场」覆盖主题的加固（改 manifest url 等）需另立任务
