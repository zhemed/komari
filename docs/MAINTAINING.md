# 维护本仓库（komari 自维护版 · 当前 0.0.5）

本仓库是 **komari 的自维护分叉**，版本线从 **0.0.1** 起步（当前 **0.0.5**），由我们独立维护。

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
| 项目版本 | `0.0.5`（唯一默认值在 `scripts/version.env`） | 构建时由 `scripts/build-komari.sh` 以 ldflags 注入 `CurrentVersion`；agent 用同一版本号 |
| 后端代码来源 | 上游 tag `1.4.3` → `bf6b45ec3abfc56bba5e9223650a47a72f665371` | 主干分支 `komari-1.4.3`（分支名保留历史来源，不代表版本号） |
| 前端来源 | 上游 tag `1.4.3` → `4a74e8a81e2e4b1c3da8ad795f9523151efb6b56` | 记录于 `scripts/frontend-pin.env` |
| 前端产物 | `web/public/defaultTheme/`（已 vendor 进仓库） | 目录树哈希记录于 `scripts/frontend-pin.env` |
| agent 代码来源 | 上游 agent commit `1186aafb0d41445daac05d8d282d748a897fd495`（2026-08-07，**1.4.3 同期**） | 记录于 `scripts/agent-pin.env`；**不 fork**，且**不跟上游 agent 1.5.x**，见第 11 节 |
| agent 资产 | `komari-agent-<os>-<arch>`（14 个平台） | 与服务器资产**同一个 release**，由 `scripts/build-agent.sh` 构建 |
| agent 镜像 | `ghcr.io/zhemed/komari-agent:<版本>` / `:latest` | 由 `scripts/build-agent-image.sh --push` 推送 |

**为什么必须自己 pin 前端**：`web/public/public.go:18` 是 `//go:embed defaultTheme`，
主题产物缺失时 `static()` 会在 `public.go:130` 直接 panic——即**本后端仓库单独无法构建**。
上游 CI 里的 `.github/actions/build-frontend/action.yml:34`（**该目录已从本仓库移除**）在普通 tag 下
会退化为克隆 komari-web 的**默认分支**，所以上游同 tag 的二进制所用前端本身就不是确定可复现的。

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

## 3. 日常操作

### 3.1 构建（只需 Go，不需要网络与 Node）

```bash
./scripts/build-komari.sh          # 输出 bin/komari
```

主题产物已 vendor 在仓库内，克隆后即可直接构建。若 `web/public/defaultTheme/` 缺失，
脚本会明确报错并指出这正是 `public.go:130` panic 的根因。

### 3.2 重新生成前端产物（需要网络 + Node）

```bash
./scripts/sync-frontend.sh
```

脚本会：按 pin 的 commit 检出 komari-web → 按序应用 `scripts/patches/` 下全部补丁 →
`npm ci`（依 `package-lock.json` 锁定）→ `npm run build` → 原子替换
`web/public/defaultTheme/` → 校验目录树哈希与上游地址残留。

哈希不一致时脚本会失败并给出实际值：确认接受后更新 `scripts/frontend-pin.env`
的 `FRONTEND_TREE_SHA256`，并在提交信息里说明原因。

