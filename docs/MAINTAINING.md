# 维护本仓库（komari 0.0.1 自有基线）

本仓库是 **komari 的自维护分叉**，版本线从 **0.0.1** 开始，由我们独立维护。

- 上游后端：<https://github.com/komari-monitor/komari>
- 上游前端：<https://github.com/komari-monitor/komari-web>
- 本仓库：<https://github.com/zhemed/komari>

> **代码来源**：0.0.1 由上游 `komari@1.4.3`（commit `bf6b45ec…`）+ `komari-web@1.4.3`（commit `4a74e8a8…`）派生。
> 1.4.3 只是**历史来源**，不是我们对外声明的版本；对外一律是 0.0.x。

---

## 1. 版本与固定点

| 组件 | 固定值 | 说明 |
|---|---|---|
| 项目版本 | `0.0.1` | 构建时由 `scripts/build-komari.sh` 以 ldflags 注入 `CurrentVersion` |
| 后端代码来源 | 上游 tag `1.4.3` → `bf6b45ec3abfc56bba5e9223650a47a72f665371` | 主干分支 `komari-1.4.3`（分支名保留历史来源，不代表版本号） |
| 前端来源 | 上游 tag `1.4.3` → `4a74e8a81e2e4b1c3da8ad795f9523151efb6b56` | 记录于 `scripts/frontend-pin.env` |
| 前端产物 | `web/public/defaultTheme/`（已 vendor 进仓库） | 目录树哈希记录于 `scripts/frontend-pin.env` |

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
| Node / npm | 仅"重新生成前端"时需要 | `node v22.23.2` + `npm 10.9.8` 通过 |

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

1. `KOMARI_VERSION=0.0.2 KOMARI_STATIC=1 ./scripts/build-komari.sh`
2. 资产命名要与 `install-komari.sh` 的期望一致：`mv bin/komari komari-linux-amd64`
3. `git tag 0.0.2 && git push origin 0.0.2`
4. `gh release create 0.0.2 --title 0.0.2 --notes "..." komari-linux-amd64 [komari-linux-arm64]`

tag 与 `KOMARI_VERSION` 保持一致（`install-komari.sh` 默认按 `KOMARI_TAG=0.0.1` 拉取）。
**本仓库没有 CI**（上游流水线已移除），发布必须手动执行以上步骤。

> **`gh` 陷阱（0.0.1 发布时实际踩到）**：本仓库有两个 remote（`origin`=自有、`upstream`=只读参考），
> `gh release create` 可能把仓库解析成 `upstream`，报
> `tag 0.0.1 exists locally but has not been pushed to komari-monitor/komari`。
> 发布时给 `gh` 显式加 `-R zhemed/komari`。

## 4. 与上游的解耦点

| 位置 | 改动 | 原因 |
|---|---|---|
| `web/public/defaultTheme/` | vendor 进仓库 | 让后端无网络/无 Node 也可构建（见第 1 节） |
| `web/public/.gitignore` | 取消忽略 `defaultTheme/*` | 上游默认忽略该注入目录，不改则 vendor 产物根本提交不进去 |
| `scripts/patches/0001-update-check-repo.patch` | 更新检查由上游改为 `zhemed/komari` | 否则后台持续提示升级到上游 1.5.x |
| `scripts/patches/0002-reproducible-build-time.patch` | 构建时间可被 `SOURCE_DATE_EPOCH` 覆盖 | 见 3.2；这是产物可复现的前提 |
| `scripts/patches/0003-drop-plugin-system.patch` | 删除插件页面、路由、菜单与上传 purpose | 见第 5 节 |
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
- `pkg/jsruntime/` **保留**：它不是插件专用——`utils/messageSender/javascript` 依赖它。
- 主题系统与主题市场**保留**，且主题本就没有版本门禁，不受版本号影响。
- 流量报告：内置实现**仍可用**（上游计划在 1.5.0 移除，我们停留在 1.4.3 基线，故不受影响）；
  原先指向插件市场的引导提示已删除。

## 6. 已知遗留与注意事项

- **安全修复不会自动到来**：上游 1.5.x 之后的修复需我们自行判断是否 backport。
  决定采纳时有意识地 `git fetch upstream <ref>` 后 cherry-pick——`upstream` 的 fetch refspec
  目前被锁在 tag 1.4.3，这是防止误引入 1.5.x 的**安全默认**，不要随手改掉。
- **关于页仍读取上游 README**：`src/pages/admin/about.tsx:19` 拉取上游仓库 README 用于展示，
  属信息展示而非升级路径，未做改动。
- **不要 `git push upstream`**：`upstream` 只作为只读参考。
- **版本切换会触发一次升级备份**：`database/dbcore/dbcore.go:233` 的规则是"版本标识不同即备份"，
  标识为 `CurrentVersion-VersionHash`。因此 1.4.3 → 0.0.1 首次启动会 zip 整个 `./data`
  （`dbcore.go:258`）；之后每次以**不同 commit** 重新构建再启动也会再备份一次（上游同样如此，只是上游只在发版时构建）。

## 7. 回滚

- **回滚前端 vendor**：删除 `web/public/defaultTheme/` 并 `git revert` 对应提交即可；
  注意此时 `go build` 会因 embed 缺失而失败，需重新运行 `sync-frontend.sh` 或恢复该目录。
- **回滚安装脚本/补丁**：`git checkout <commit> -- install-komari.sh scripts/`。
- **回滚插件系统移除**：`git revert` 该提交即可恢复插件代码与 1.4.3 版本号（vendored 产物在同一提交内）。
- **部署侧回滚**：升级前自动生成的 `data/backup/upgrade-*.zip` 即为回滚素材。

## 8. 克隆与推送

- **只构建不需要历史**：`git clone --depth 1 <本仓库>` 后即可 `./scripts/build-komari.sh`。
- **推送需要完整历史**：若以 `--depth 1` 克隆后直接 `git push`，会因缺少被引用对象报
  `remote unpack failed: index-pack failed`。修复方式（本仓库实际使用）：

  ```bash
  git fetch --unshallow --refetch upstream "+refs/tags/1.4.3:refs/tags/1.4.3"
  ```

  注意仅 `git fetch --unshallow` 可能**无效**（refspec 只覆盖那个 tag 且 tip 未变时不会加深），
  必须显式给出 refspec 并配合 `--refetch`。

## 9. 提交历史与我们自己的仓库

- **历史已于 2026-09-16 重写**：上游 835 个提交不再出现在历史中。上游代码以**单个快照根提交**
  引入（`chore: import komari 1.4.3 (upstream bf6b45ec) as our 0.0.1 code snapshot`），
  我们自己的提交重挂在该根之上，提交粒度保留。
- 重写前的最后一次提交保留在**本地分支 `backup/pre-rewrite`**（未推送）上，作为回滚锚点。
  因此旧 journal 里记录的 hash（如 `18305a9`、`77f36da`）在重写后**已不可达**，只具历史意义；
  需要时以 `backup/pre-rewrite` 为准。
- 「代码来源」由 `LICENSE` / `NOTICE` / `README.md` / 根提交信息承载，而不再由逐行历史承载。
- 需要取回上游历史做 backport 时：`git fetch upstream --tags`（`upstream` remote 保留）。
- 本地曾存在的 68 个上游 tag 已删除，避免上游对象长期驻留；本仓库只推送自己的 tag。
