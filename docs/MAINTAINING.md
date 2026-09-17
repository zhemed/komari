# 维护本仓库（komari 自维护版 · 当前 0.0.8）

本仓库是由 **zhemed 独立维护的 komari 发行版**：版本线从 **0.0.1** 起步（当前 **0.0.8**），
服务端、面板前端与 agent 的**源码都在本仓库内**，构建不克隆上游、可离线构建。
上游 komari 只作为 1.4.3 的历史来源，**不是本仓库的发行方**。

- 上游后端：<https://github.com/komari-monitor/komari>
- 上游前端：<https://github.com/komari-monitor/komari-web>
- 上游 agent：<https://github.com/komari-monitor/komari-agent>
- 本仓库：<https://github.com/zhemed/komari>

> **代码来源**：0.0.1 由上游 `komari@1.4.3`（commit `bf6b45ec…`）+ `komari-web@1.4.3`（commit `4a74e8a8…`）派生。
> 1.4.3 只是**历史来源**，不是我们对外声明的版本；对外一律是 0.0.x。

---

## 1. 版本与固定点

| 组件 | 固定值 | 说明 |
|---|---|---|
| 项目版本 | `0.0.8`（唯一默认值在 `scripts/version.env`） | 构建时由 `scripts/build-komari.sh` 以 ldflags 注入 `CurrentVersion`；agent 用同一版本号 |
| 后端代码来源 | 上游 tag `1.4.3` → `bf6b45ec3abfc56bba5e9223650a47a72f665371` | 主干分支 `komari-1.4.3`（分支名保留历史来源，不代表版本号） |
| 前端源码 | **在本仓库**：`frontend/`（上游 tag `1.4.3` → `4a74e8a8…` 的快照 + 我们内联的改动） | 溯源与构建参数在 `scripts/frontend-build.env` |
| 前端产物 | `web/public/defaultTheme/`（已提交进仓库） | 目录树哈希记录于 `scripts/frontend-build.env` |
| agent 源码 | **在本仓库**：`agent/`（上游 commit `1186aafb…`，2026-08-07，**1.4.3 同期**的快照 + 我们内联的改动） | 溯源与构建参数在 `scripts/agent-build.env`；见第 11 节 |
| agent 资产 | `komari-agent-<os>-<arch>`（14 个平台） | 与服务器资产**同一个 release**，由 `scripts/build-agent.sh` 构建 |
| agent 镜像 | `ghcr.io/zhemed/komari-agent:<版本>` / `:latest` | 由 `scripts/build-agent-image.sh --push` 推送 |

**为什么前端源码/产物都在仓库里**：`web/public/public.go:18` 是 `//go:embed defaultTheme`，
主题产物缺失时 `static()` 会在 `public.go:130` 直接 panic——即后端单独无法构建。所以：
产物 `web/public/defaultTheme/` **提交进仓库**（离线可构建），源码 `frontend/` 也在仓库里
（改前端不用克隆上游）。上游 CI 里的 `.github/actions/build-frontend/action.yml:34`
（该目录已从本仓库移除）在普通 tag 下会退化为克隆前端**默认分支**，这也是我们自己 vendor 的原因之一。

**发版本规则**：递增三段中的 patch 位（`0.0.2`、`0.0.3`…）。
前端 `AdminPanelBar.tsx` 的 `parseSemver` 只取 `x.y.z` 三段并要求严格递增，
所以带后缀的 tag（如 `0.0.1-fix1`）**永远不会**被判为"可更新"。

## 2. 工具链

| 组件 | 要求 | 实测 |
|---|---|---|
| Go | ≥ `1.25.0`（以 `go.mod:3` 为准） | `go1.26.6` 通过 |
| gcc | 必需（`CGO_ENABLED=1`，`mattn/go-sqlite3`） | `gcc 11.4.0` 通过 |
| zig | 仅**静态**发布构建时需要（服务器） | `zig 0.16.0` 通过 |
| Node / npm | 仅"重新生成前端"时需要 | `node v22.23.2` + `npm 10.9.8` 通过 |
| （agent 构建） | **纯 Go，`CGO_ENABLED=0`——不需要 gcc、不需要 zig** | `go1.26.6` 通过 |

> 上游 `.github/workflows/release.yml:105`（已随该目录移除）写的是 `go-version: "1.23"`，
> 与 `go.mod` 的 `1.25.0` 不一致；**以 `go.mod` 为准**。

## 2.1 仓库自检（一条命令跑完机械检查）

```bash
./scripts/check-repo.sh          # 秒级：版本字面量、文档路径/锚点、脚本语法、脏文件、密钥扫描、产物哈希、无克隆上游
./scripts/check-repo.sh --full   # 追加：go build/vet/test、离线构建、agent 三道门禁
```

它只做**能机械判定**的检查，不猜意图；任何一项不通过都会打印具体文件与原因并以非 0 退出。
涉及的检查项与"为什么这样查"都写在脚本头部注释里。**改文档/脚本后应当跑一次快速检查，
发版前跑一次 `--full`。**

## 3. 日常操作

### 3.1 构建（只需 Go，不需要网络与 Node）

```bash
./scripts/build-komari.sh          # 输出 bin/komari
```

主题产物已 vendor 在仓库内，克隆后即可直接构建。若 `web/public/defaultTheme/` 缺失，
脚本会明确报错并指出这正是 `public.go:130` panic 的根因。

### 3.2 重新构建前端产物（改了 frontend/ 才需要，需要网络 + Node）

```bash
./scripts/build-frontend.sh
```

脚本会：在仓库内的 `frontend/`（上游快照 + 我们内联的改动）里 `npm ci`
（依 `package-lock.json` 锁定）→ `npm run build` → 原子替换 `web/public/defaultTheme/`
→ 校验目录树哈希与上游地址残留。**不克隆上游、不打补丁**；只改后端的人不需要跑它，
因为产物已提交进仓库。

哈希不一致时脚本会失败并给出实际值：确认接受后更新 `scripts/frontend-build.env`
的 `FRONTEND_TREE_SHA256`，并在提交信息里说明原因。