**为什么需要补丁 0002（可复现性）**：上游 `vite.config.ts:53` 取
`new Date().toISOString()` 并经 `define.__BUILD_TIME__`（`vite.config.ts:125-126`）注入产物，
最终由页脚 `src/components/Footer.tsx:26` 显示。这个每次构建都不同的常量会让承载它的 chunk
改名 → 所有引用它的 chunk 连锁改名 → 索引与 Service Worker 的预缓存 revision 同步变化，
于是同源码同 lockfile 两次构建的哈希必然不同。补丁 0002 让 `buildTime` 优先读取
`SOURCE_DATE_EPOCH`（[reproducible-builds](https://reproducible-builds.org/docs/source-date-epoch/) 标准约定），
脚本把它设为 **pin commit 的提交时间**，产物因此确定可复现。

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
2. 服务器静态产物（需 zig）：
   ```bash
   KOMARI_STATIC=1 KOMARI_OUTPUT=dist/komari-linux-amd64 ./scripts/build-komari.sh
   KOMARI_STATIC=1 KOMARI_GOARCH=arm64 KOMARI_OUTPUT=dist/komari-linux-arm64 ./scripts/build-komari.sh
   ```
3. agent 全平台（14 个，纯 Go）：`./scripts/build-agent.sh`
4. 自检：`./scripts/build-agent.sh --only linux/amd64` 必须通过它自带的两道门禁，
   且 `go build ./... && go vet ./... && go test ./...` 全绿。
5. `git tag <版本> && git push origin <版本>`
6. `gh release create <版本> -R zhemed/komari --title "<版本>" --notes "..." \
      dist/komari-linux-amd64 dist/komari-linux-arm64 dist/agent/komari-agent-*`
7. 推送镜像：`./scripts/build-agent-image.sh --push`

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

- 数据目录跟**工作目录**走（Docker 镜像里是 `/app/data`，systemd 单元里是 `/opt/komari/data`）。
- 二进制的版本标识（`CurrentVersion-VersionHash`）与库中记录不一致时，启动会先把整个 `./data`
  打包到 `data/backup/upgrade-<时间>.zip` 再继续（见 §7 最后一条）。
- 后台可以上传备份并自动重启以应用。

## 4. 与上游的解耦点

| 位置 | 改动 | 原因 |
|---|---|---|
| `web/public/defaultTheme/` | vendor 进仓库 | 让后端无网络/无 Node 也可构建（见第 1 节） |
| `web/public/.gitignore` | 取消忽略 `defaultTheme/*` | 上游默认忽略该注入目录，不改则 vendor 产物根本提交不进去 |
| `scripts/patches/0001-update-check-repo.patch` | 更新检查由上游改为 `zhemed/komari` | 否则后台持续提示升级到上游 1.5.x |
| `scripts/patches/0002-reproducible-build-time.patch` | 构建时间可被 `SOURCE_DATE_EPOCH` 覆盖 | 见 3.2；这是产物可复现的前提 |
| `scripts/patches/0003-drop-plugin-system.patch` | 删除插件页面、路由、菜单与上传 purpose | 见第 5 节 |
| `scripts/patches/0004-drop-https-warn.patch` | 去掉非 HTTPS 的全站红色告警横幅 | 自托管常用内网 HTTP，横幅无操作价值 |
| `scripts/patches/0005-drop-notification-system.patch` | 删除 5 个通知页面、菜单与路由 | 见第 6 节 |
| `scripts/patches/0006-agent-install-source.patch` | 安装命令/镜像/关于页 README/GitHub 按钮指向 `zhemed/komari` | 面板里给出的 agent 安装命令必须装我们的 agent，见第 11 节 |
| `scripts/patches-agent/0001-own-update-line.patch` | agent 自更新指向本仓库 + 资产过滤 + 容器跳过 + 默认关更新 | 见第 11 节（含实测证据） |
| `scripts/patches-agent/0002`、`0003` | 安装脚本改指本仓库、默认目录 `/opt/komari-agent`、默认装 pin 的版本 | 见第 11 节 |
| `install-komari.sh` | 指向自有仓库并锁定 tag；`curl -f` + 先下临时文件再替换 | 不再安装上游 1.5.x；失败时不写入错误页、不截断运行中的二进制 |
| `.gitignore` | 忽略 `/.build/`、`/bin/`；把上游 `komari` 规则锚定为 `/komari` | 后者原为未锚定规则，会连带忽略 `.trellis/workspace/komari/`，使跨会话记忆无法提交 |
| `Dockerfile` | `ARG TARGETOS/TARGETARCH` 给出默认值 `linux/amd64` | 让普通 `docker build`（非 buildx）也能定位上下文里的二进制 |
| `.github/workflows`、`.github/actions`、`.github/ISSUE_TEMPLATE` | 全部删除 | 上游流水线会从前端**默认分支**构建、并向 `ghcr.io/komari-monitor` 推镜像，对本仓库是错误产出 |
| `README.md`、`README_zh-cn.md` | 删除上游版本，改为我们自己的单份 `README.md` | 上游 README 含上游徽章/部署按钮/截图与升级到 1.5.x 的指引 |

更新检查的目标仓库可在构建期覆盖：

```bash
VITE_KOMARI_UPDATE_REPO=owner/repo ./scripts/sync-frontend.sh
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
  `komari-document.pages.dev`。上游文档描述的是 1.4.3/1.5.x 的行为，与本 fork（无插件/无通知）
  有出入。要改得加前端补丁并**重新发版**（前端内嵌在服务器二进制里），暂留。
- **两个 Dockerfile 的基础镜像用 tag 而非 digest**：`alpine:3.21` 会随上游更新而变，
  同一份源码在不同时间构建的镜像不完全可复现；二进制产物本身可复现。
- **已装在别处的上游 agent 无法被我们改写**：见 §11.4。
- **`raw.githubusercontent.com/.../refs/heads/main/<新文件>` 会短暂 404**：面板安装命令用的就是这个
  路径形式。0.0.4 实测：`main` 推上去后该路径仍 404 约 10 分钟（同一文件用 commit SHA 或
  去掉 `refs/heads/` 的形式立刻 200），随后自愈。刚发完版别急着怀疑脚本没推上去。
- **不要 `git push upstream`**：`upstream` 只作为只读参考。
- **版本切换会触发一次升级备份**：`database/dbcore/dbcore.go:233` 的规则是"版本标识不同即备份"，
  标识为 `CurrentVersion-VersionHash`。因此 1.4.3 → 0.0.1 首次启动会 zip 整个 `./data`
  （`dbcore.go:258`）；之后每次以**不同 commit** 重新构建再启动也会再备份一次（上游同样如此，只是上游只在发版时构建）。

## 8. 回滚

- **回滚前端 vendor**：删除 `web/public/defaultTheme/` 并 `git revert` 对应提交即可；
  注意此时 `go build` 会因 embed 缺失而失败，需重新运行 `sync-frontend.sh` 或恢复该目录。
- **回滚安装脚本/补丁**：`git checkout <commit> -- install-komari.sh scripts/`。
- **回滚插件系统移除**：`git revert` 该提交即可恢复插件代码与 1.4.3 版本号（vendored 产物在同一提交内）。
- **回滚 agent 发行线**：删掉 `install-agent.sh` / `install-agent.ps1` / `scripts/patches-agent/` /
  `scripts/build-agent.sh` / `scripts/build-agent-image.sh` / `Dockerfile.agent` / `scripts/agent-pin.env`
  并 `git revert` 补丁 0006（然后重跑 `sync-frontend.sh` 让面板回到上游命令，哈希也要一起回填）。
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

## 11. agent 发行线（0.0.4 起）

**我们不 fork agent**：agent 二进制由本仓库自己构建、作为**本仓库 release 的资产**发布，
所以自有仓库始终只有 `zhemed/komari` 一个（前面说的"三条线"是发布链上的三方，
其中前端与 agent 都只是上游依赖）。

| 环节 | 事实 |
|---|---|
| 源码来源 | 上游 `komari-monitor/komari-agent`，pin 在 commit `1186aafb…`（2026-08-07，**1.4.3 同期**；`scripts/agent-pin.env`） |
| 补丁 | `scripts/patches-agent/0001`（Go 代码）、`0002`/`0003`（安装脚本） |
| 构建 | `./scripts/build-agent.sh`：纯 Go 交叉编译（`CGO_ENABLED=0`，不需要 zig），一次出 **14** 个平台 |
| 资产名 | `komari-agent-<os>-<arch>[.exe]`，与上游一致（安装脚本与自更新都按这个名字找资产） |
| 版本号 | 与我们同一条 `0.0.x` 线（`scripts/version.env`），不沿用上游 agent 的 1.x |
| 自更新目标 | `update.Repo = zhemed/komari`（源码默认值 + 构建期 `-X` 双保险） |
| 自更新默认 | **关闭**；开启用 `--enable-auto-update` 或 `AGENT_ENABLE_AUTO_UPDATE=1`（旧的 `-autoUpdate` 仍表示开启） |
| 安装脚本 | `install-agent.sh` / `install-agent.ps1`（上游脚本 vendor + 补丁）；默认装脚本 pin 的版本，`--install-version latest` 可装最新 |
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

### 11.2 构建脚本自带的两道门禁

`./scripts/build-agent.sh` 除了算出源码树哈希（`AGENT_SOURCE_TREE_SHA256`）外，还：

1. 把补丁回放到上游源码后，与仓库里的 `install-agent.sh` / `install-agent.ps1` **逐字节比对**
   ——防止上游文件变了而仓库成品没重新生成；
2. 检查两个安装脚本里 pin 的默认版本（`default_agent_version` / `$DefaultAgentVersion`）
   等于本次 `KOMARI_VERSION`——防止发版忘了同步（除非 `KOMARI_VERSION` 与仓库默认值不同，
   这种情况按临时构建处理，只告警）。

### 11.3 升级 agent 的上游 pin

1. 改 `scripts/agent-pin.env` 的 `KOMARI_AGENT_COMMIT`（先看上游 diff 里 `update/`、`cmd/`、
   `install.sh` 是否变结构，补丁可能失效）。
2. `./scripts/build-agent.sh --only linux/amd64`：补丁失效会明确报错；哈希变化会打印实际值。
3. 按提示把 `AGENT_SOURCE_TREE_SHA256` 更新为新值，并在提交信息里说明。
4. 安装脚本若也变了：把 `.build/agent-src/install.sh`（补丁已应用）拷回 `install-agent.sh`，
   `install.ps1` 同理，并确认 `default_agent_version` 与版本线一致。
5. 重跑构建脚本，确认两道门禁通过。

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
