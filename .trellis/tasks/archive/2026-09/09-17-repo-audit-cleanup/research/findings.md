# 仓库体检发现（2026-09-17，只读取证）

每条都带可复现命令与关键输出。**A 类**= 不动产物、不牵动发版，可直接处理；
**B 类**= 牵动产物/历史/发版或删数据，需用户确认。

## 0. 结论摘要

| 维度 | 状态 |
|---|---|
| 版本口径（0.0.16） | ✅ 4 处字面量 + MAINTAINING 全一致 |
| 发布与资产 | ✅ release `0.0.16` 18 个资产齐全，含两份 SHA256SUMS |
| Git 卫生 | ✅ 工作区干净、无 stash、与 origin 一致；⚠️ 一个本地遗留分支 |
| 我们自己的代码 TODO/FIXME | ✅ 零 |
| 文档口径 | ⚠️ `web/rpc/jsonrpc/README.md` 仍写"插件扩展点/插件待实现"与 notification 方法；MAINTAINING §7 有一条已修项未更新；README 自检命令与实际工具不一致 |
| 死代码 | ⚠️ 前端 2 个 Context 完全无引用；5 个语言包里 `plugin` 文案块无任何代码引用 |
| 本地磁盘 | ⚠️ `.build` 3.7G（其中真在用的仅 zig 工具链 ~450M + 发布说明/截图 ~6M） |

## A 类（本次直接处理）