**为什么 `SOURCE_DATE_EPOCH` 是产物可复现的前提**：上游 `vite.config.ts:53` 取
`new Date().toISOString()` 并经 `define.__BUILD_TIME__`（`vite.config.ts:125-126`）注入产物，
最终由页脚 `src/components/Footer.tsx:26` 显示。这个每次构建都不同的常量会让承载它的 chunk
改名 → 所有引用它的 chunk 连锁改名 → 索引与 Service Worker 的预缓存 revision 同步变化，
于是同源码同 lockfile 两次构建的哈希必然不同。`frontend/vite.config.ts` 里的 `buildTime`
优先读取 `SOURCE_DATE_EPOCH`（[reproducible-builds](https://reproducible-builds.org/docs/source-date-epoch/)
标准约定），脚本把它设为**上游 commit 的提交时间**（`KOMARI_FRONTEND_SOURCE_DATE_EPOCH=1786612131`），
产物因此确定可复现。

> 沙箱/受限环境下若 `~/.npm` 不可写，可加 `npm_config_cache=<某可写目录>` 前缀。

### 3.3 静态构建（发布用）

发布产物必须是**静态链接**的：`Dockerfile` 基于 `alpine:3.21`（musl），glibc 动态二进制在其中
无法运行；而 glibc 静态虽然能链接成功，但 `getaddrinfo`/NSS 依赖宿主共享库，不作为发布形态。

```bash
KOMARI_STATIC=1 ./scripts/build-komari.sh                      # linux/amd64 静态（需 zig）
KOMARI_STATIC=1 KOMARI_GOARCH=arm64 ./scripts/build-komari.sh  # linux/arm64 静态
```

- zig 缺失或不可用时脚本**明确报错**，不会静默退化为动态链接（否则 alpine 部署会在运行期才失败）。
- 受限环境下若 zig 默认缓存目录不可写，可设 `ZIG_GLOBAL_CACHE_DIR` / `ZIG_LOCAL_CACHE_DIR`。

### 3.4 发布一个版本

一次发布 = **整套栈**：服务器静态产物 + 14 个 agent 资产 + agent 镜像。

1. 同步版本字面量（`scripts/version.env` 的 `KOMARI_VERSION`、`install-komari.sh` 的 `REPO_TAG`、
   `install-agent.sh` 的 `default_agent_version`、`install-agent.ps1` 的 `$DefaultAgentVersion`）。
   **发布说明的措辞门禁**（2026-09-17 事故后加，见 `.trellis/spec/guides/evidence-and-claims-guide.md`）：
   说明里每写一条"修复/原因/已知问题"，都必须能指向一条判别性验证（命令或测试）写进正文；
   是推断就要标置信度，是"已知问题"就要给下一步实验，不许把推断写成"已定位"。
2. 服务器静态产物（需 zig）：
   ```bash
   KOMARI_STATIC=1 KOMARI_OUTPUT=dist/komari-linux-amd64 ./scripts/build-komari.sh
   KOMARI_STATIC=1 KOMARI_GOARCH=arm64 KOMARI_OUTPUT=dist/komari-linux-arm64 ./scripts/build-komari.sh
   ```
3. agent 全平台（14 个，纯 Go）：`./scripts/build-agent.sh`
3.5 生成服务端校验和资产（面板一键升级按它校验下载内容，0.0.8 起必需）：
   ```bash
   ./scripts/gen-release-sums.sh        # 产出 dist/komari-SHA256SUMS（两行：amd64/arm64）
   ```
4. 自检：`./scripts/check-repo.sh --full` 必须全绿（含版本字面量一致性、文档锚点、
   `go build/vet/test`、离线构建、agent 三道门禁、前端产物哈希）。
   自检里的 agent 门禁构建到 `.build/check-agent`，**不会动 `dist/`**；反过来说，
   `scripts/build-agent.sh` 默认写的就是 `dist/agent` 并会先清空该目录，别拿它当临时构建用。
5. `git tag <版本> && git push origin <版本>`
6. `gh release create <版本> -R zhemed/komari --title "<版本>" --notes-file <说明.md> \
      dist/komari-linux-amd64 dist/komari-linux-arm64 dist/komari-SHA256SUMS \
      dist/komari-agent-SHA256SUMS dist/agent/komari-agent-*`
   （共 18 个资产：服务端 2 + `komari-SHA256SUMS` + agent 14 + `komari-agent-SHA256SUMS`。
   0.0.8 起服务端也有校验和资产——面板一键升级与 `install-komari.sh` 都按它校验；
   缺该资产的旧 release（≤0.0.7）在升级界面里会被明确拒绝："请用 install-komari.sh 升级"）
7. 推送镜像：`./scripts/build-server-image.sh --push && ./scripts/build-agent-image.sh --push`

> **顺序很重要**：服务器二进制的版本 hash 来自构建时的 `git rev-parse HEAD`，
> 且 Go 会把 VCS 信息也编进去。所以**先提交、后构建**，否则 release 里的二进制
> 声称的 hash 与实际 tag 不一致（0.0.4 发布时踩到过一次）。

tag 与 `KOMARI_VERSION` 必须一致（`install-komari.sh` 默认按 `KOMARI_TAG` 拉取）。
**同一个 release 里同时有服务器与 agent 资产是必需形态**——agent 靠补丁 0001 的资产过滤
区分两者（见 §11.1）。**本仓库没有 CI**（上游流水线已移除），发布必须手动执行以上步骤。

> **`gh` 陷阱（0.0.1 发布时实际踩到）**：本仓库有两个 remote（`origin`=自有、`upstream`=只读参考），
> `gh release create` 可能把仓库解析成 `upstream`，报
> `tag 0.0.1 exists locally but has not been pushed to komari-monitor/komari`。
> 发布时给 `gh` 显式加 `-R zhemed/komari`。

### 3.5 命令行子命令

服务器只有一个二进制，子命令见 `cmd/`（实测 `komari --help`）：

| 命令 | 用途 |
|---|---|
| `komari server` | 启动服务；`-l/--listen` 指定监听地址（默认 `0.0.0.0:25774`，也可用环境变量 `KOMARI_LISTEN`） |
| `komari chpasswd` | 忘记密码时强制改管理员密码 |
| `komari disable-2fa` | 丢弃 2FA 配置 |
| `komari permit-login` | 恢复密码登录（例如 SSO/OIDC 配置出错登不进去时） |

全局参数：`-d/--database`（默认 `./data/komari.db`）、`-t/--db-type`（默认 `sqlite`）。
CLI 的帮助文本里保留着上游作者署名（`Made by Akizon77 with love.`），归属信息同时由
`LICENSE` / `NOTICE` 承载。

### 3.6 数据与备份

| 路径 | 说明 |
|---|---|
| `./data/komari.db` | 主库（配置、账号、节点） |
| `./data/metrics.db` | 指标库 |
| `./data/theme/` | 已安装主题 |
| `./data/backup/` | 备份归档 |
| `komari.db` 的 `client_traffic_totals` | **跨重启的流量累计**（本仓库自有扩展，`database/models/traffic.go`） |

- 数据目录跟**工作目录**走（Docker 镜像里是 `/app/data`，systemd 单元里是 `/opt/komari/data`）。
- 二进制的版本标识（`CurrentVersion-VersionHash`）与库中记录不一致时，启动会先把整个 `./data`
  打包到 `data/backup/upgrade-<时间>.zip` 再继续（见 §7 最后一条）。
- 后台可以上传备份并自动重启以应用。

#### 流量累计为什么单独存一张表

上游把面板“总流量”直接显示 agent 报的**开机以来计数器**（`/proc/net/dev`），因此**机器一重启，
这个数字就归零**——历史上无法保存流量。我们的做法：服务端复用 metric store 已经算好的
**重置感知增量**（`internal/metricstore/report_batcher.go` 的 `TrafficCounterDelta`），
在 `client_traffic_totals` 里持续累加：

- 首次见到某节点用**当时的计数器做基线**（避免功能上线后数字跳变），之后只加增量；
- 因此 **agent 重启、机器重启（计数器归零）、服务端重启都不会让累计回退**；
- 面板的卡片“总流量”和流量阈值进度读的是它（`web/rpc/jsonrpc/common.go` 的
  `getNodesLatestStatus`）；没有累计行时回退到实时计数器；
- 删除节点会一并删除该行（`database/clients/client.go` 的 `DeleteClient`）。

接线方式：`internal/server/metric_store.go` 在 metric store 就绪后调用
`clients.InitTrafficTotals()` 并用 `metricstore.SetTrafficAccumulator(...)` 注册回调，
metricstore 不反向依赖 `database/*`。

## 4. 与上游的解耦点

| 位置 | 改动 | 原因 |
|---|---|---|
| `web/public/defaultTheme/` | vendor 进仓库 | 让后端无网络/无 Node 也可构建（见第 1 节） |
| `web/public/.gitignore` | 取消忽略 `defaultTheme/*` | 上游默认忽略该注入目录，不改则 vendor 产物根本提交不进去 |
| `frontend/`（源码内联） | 更新检查指向 `zhemed/komari`、构建时间可用 `SOURCE_DATE_EPOCH` 覆盖、删除插件系统、去掉非 HTTPS 告警横幅、删除通知系统、安装命令与镜像指向本仓库 | 这些原本是补丁 0001–0006，源码 vendor 化后**已内联进 `frontend/`**，见第 4 节与第 11 节 |
| `agent/`（源码内联） | 自更新指向本仓库 + 资产过滤 + 容器内跳过 + 默认关自动更新 | 原本是 patches-agent 0001–0003，已内联进 `agent/`；理由见第 11 节（含实测证据） |
| `install-komari.sh` | 指向自有仓库并锁定 tag；`curl -f` + 先下临时文件再替换 | 不再安装上游 1.5.x；失败时不写入错误页、不截断运行中的二进制 |
| `.gitignore` | 忽略 `/.build/`、`/bin/`；把上游 `komari` 规则锚定为 `/komari` | 后者原为未锚定规则，会连带忽略 `.trellis/workspace/komari/`，使跨会话记忆无法提交 |
| `Dockerfile` | `ARG TARGETOS/TARGETARCH` 给出默认值 `linux/amd64` | 让普通 `docker build`（非 buildx）也能定位上下文里的二进制 |
| `.github/workflows`、`.github/actions`、`.github/ISSUE_TEMPLATE` | 全部删除 | 上游流水线会从前端**默认分支**构建、并向 `ghcr.io/komari-monitor` 推镜像，对本仓库是错误产出 |
| `README.md`、`README_zh-cn.md` | 删除上游版本，改为我们自己的单份 `README.md` | 上游 README 含上游徽章/部署按钮/截图与升级到 1.5.x 的指引 |

更新检查的目标仓库可在构建期覆盖：

```bash
VITE_KOMARI_UPDATE_REPO=owner/repo ./scripts/build-frontend.sh
```

## 5. 插件系统已移除

**本仓库不包含插件系统**，这是刻意决定（见任务 `.trellis/tasks/archive/2026-09/09-16-rebase-0.0.1-drop-plugins/`）：

- 已删除：`internal/plugin/` 整包、插件市场（后端 API + 前端页面）、插件 RPC、`/api/plugin/*`
  公开路由、插件模型与迁移注册、备份白名单中的插件目录、WebSocket 插件帧拦截器、
  前端插件页面/路由/菜单/类型。
- **不要再重新引入**：升级上游代码时若带回这些文件，必须重新剔除。
- 数据库中的历史插件表**保留**（孤儿表），未做破坏性迁移。
- `pkg/jsruntime/` 在 0.0.3 已随通知系统一并移除（见第 6 节）；插件系统移除时它确实还被需要，两者不再共存。
- 主题系统与主题市场**保留**，且主题本就没有版本门禁，不受版本号影响。
- 流量报告：内置实现**仍可用**（上游计划在 1.5.0 移除，我们停留在 1.4.3 基线，故不受影响）；
  原先指向插件市场的引导提示已删除。

## 6. 通知系统与内嵌 JS 运行时已移除

**本仓库不包含通知系统**（0.0.3 起），与插件系统同样属于刻意决定：

- 已删除：`utils/messageSender/`（框架 + 8 个渠道：bark/email/javascript/serverchan3/
  serverchanturbo/telegram/webhook/empty）、`utils/notifier/`（离线、负载、流量、
  流量报告、到期提醒）、`database/notification/`、通知相关模型与迁移步骤、
  通知 RPC 与路由、`admin:testSendMessage`、调度任务（`notifier:traffic`/`notifier:expire`）、
  以及 `internal/config/settings.go` 里的通知配置项。
- 调用点重接线（0.0.3 实际改动）：agent 上/下线改为 `logger.Infof("client-api", ...)`
  日志（`web/api/client/report.go`、`report_v2.go`）；登录改为 `auditlog.EventLog("auth", ...)`
  审计记录（`database/accounts/sessions.go`——此前登录**只有**通知这一条痕迹，故必须补）；
  续费路径本就有 `auditlog.EventLog("renewal", ...)`，无需补。
- 前端（补丁 `0005-drop-notification-system.patch`，纯删除 1,789 行）：删除
  `pages/admin/notification/`（channels/general/load/offline/traffic_report）、通知菜单组与路由。
- **`pkg/jsruntime`（46 文件 / 11,427 行）随之一并移除**：它的消费者只剩插件系统（更早移除）
  与 JavaScript 通知渠道；同时从 `go.mod` 去掉了 goja / goja_nodejs / base64dec。
- 数据库中的历史通知表**保留**（孤儿表），迁移步骤已删除但表与数据不动。
- **不要再重新引入**：从上游 cherry-pick 时若带回这些文件，必须重新剔除。

注：`pkg/rpc` 里的 `NewNotification` 是 JSON-RPC 协议概念（无 id 的通知型请求），与此无关，保留。

## 7. 已知遗留与注意事项

- **安全修复不会自动到来**：上游 1.5.x 之后的修复需我们自行判断是否 backport。
  决定采纳时有意识地 `git fetch upstream <ref>` 后 cherry-pick——`upstream` 的 fetch refspec
  目前被锁在 tag 1.4.3，这是防止误引入 1.5.x 的**安全默认**，不要随手改掉。
- **关于页/GitHub 按钮已指向我们**（补丁 0006）：`src/pages/admin/about.tsx` 读的是本仓库 README。
- **`install-komari.sh` 的 tag 是字面量**：它是给 `curl | bash` 用的独立脚本，没法在运行时读
  `scripts/version.env`，发版要手动同步（见 §3.4 第 1 步）。
- **面板“文档”链接仍指向上游文档站**：`menuConfig.json` 的 `common.documentation` →
  `komari-document.pages.dev`。上游文档描述的是 1.4.3/1.5.x 的行为，与本仓库（无插件/无通知）
  有出入。要改得加前端补丁并**重新发版**（前端内嵌在服务器二进制里），暂留。
- **流量增量"个别分钟偏低/偶发 2×"已定位并修掉**（2026-09-17 实测：**不是数据丢失**，
  是查询端的聚合语义）：

  - **记录本身是准的**：60s 桶端点对齐后逐分钟核对（本桶 Σ增量 vs 相邻桶计数器差值），
    636 个分钟**逐分钟零误差**（差值全为 0 B），10.5 小时全窗口差额 +336 B（+0.00%），
    计数器零回退。面板"总流量"读的累计表由同一份增量喂入，因此同样准确。
  - **更正一条旧记录**：本文件先前记的"05:38 整分钟只记到 5.5 KB，而计数器涨了 190 KB"
    **是错的**（当时混着 60s 与 300s 两种分辨率在比）：实测该分钟 ΔC = 5,554 B、
    ΔR = 5,554 B，完全一致。"增量扎堆"（一条 2.7 MB、其余约 144 B）也只是同一次突发
    被分摊到一条上报上的正常形态，不是重复计数或丢数。
  - **真正的根因**：`queryMetrics` 对 `traffic.up/down` 采用了客户端的全局聚合偏好
    （面板默认 `avg`，前后端默认都是）。这两个指标每个采样点的含义是"两次上报之间的字节数"，
    再取平均等于把点位又除以桶内采样条数：实测 60/60 分钟的"真实值 ÷ 点值"**精确等于该桶
    采样条数**（20）。桶内条数不齐时（未封桶、回退窗口）缩放比随之变化，同一张图上相邻分钟
    被除以不同的数，看着就是"个别分钟偏低/偶发 2×"；24 小时视图切到 300s 桶后条数变为 ~100，
    量级再差 5 倍。
  - **修法**：把"按指标语义固定"的聚合（`traffic.up/down → sum`、`net.total.up/down → last`，
    即 records 路径里早就存在的 `recordMetricAggregation` 约定）抽成
    `metricstore.SemanticAggregation`，并让 `resolveMetricAggregation` 的优先级变为
    **按指标显式指定 > 语义默认 > 全局指定 > avg**。前端无需改动（面板照旧发 `avg`，
    服务端按语义纠正）；确实要覆盖语义默认时用 `aggregation_by_metric`。
  - **验证方式**（不改动线上部署）：复制 `data/`、用修复版二进制在备用端口起实例，
    按面板同款参数（`hours=1/24`、`aggregation=avg`、`max_points=700`）与线上的 0.0.6 对拍——
    线上每分钟点值恰为真实值的 1/20，修复版**逐分钟精确等于**真实值；`hours=24` 的 300s 桶值
    等于该 5 分钟内各分钟之和（如 11:25 桶 1,653,537 B = 294+300+2,661+942+1,649,340）。

  **一个已修掉、但从未被证实触发过的缺陷（0.0.6 写成"确因"，是过度断言，在此更正）**：
  v2 协议的报告没有 `uptime` 字段，服务端读到 0，而"agent 是否重启"的判据是
  `report.Uptime < values.uptime`；**若**节点在 v2 与 v1 之间切换（v2 整体失败降级到 v1，
  之后重连回 v2——这是 agent 里真实存在的代码路径），两侧的 uptime 会互相误判成重启、
  把该条上报的增量清零。判据已改为"两次上报都必须带有效 uptime"
  （`report_batcher.go`），回归测试 `TestWriteReportKeepsTrafficWhenUptimeMissing`。
  **证据边界**：本机 agent 的表报日志里只有 v2 WebSocket（从未降级），所以这个缺陷
  有没有在真实环境触发过，我们**没有证据**；当初把它写成"已定位的原因"是把
  "代码路径上必然发生"当成了"实测观测到"，这是本次错误的第二个来源。

  **关于"两条通道"的更正**：agent 的设计是**同一时刻只走一条上报通道**——
  v2 WebSocket（首选）→ 连不上时进 v2 HTTP POST 回退（报告 POST + 事件 pull 两条 POST 循环，
  见 `agent/server/websocket.go` 的 `runPostFallback`/`runV2PullLoop`）→ v2 端点整体失败时
  降级到 v1（`/api/clients/report`）直到连接断开。服务端日志里的 `online (POST session)` **不等于**
  v1 通道：任何 POST 形态的 ingest 都会刷新 presence 而打出这条日志（v2 的 HTTP 入口也标
  `markPresence=true`，`report_v2.go:53`）。本机 agent 从未进过回退/降级（日志里只有
  "WebSocket connected using v2 protocol"）。

- **两个 Dockerfile 的基础镜像用 tag 而非 digest**：`alpine:3.21` 会随上游更新而变，
  同一份源码在不同时间构建的镜像不完全可复现；二进制产物本身可复现。
- **已装在别处的上游 agent 无法被我们改写**：见 §11.4。
- **agent 自带测试里有 3 个依赖外网的**：`agent/server/task_test.go` 的 `TestICMPPing` /
  `TestTCPPing` / `TestHTTPPing` 会 ping 硬编码的外部目标，在无外网或目标不可达的机器上必然失败
  （实测本机 34s 后 3 个全红，其余包全绿）。本地自检用离线安全子集：

  ```bash
  (cd agent && go test ./monitoring/... ./terminal/... ./update/...)
  ```
  正因如此，`scripts/build-agent.sh` **不把 agent 测试当发布门禁**（会变成 flaky）。
- **偶发：升级重启后 `data/komari.db-wal` 被 unlink，外部读到旧数据**（2026-09-16 实测一次）。
  现象：`0.0.4 → 0.0.5` 升级重启后，服务端进程持有 `komari.db-wal`/`-shm` 的 fd，但文件已从
  目录消失（`ls -l /proc/<pid>/fd` 显示 `(deleted)`），此时用外部 `sqlite3` 读 `./data/komari.db`
  看到的是升级前的数据（表现为面板里的节点版本停在旧值）。
  - 已排除：外部只读连接不是原因（重启后连续两次外部 `SELECT` 均未触发，WAL 正常）；
    升级流程也不删 WAL（代码里只有 `PRAGMA wal_checkpoint(TRUNCATE)`，`database/dbcore/dbcore.go:247`）。
  - 处置：**重启一次服务端即恢复一致**（之后外部读到的就是实时数据），本次重启后
    `komari.db-wal` 0 字节、fd 正常、外部读取实时。
  - 影响面：WAL 处于 unlink 状态时，若进程被 `kill -9` 会丢掉尚未 checkpoint 的写入；
    正常 `systemctl stop`（SIGTERM）会 checkpoint 回主库，本次未观察到数据丢失。
  - 结论：升级后发现"外部工具读到旧数据"，先重启一次服务再判断，不要直接当成丢数据。
- **`raw.githubusercontent.com/.../refs/heads/main/<新文件>` 会短暂 404**：面板安装命令用的就是这个
  路径形式。0.0.4 实测：`main` 推上去后该路径仍 404 约 10 分钟（同一文件用 commit SHA 或
  去掉 `refs/heads/` 的形式立刻 200），随后自愈。刚发完版别急着怀疑脚本没推上去。
- **不要 `git push upstream`**：`upstream` 只作为只读参考。
- **版本切换会触发一次升级备份**：`database/dbcore/dbcore.go:233` 的规则是"版本标识不同即备份"，
  标识为 `CurrentVersion-VersionHash`。因此 1.4.3 → 0.0.1 首次启动会 zip 整个 `./data`
  （`dbcore.go:258`）；之后每次以**不同 commit** 重新构建再启动也会再备份一次（上游同样如此，只是上游只在发版时构建）。

## 8. 回滚

- **回滚前端 vendor**：删除 `web/public/defaultTheme/` 并 `git revert` 对应提交即可；
  注意此时 `go build` 会因 embed 缺失而失败，需重新运行 `build-frontend.sh` 或恢复该目录。
- **回滚安装脚本/补丁**：`git checkout <commit> -- install-komari.sh scripts/`。
- **回滚插件系统移除**：`git revert` 该提交即可恢复插件代码与 1.4.3 版本号（vendored 产物在同一提交内）。
- **回滚 agent 发行线**：删掉 `install-agent.sh` / `install-agent.ps1` / `agent/` /
  `scripts/build-agent.sh` / `scripts/build-agent-image.sh` / `Dockerfile.agent` / `scripts/agent-build.env`
  并 `git revert` 对应改动（然后重跑 `build-frontend.sh`，哈希也要一起回填）。
- **回滚已发布的 agent 资产/镜像**：`gh release delete-asset`、`gh api -X DELETE /user/packages/...`
  （token 有 `delete:packages`）。
- **部署侧回滚**：升级前自动生成的 `data/backup/upgrade-*.zip` 即为回滚素材。

## 9. 克隆与推送

- **只构建不需要历史**：`git clone --depth 1 <本仓库>` 后即可 `./scripts/build-komari.sh`。
- **推送需要完整历史**：若以 `--depth 1` 克隆后直接 `git push`，会因缺少被引用对象报
  `remote unpack failed: index-pack failed`。修复方式（本仓库实际使用）：

  ```bash
  git fetch --unshallow --refetch upstream "+refs/tags/1.4.3:refs/tags/1.4.3"
  ```

  注意仅 `git fetch --unshallow` 可能**无效**（refspec 只覆盖那个 tag 且 tip 未变时不会加深），
  必须显式给出 refspec 并配合 `--refetch`。

## 10. 提交历史与我们自己的仓库

- **历史已于 2026-09-16 重写**：上游 835 个提交不再出现在历史中。上游代码以**单个快照根提交**
  引入（`chore: import komari 1.4.3 (upstream bf6b45ec) as our 0.0.1 code snapshot`），
  我们自己的提交重挂在该根之上，提交粒度保留。
- 重写前的最后一次提交保留在**本地分支 `backup/pre-rewrite`**（未推送）上，作为回滚锚点。
  因此旧 journal 里记录的 hash（如 `18305a9`、`77f36da`）在重写后**已不可达**，只具历史意义；
  需要时以 `backup/pre-rewrite` 为准。
- 「代码来源」由 `LICENSE` / `NOTICE` / `README.md` / 根提交信息承载，而不再由逐行历史承载。
- 需要取回上游历史做 backport 时：`git fetch upstream --tags`（`upstream` remote 保留）。
- 本地曾存在的 68 个上游 tag 已删除，避免上游对象长期驻留；本仓库只推送自己的 tag。
- **前端与 agent 的源码也是快照导入**（2026-09-16，`chore(vendor): import ...`）：
  同样不带上游历史，上游 commit 只作为溯源信息记在 `scripts/frontend-build.env` /
  `scripts/agent-build.env` 与导入提交信息里。

## 11. agent 发行线（0.0.4 起）

**我们不 fork agent**：agent 二进制由本仓库自己构建、作为**本仓库 release 的资产**发布，
所以自有仓库始终只有 `zhemed/komari` 一个（前面说的"三条线"是发布链上的三方，
其中前端与 agent 都只是上游依赖）。

| 环节 | 事实 |
|---|---|
| 源码 | **在本仓库**：`agent/`（上游 `komari-monitor/komari-agent@1186aafb…`，2026-08-07 的快照 + 我们内联的改动；溯源见 `scripts/agent-build.env`） |
| 我们对源码做的改动 | 自更新目标改指本仓库、`selfupdate` 加资产过滤、容器内跳过自更新、自动更新默认关闭、`--enable-auto-update` 新参数；安装脚本见下表 |
| 构建 | `./scripts/build-agent.sh`：在 `agent/` 里纯 Go 交叉编译（`CGO_ENABLED=0`，不需要 zig/gcc），一次出 **14** 个平台；用 `-buildvcs=false`，让产物只取决于源码与注入的版本号 |
| 资产名 | `komari-agent-<os>-<arch>[.exe]`，与上游一致（安装脚本与自更新都按这个名字找资产） |
| 版本号 | 与我们同一条 `0.0.x` 线（`scripts/version.env`），不沿用上游 agent 的 1.x |
| 自更新目标 | `update.Repo = zhemed/komari`（源码默认值 + 构建期 `-X` 双保险） |
| 自更新默认 | **关闭**；开启用 `--enable-auto-update` 或 `AGENT_ENABLE_AUTO_UPDATE=1`（旧的 `-autoUpdate` 仍表示开启） |
| 安装脚本 | `install-agent.sh` / `install-agent.ps1`（放在仓库根，前端的安装命令直指这里）：默认装脚本 pin 的版本，`--install-version latest` 可装最新 |
| 镜像 | `ghcr.io/zhemed/komari-agent:<版本>` 与 `:latest`（多架构 amd64/arm64/armv7），`./scripts/build-agent-image.sh --push`；`Dockerfile.agent` **刻意不含任何 RUN**，否则没有 QEMU/binfmt 的机器上多架构构建会 `exec format error` |

### 11.1 为什么必须给自更新加资产过滤（发布前实测过的坑）

`go-github-selfupdate` 选资产用的是**后缀**匹配（`selfupdate/detect.go` 的 `findAssetFromRelease`：
`strings.HasSuffix(name, "linux-amd64")` 之类），**不看前缀**。同一个 release 里既有服务器的
`komari-linux-amd64`、又有 agent 的 `komari-agent-linux-amd64` 时，两者都命中后缀 `linux-amd64`，
agent 可能把自己刷成**服务器二进制**。

补丁 0001 因此传入 `selfupdate.Config{Filters: []string{"^komari-agent-"}}`（库在 `updater.go` 里
要求过滤与后缀同时命中，见 `updater.go:53-67`）。0.0.4 发布前用两个临时 release 实测：

- **带过滤**：跳过只有服务器资产的 `0.0.99`，取 `0.0.98` 的 `komari-agent-linux-amd64` → 更新后自证 `I-AM-AGENT`；
- **去掉过滤（对照组）**：直接取 `0.0.99` 的 `komari-linux-amd64` → 更新后自证 `I-AM-SERVER`（节点报废）。

结论：**这条防线不是装饰**，升级上游 agent 代码时必须保留；临时 release/tag 验完已删除。

### 11.2 构建脚本自带的三道门禁

`./scripts/build-agent.sh` 在构建前会 grep 源码并断言：

1. `agent/update/update.go` 里仍有资产过滤 `Filters: []string{"^komari-agent-"}`——删了它
   节点会被刷成服务器二进制（§11.1）；
2. `agent/update/update.go` 的 `Repo` 仍是 `zhemed/komari`——自更新只能指向我们；
3. `install-agent.sh` / `install-agent.ps1` 里 pin 的默认版本等于本次 `KOMARI_VERSION`
   ——防止发版忘了同步（`KOMARI_VERSION` 与仓库默认值不同时按临时构建处理，只告警）。

源码 vendor 化之前这里还有"源码树哈希""补丁回放比对"两道门禁：源码既然已经在仓库里，
这两类问题（pin 漂移、补丁失效）从结构上就不存在了。

### 11.3 如何跟进上游 agent 的修复

源码已在仓库里，**没有 pin 和补丁可以重放**，跟进方式回到最朴素的 git 流程：

1. 看上游要拿的 commit（`git log` 远程仓库，或 `gh api` 查 commit）改了什么；
2. 在本仓库 `agent/` 里手工 apply/改写（不要整棵覆盖——我们内联过改动，直接覆盖会丢）；
3. `./scripts/build-agent.sh --only linux/amd64`，三道门禁必须通过；
4. 涉及自更新/协议的行为改动，先在 `.build/` 里搭临时 release 做对照实验（§11.1 的做法），
   验完把这些提交信息写清楚。

**不要**把上游 agent 整条线跟上来：0.0.4 的教训就是跟到了 1.5.10（§11.5）。

### 11.4 已经装出去的上游 agent 怎么办

我们**无法**远程改变别人机器上已装的上游 agent：它内部指向 `komari-monitor/komari-agent`，
默认每 6 小时自更新到上游最新（当前 `1.5.10`）。能做的只有引导重装（面板里的安装命令已经
指向我们的脚本）。服务器侧**不做版本闸门**，上游 agent 仍能正常上报（协议 v1/v2 未变）。

## 12. 容器镜像

两个镜像都发布在 ghcr.io 下，公开可拉：

| 镜像 | 内容 | 平台 | 构建脚本 |
|---|---|---|---|
| `ghcr.io/zhemed/komari` | 服务器（alpine + 静态二进制） | amd64、arm64 | `scripts/build-server-image.sh --push` |
| `ghcr.io/zhemed/komari-agent` | agent | amd64、arm64、armv7 | `scripts/build-agent-image.sh --push` |

发布时（§3.4 第 7 步）：

```bash
gh auth token | docker login ghcr.io -u zhemed --password-stdin
./scripts/build-server-image.sh --push    # 依赖 dist/komari-linux-{amd64,arm64}，且必须静态链接
./scripts/build-agent-image.sh --push     # 依赖 dist/agent/komari-agent-linux-{amd64,arm64,arm}
```

- 版本号取自 `scripts/version.env`；`:latest` 与 `:<版本>` 一起推。
- 两个脚本在缺产物、或产物不是静态链接时会**直接报错**，不会推一个跑不起来的镜像。
- 服务器镜像里有 `RUN apk add`，跨架构构建需要本机有 QEMU/binfmt：
  `docker run --privileged --rm tonistiigi/binfmt --install arm64`；
  agent 镜像**刻意不含任何 RUN**（只有 COPY），因此不需要模拟器（0.0.4 实测）。
- **包可见性只能手点**：用户级 ghcr 包的可见性无法用 API 改（`PATCH /user/packages/...`
  实测一律 404，连已公开的包也一样），新建的包默认 **private**。要公开得去
  `https://github.com/users/<user>/packages/container/<包名>/settings` → Change visibility → Public。
  当前状态（2026-09-16）：`komari` 与 `komari-agent` 都已人工设为 **public**。
- 验证公开可拉（发版后必跑）：空凭据目录能成功即为公开——
  ```bash
  mkdir -p /tmp/dockerclean
  for t in 0.0.4 latest; do
    DOCKER_CONFIG=/tmp/dockerclean docker manifest inspect "ghcr.io/zhemed/komari:$t" >/dev/null && echo "komari:$t ok"
    DOCKER_CONFIG=/tmp/dockerclean docker manifest inspect "ghcr.io/zhemed/komari-agent:$t" >/dev/null && echo "komari-agent:$t ok"
  done
  ```
  0.0.4 实测：四个 tag 全部匿名可拉，服务器镜像匿名 `docker run` 后 `/install` 200、
  数据落在挂载卷。
- Dockerfile 里带 `org.opencontainers.image.source` 标签，ghcr 包页面会链回本仓库。

### 11.5 为什么停在 1.4.3 同期 agent（而不是上游最新的 agent）

上游 agent 与服务器是**两个独立仓库、两条版本线**。服务器停在 `1.4.3` 血统，agent 也应该停在
**同一天**的代码上，否则节点上跑的是比服务器更新一代的 agent。实测时间线：

| 上游 agent | 时间 | 说明 |
|---|---|---|
| tag `1.2.60` | 2026-07-08 | |
| **`1186aafb`（我们的 pin）** | **2026-08-07** | 服务器/前端 1.4.3 是 2026-08-13，这是它之前最后一个 agent 提交 |
| tag `1.5.0` / `1.5.10` | 2026-09-14 / 09-15 | 1.5 线开始，1.5.10 就是我们**最初**误 pin 的 commit（`9e532e04`） |

停在 1.4.3 同期实际付出的代价（逐条核对过）：

- **不算损失**（我们这条血统根本调不到这些能力）：文件访问 `server/files.go`（1152 行）、
  终端会话重连、文件上传链路修复 —— 服务器与前端都没有这些功能，agent 里有也永远不会被调用；
  反过来，回退后节点上的 root 二进制里少掉整个文件读写实现，攻击面更小。
- **真实损失**：3 个检测修复（AMD GPU 在 `rocm-smi` 缺失时读 sysfs、Android FUSE 磁盘识别、
  macOS `nullfs` 去重）、安装脚本两处改进（无 bash 环境可装、snapshot 通道）。
  需要时按 §7 的 backport 政策单独 cherry-pick，不要整条线跟上去。

### 11.6 为什么会看到 “Remote control is enabled on this device”（0.0.4 时期发生的事）

0.0.4 的 agent 是最初误 pin 的 **1.5.10**，它带一个上游在 `79d8d45 增强安全提醒`（2026-09-14）
新加的提醒：**只要远程控制开着**（上游默认开），agent 就把一段告警写进 `/etc/motd`：

```text
[Komari] Remote control is enabled on this device
127.0.0.1:25774 can execute commands and read or modify files on this device as root.
...
```

这不是入侵痕迹，而是 agent 自己写的"你这台机器上远程控制是开着的"提示，内容说的就是
WebSSH / 远程执行本身的能力。1.4.3 同期的 agent（0.0.5 起）**没有**这段注入逻辑，
只在 Web 终端的 shell 前置脚本里**读**一次 `/etc/motd`（`terminal/terminal_unix.go`）。

排查这类提示的正确姿势：`journalctl -u komari-agent | grep -i "remote control"`，
以及确认 agent 的启动日志里 `Github Repo:` 指向 `zhemed/komari`（不是上游仓库）。

## 13. 上报协议：v1 / v2 与"1.4 时期"的口径

**结论**：我们冻结的 1.4.3 血统里，上报协议是 **v2 主力 + v1 兜底并存**（不是"v1 时代"，
也不是 v2-only）。两条都实现在本仓库内，都要维护。

| 侧 | 实现 | 默认 |
|---|---|---|
| 服务端 | `protocol/v1/report.go`、`protocol/v2/{jsonrpc.go,networktest.go}`；路由 `/api/clients/report`（v1 WS/POST）与 `/api/clients/v2/rpc`（v2 WS/POST），见 `web/router/router.go:68-72` | 两套端点都开着，由 agent 选 |
| agent | `agent/protocol/v1`、`agent/protocol/v2`、`agent/protocol/transport` | `--protocol-version` 默认 **2**（`AGENT_PROTOCOL_VERSION` 可覆盖） |

### 13.1 时间线（上游实测）

- **2026-05-31**：服务端 `protocol/v2` 引入（`e149e8b refactor: remove legacy client APIs and support v2 pings`，
  处于 1.2.0→1.2.3 之间）；同一时期 agent 也加了 v2（`9f088ab feat(protocol): add configurable v2 reporting support`）。
- **1.4.0（2026-08-05）～ 1.4.3（2026-08-13）**：v1 与 v2 全程并存——我们导入的 1.4.3 快照里两套路由、
  两套协议文件都在，可以直接 `git show <snapshot>:web/router/router.go` 核对。
- **2026-08-29**：上游 **agent** 做了 `8fdab5b refactor: 仅保留v2协议，移除v1回退`——注意这在我们
  agent pin（`1186aafb`，2026-08-07）**之后**，属于 agent 的 1.5 线（1.5.0 = 2026-09-14）。
  也就是说"**v1 兜底是 1.4 时期的行为**"，我们冻结 1.4.3 同期 agent 正好把它保留下来。

### 13.2 选择与降级（agent 侧）

```
默认 v2 WebSocket ──连不上/失败达阈值──▶ 降级 v1（直到该连接断开）──▶ 断开后重试 v2
        └─ WS 重试次数用尽 ─▶ v2 HTTP POST 回退（报告 POST + 事件 pull 循环，仍在 v2 端点）
```

两个容易误判的点：

1. **同一时刻只走一条上报通道**；`online (POST session)` 只是 presence 刷新日志，
   v2 的 HTTP 入口也会打（`web/api/client/report_v2.go` 的 `ingestReport(..., 2, true)`），
   **不能**当成"v1 通道在工作"的证据。
2. **面板看不到也不能选协议版本**：服务端只在内存里记 v2 与否（`web/agent/connections.go` 的
   `IsV2Client`），没有暴露给前端。

### 13.3 维护策略（当前事实）

- 两条通道最终都汇入 `web/api/client/ingest.go` 的 `ingestReport` → `metricstore.WriteReport`，
  因此**新逻辑一律做在协议无关层**（指标、流量、数据库、面板 RPC），两条通道同时受益；
  0.0.x 至今的改动（删插件/通知、流量跨重启累计）都属于这一类。
- 改协议层时要同时看两条入口；v1 属**冻结兼容**（服务老 agent 与降级路径），
  不要在没有理由的情况下删它——删掉会让只懂 v1 的节点失联，也让 agent 的兜底失去意义。
- 与协议版本唯一相关的一次修复：v2 报告没有 `uptime` 字段，而"agent 是否重启"的判据依赖它，
  **若**发生 v1↔v2 切换就会误判并把增量清零（详见 §7；该缺陷存在但从未被证实触发过）。

## 14. 面板一键升级（0.0.8 起）

让管理员不必 SSH：在面板"有新版本"弹窗里点一下就完成升级。全部动作在**服务端**执行
（`internal/upgrade`），前端只触发与展示进度。

### 14.1 能做什么

| 能力 | 说明 |
|---|---|
| 升到最新稳定版 | 取 release 列表里版本号最大的非 prerelease |
| 安装指定版本 | 版本列表里选任意 tag（**回滚也走这条路**） |
| 进度与结果 | `admin:upgradeStatus` 返回 `phase`（downloading/verifying/replacing/restarting/failed/completed）；进程重启后从状态文件读"上次结果" |
| 开关与来源 | 系统设置 → "服务器升级"：`server_upgrade_enabled`（默认开）、`server_update_repo`（默认 `zhemed/komari`） |

### 14.2 支持矩阵（写实，不假装都支持）

| 部署形态 | 行为 |
|---|---|
| linux/amd64、linux/arm64 + systemd + 目录可写 | ✅ 下载 → 校验 → 自检 → 备份 + 原子替换 → 退出交 systemd 拉起 |
| 容器 | ⚠️ 不替换二进制（会随容器重建丢失）：仅返回 `docker pull ghcr.io/zhemed/komari:<tag>` 供复制 |
| 无 systemd（前台裸跑） | ⚠️ 仅下载到 `<二进制目录>/upgrades/`（不可写则退到 `$TMPDIR/komari-upgrades`），不退出不替换 |
| darwin / windows / 其它架构 | ❌ 没有发布资产，接口直接拒绝 |

### 14.3 不变量（改这块代码必须保持）

1. **目标仓库只来自服务端配置**，接口不接受请求方传入的 URL（否则就是任意代码执行入口）。
2. **替换前必须自检**：下载物跑 `<新二进制> --help`，输出里要出现 `Komari Monitor <tag>`。
   注意本项目的二进制**没有** `--version` flag（实测退出码 1），别改成它。
3. **校验和先行**：`komari-SHA256SUMS` 缺失或对不上就拒绝安装并删除临时文件；
   旧版本缺该资产时给出"用 install-komari.sh"的可操作提示。
4. **旧二进制不丢**：替换前 `os.Rename` 成 `<二进制>.backup.<旧版本>`；任何一步失败都不覆盖它。
5. **不做自动回滚**（0.0.8 的明确决策）：新二进制若在初始化阶段就崩，它自己没机会执行回滚；
   兜底是 systemd 启动限流（`StartLimitBurst=5`/10s 后进入 failed，不会无限重启）+
   "安装指定版本"手工回退。

### 14.4 升级失败怎么救（照抄命令即可）

```bash
# 1) 看状态与备份路径
journalctl -u komari -n 50 --no-pager | grep upgrade
ls -l /opt/komari/komari.backup.* /opt/komari/.komari-upgrade* 2>/dev/null

# 2) 用备份直接换回去
systemctl stop komari
mv /opt/komari/komari.backup.<旧版本> /opt/komari/komari
systemctl start komari

# 3) 或者重装指定版本（等价于回滚，脚本会做校验）
KOMARI_TAG=<旧版本> bash install-komari.sh
```

### 14.5 安全边界（诚实写明）

- 面板因此获得"下载并执行代码"的能力 → 限制为管理员 RPC、固定仓库、审计日志（`auditlog`）、可开关；
- **SHA256 只保证"下载内容与发布清单一致"**：发布账号/发布流水线被攻破时它不提供保护；
  签名体系（minisign/GPG）留待后续；
- 故意**没有**给 `admin:upgradeServer` 标记 `rpc.MarkSensitive`：那会让每次调用都必须带 2FA 码，
  而面板目前没有该提示流程。若后续接上提示，应把它加入敏感方法。