### A1. `web/rpc/jsonrpc/README.md` 与真实架构不符
- L45「声明自定义权限（供插件使用）」、L98-107「## 插件扩展点（地基已就位，插件本身待实现）」、
  L53「plugin:* 与 plugin:publicStat 可共存」、L139「web/api/public（login/logout/oauth/**plugin**）」。
- 取证：`grep -rn "plugin" web/api/public/ web/api/admin/ --include="*.go"` → 空；
  `ls -d plugin pkg/plugin` → 不存在；README 顶部又写着"没有插件系统"（README.md:24）。
- 处置：删掉插件章节，把 ACL 示例换成中性命名空间；修正 public 路由说明。

### A2. 同一文件 L116 列了 `notification（load/offline/traffic）` RPC 方法
- 取证：`grep -rn "notification:" web/rpc/jsonrpc/*.go` → 空；
  `ls -d internal/notification pkg/notification` → 不存在；
  MAINTAINING §6 记录通知系统已整体移除。
- 处置：从"已迁移接口"表里去掉 notification 方法描述。

### A3. `pkg/rpc/registry.go:36` 注释仍用 plugin 语汇
- 原文：`// plugin-owned methods that do not provide explicit metadata.`
- 处置：改为"注册方未提供显式元数据的方法"（行为不变，仅注释）。

### A4. MAINTAINING §7 有一条**已修但未更新**的遗留
- L355-358「升级弹窗在未登录时会显示按钮…下次发版把条件收紧为
  `upgradeStatus && upgradeStatus.enabled !== false` 即可」。
- 取证：`grep -n "upgradeStatus && upgradeStatus.enabled" frontend/src/components/admin/AdminPanelBar.tsx`
  → L633、L714 已是收紧后的条件（0.0.14/0.0.15 期间修掉）。
- 处置：标注为已修（并给出代码行号），从"待办"语境移出。

### A5. MAINTAINING §2.1 的检查项枚举漏了第 8 项闸门
- §2.1 仍写"秒级：版本字面量、文档路径/锚点、脚本语法、脏文件、密钥扫描、产物哈希、无克隆上游"，
  `--full` 追加项也没提闸门与 YAML 检查。
- 处置：补上"第 8 项 Trellis 流程闸门 + 工作流 YAML 可解析"。

### A6. README 的"提交前自检"与实际工具链不一致
- README.md:117-121 只给 `go build ./... && go vet ./... && go test ./...`，
  而仓库真正的自检入口是 `./scripts/check-repo.sh`（含版本/文档/产物/闸门 11 项）。
- 处置：README 改为指向 `./scripts/check-repo.sh` / `--full`，Go 命令作为其中一部分提及。

## B 类（需确认后才能动）

### B1. `.build` 3.7G 里绝大部分是一次性实验与缓存
`du -sh .build/*` 关键项：`gocache` 1.8G、`tools` 449M（**zig 工具链，必须留**）、`gopath` 411M、
`komari-web` 336M、`zig-cache*` 221M、`statictest` 128M、`rel*` 146M、`server-image`+`agent-image` 102M、
`npm-cache` 63M、`decoy-test` 22M、`agent-src*`+`agent-spike` 8M、`shots` 2M（**证据截图，建议留**）。
- 关键取证：`go env GOMODCACHE GOCACHE` → `/root/go/pkg/mod`、`/root/.cache/go-build`，
  即 `.build/gopath`、`.build/gocache` **不是**构建脚本在用的缓存（脚本里只引用 `.build/tools` 找 zig、
  `.build/defaultTheme-stage`、`.build/check-agent`）。
- 处置建议：删 `gocache/gopath/komari-web/zig-cache*/statictest/rel*/server-image/agent-image/npm-cache/
  decoy-test/agent-src*/agent-spike/komari/check-agent`，保留 `tools/`、`shots/`、`rel-notes-*.md`。
  预计回收 ~3.2G。

### B2. `dist/` 236M：当前 release（0.0.16）的产物副本
- 18 个资产已发布在 GitHub Release；本地这份是构建产物（可重建）。
- 处置建议：可留（离线复用），也可只保留 `komari-SHA256SUMS`/`komari-agent-SHA256SUMS` 两份校验和。

### B3. 本地分支 `backup/pre-rewrite`（仅本地，无远端对应）
- `git branch -vv` → 指向 `9d2bb44 docs: 用分支引用替代硬编码的重写前 HEAD`，
  是重写历史前的安全分支。远端只有 `origin/main`。

### B4. 前端死代码 + 死文案（清掉需要重建产物 + 发一版才有意义）
- `frontend/src/contexts/NotificationContext.tsx`（61 行）、`LoadAlertContext.tsx`（84 行）：
  导出名在 `frontend/src` 全树零引用（`OfflineNotificationProvider` / `useOfflineNotification` /
  `LoadAlertProvider` / `useLoadAlert` 全空）。
- 5 个语言包的 `plugin` 文案块（`zh_CN/en/zh_TW/ja_JP/id_ID.json` 约 L630-670）：
  `grep -rn "plugin" frontend/src --include="*.ts*"` 只命中 locales 自身 → 死键。
- 影响：`frontend/` 是内嵌资源的源，改完要 `./scripts/build-frontend.sh` + 更新
  `FRONTEND_TREE_SHA256`，用户侧还要发一个新版本才能拿到。
- 附注：粗扫"未被 import 的文件"会报出 pages/* 等 40 个，属于**误报**——路由是注册表/glob 装载、
  目录导入（`@/utils`）等原因；仅上面两类经人工确认无引用。

### B5. 其它 §7 遗留（今日未动，供排期）
- 面板"文档"链接仍指向上游文档站（改需前端补丁 + 发版）；
- 两个 Dockerfile 基础镜像用 `alpine:3.21` tag 而非 digest；
- agent 3 个依赖外网的测试（`TestICMPPing`/`TestTCPPing`/`TestHTTPPing`）在无外网机器必红；
- 升级重启后 WAL unlink 的偶发现象（观察记录，处置=重启一次）；
- `raw.githubusercontent.com` 新文件短暂 404 窗口（观察记录）。

## 未覆盖 / 未取证的方面（诚实边界）

- Go 侧未做"死函数/死包"级分析（只有 `go vet` 与构建通过；未跑 deadcode 类工具）；
- 前端"死文件"只做了人工确认的两类，不做全量判定（误报率高，已在 B4 注明原因）；
- 未审计 `frontend/node_modules`、`.build/gopath` 等第三方内容。

---

# 执行结果（用户 2026-09-17 全选 B 类后的处理）

| 项 | 结果 | 证据 |
|---|---|---|
| A1-A3 文档/注释残留 | ✅ 已清理并提交 `fc1b597` | `grep -n "plugin\|notification" web/rpc/jsonrpc/README.md` → 空 |
| A4-A6 口径对齐（§7 已修项、§2.1 检查项、README 自检命令） | ✅ | `./scripts/check-repo.sh` 全部通过 |
| B1 `.build` 瘦身 | ✅ 3.7G → **451M** | 保留 `tools/`（zig 工具链 449M）、`shots/`、`rel-notes-*.md`、`rel-server.sha256`、`traffic-samples.csv`、`traffic_sampler.py`；其余一次性实验/缓存全部删除（`du -sh .build`） |
| B2 `dist/` 精简 | ✅ 236M → **12K** | 只留 `komari-SHA256SUMS`、`komari-agent-SHA256SUMS`（0.0.16 的 18 个资产已在 GitHub Release） |
| B3 删本地分支 | ✅ 已删 | 删前记录 SHA `9d2bb44`，恢复方法写进 MAINTAINING §10；`git branch -a` 只剩 main |
| B4 前端死代码 | ✅ 已清 + 重建 + 重新固定哈希 | 删 2 个无引用 Context 与 5×71 行 plugin 文案块；哈希 `b860e30d…` → `cc8f1c10…` |
| B5 Dockerfile 钉 digest | ✅ 两个 Dockerfile 都改为 `alpine:3.21@sha256:48b0309c…07d` | `docker pull` 该 digest 成功；MAINTAINING §7 条目改写（含"安全更新不再自动进来"的代价） |
| B5 agent 外网测试 | ✅ 默认跳过（`KOMARI_AGENT_ONLINE_TESTS=1` 才跑） | `cd agent && go test ./server/...` → `ok … 0.005s`（离线全绿，原为 34s 后 3 红） |

> 未做（明确留给后续）：把升级 E2E 脚本（`.build/e2e_upgrade.py`，已删）重写为仓库内正式工具；
> Go 侧 deadcode 扫描；面板"文档"链接指向上游的修正（需前端补丁 + 发版）。
