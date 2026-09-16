# 技术设计：锁定并维护 komari 1.4.3 分叉基线

对应任务：`.trellis/tasks/09-16-pin-komari-1.4.3`（需求与验收见同目录 `prd.md`）。

## 1. 架构与边界

发布单元只有一个：**本仓库产出的单个二进制**（后端 + 内嵌默认主题）。

| 角色 | 位置 | 权限 |
|---|---|---|
| 自有仓库 `zhemed/komari` | `origin`（公开） | 读写，日常提交与发布都走这里 |
| 上游后端 `komari-monitor/komari` | `upstream` | 只读参考；refspec 锁在 tag 1.4.3，防止误拉 1.5.x |
| 上游前端 `komari-monitor/komari-web@4a74e8a8` | 构建期临时克隆（不入库） | 只读；不 fork、不长期维护 |

**不 fork 前端**的理由：我们只维护 1.4.3 这一个后端版本，前端唯一需要改动的是更新检查地址（单点）。用"固定 commit + patch 文件"即可，代价远低于维护第二个 fork；且 patch 失效时能立刻发现（`git apply` 失败即报错）。

## 2. 关键决策

- **D1 主题产物 vendor 进仓库**。目录布局直接对齐 `web/public/public.go:18` 的 `//go:embed defaultTheme`，即 `web/public/defaultTheme/{dist/,komari-theme.json,preview.png}`。**后端代码零改动**：`fs.Sub(PublicFS, "defaultTheme")`（`public.go:128`）与 `dist/index.html`（`public.go:204`）的既有契约原样满足。已验证该路径不被 `.gitignore` 忽略（`git check-ignore` 未命中），可直接提交。
- **D2 用 `npm ci` 而非 `npm install`**。komari-web@4a74e8a8 已提交 `package-lock.json`（497,428B），`npm ci` 才真正消除 `^` 浮动依赖；这与 AC5（幂等/哈希一致）是同一件事的两面。
- **D3 前端改动以 patch 文件维护**：`scripts/patches/0001-update-check-repo.patch`。改动点唯一——`src/components/admin/AdminPanelBar.tsx:263` 的硬编码 URL 改为读取 `import.meta.env.VITE_KOMARI_UPDATE_REPO`，**默认值直接写在源码里**（`"zhemed/komari"`），因此 patch 后无需 `.env` 配置即可正确指向自有仓库；同时保留构建期覆盖能力（`VITE_KOMARI_UPDATE_REPO=owner/repo npm run build`）。
  - 已核实前端不存在可复用配置机制：全仓 `import.meta.env` 仅 `src/utils/themeConfiguration.ts:40` 使用 `BASE_URL`。
  - patch 只针对 pin 住的 4a74e8a8 重放；上游前端前进不影响我们。
- **D3b 可复现性修复（实施中发现，新增补丁 0002）**：`scripts/patches/0002-reproducible-build-time.patch` 让 `vite.config.ts:53` 的 `buildTime` 优先读 `SOURCE_DATE_EPOCH`（reproducible-builds 标准约定），未设置时保持上游行为。
  - **发现过程**：AC5 的哈希门禁在首次实现后立刻失败——两次 `sync-frontend.sh`（各自全新克隆 + `npm ci`）产出不同目录树哈希。归因分析（按 8 位内容哈希归一化后比对全部 448 个文件）显示只有 2 处差异：`assets/chunk-_layout-*.js` 的**真实内容**不同（`o="<ISO 构建时刻>"`），以及被动跟随的 `sw.js`（预缓存 revision 引用被改名文件）。即：`vite.config.ts:53` 的 `new Date().toISOString()` 经 `define.__BUILD_TIME__`（`vite.config.ts:125-126`）注入产物，由 `src/components/Footer.tsx:26` 显示；该常量每次都变 → 承载它的 chunk 改名 → 所有引用它的 chunk 连锁改名。
  - **结论**：这是上游构建的确定性缺陷，不是我们的脚本问题。补丁 0002 + 脚本注入 `SOURCE_DATE_EPOCH=<pin commit 提交时间>` 后，vendor 目录树达到**字节级可复现**，AC5 的严格哈希门禁才成立（若改为"语义哈希"豁免则等于放弃该门禁）。
- **D4 版本注入与发版号规则**。ldflags 对齐上游（`release.yml:124`）：`CurrentVersion=1.4.3`、`VersionHash=<后端 commit 全 SHA>`。**规则**：将来发自有补丁版必须递增 patch 位（`1.4.4`…）；`1.4.3-fixN` 在 `AdminPanelBar.tsx` 的 `parseSemver`（只取三段）下恒判为"非更新"，仅导致提示缺失，不影响运行。
- **D5 双 remote 语义**。`origin`=自有仓库（`gh repo create` 后设置）；`upstream` 保持指向上游且**故意不改 refspec**——当前 `--single-branch --depth 1` 让它只能取到 tag 1.4.3，这正是"不会误引入 1.5.x"的安全默认。将来若需读上游历史，显式 `git fetch upstream main` 或改 refspec，属于有意识的运维动作而非默认行为。
- **D6 安装脚本去上游化**。`install-komari.sh:36` 的 `REPO` 改指自有仓库，并处理 `:291` 的 `releases/latest` 语义（自有仓库暂无 release 时该 URL 会 404）：改为优先使用我们发布的 1.4.3 资产，否则明确失败并提示，而不是静默退回上游 `latest`（会把 1.5.x 装上来）。
- **D7 `.gitignore` 增补 `.build/`**。构建缓存与工具链 GOPATH 现为 767MB，落在工作区内，必须忽略，避免误提交。
- **D8 工具链基线写入文档**：Go ≥1.25（`go.mod:3`，实测 1.26.6 通过）、Node 22/23、npm（`ci`）、gcc（cgo，`mattn/go-sqlite3`）。上游 workflow 的 `go-version: "1.23"`（`release.yml:105`）与 `go.mod` 不一致，**以 `go.mod` 为准**并记录该差异。

## 3. 数据流

```
komari-web@4a74e8a8
   └─ git apply scripts/patches/0001-update-check-repo.patch
      └─ npm ci && npm run build (vite)
         └─ dist/ + komari-theme.json + preview.png
            └─ 拷入 web/public/defaultTheme/   ← vendor（提交进仓库）
               └─ go build -ldflags(版本注入)  ← 仅需 Go 工具链
                  └─ 单二进制
                     └─ 运行时由 public.go static() 提供 SPA / 资源 / 主题 JSON
```

运行时路由契约（均已实测）：`/` → 307 `/install`；`/install` → SPA HTML；`/assets/*` 由 SPA 回退读 `dist/assets/*`（`public.go:338-344`）；`/themes/:id/*path`（`public.go:268`）；favicon 优先 `./data/favicon.ico`，否则主题内 `dist/favicon.ico`（`public.go:239-263`）。

## 4. 契约与兼容

- **目录契约**：`web/public/defaultTheme/dist/index.html` 必须存在，否则 `public.go:130` panic（这正是"后端仓库单独不可构建"的根因）。
- **版本契约**：前端 `common:getVersion` ← 后端 `web/rpc/jsonrpc/common.go:461-462` 返回 `CurrentVersion`/`VersionHash`；公开信息亦见 `web/rpc/jsonrpc/public.go:84-85`。
- **插件契约**：`internal/plugin/version.go:33-39` 以 `RunningVersion` 校验插件约束；维持 `1.4.3` 使既有兼容判定不变（同时意味着要求更高版本的新插件仍会被拒）。
- **数据库**：源码同版本，无 schema/migration 变化；沿用 `./data/komari.db`（`web/public/public.go:23` 的 `DataDir`）。
- **更新提示契约**：改为查询 `zhemed/komari` 的 releases；仓库公开是前提（无头浏览器未登录 github.com，浏览器端未认证请求读不到私有仓库 releases）。

## 5. 权衡

| 选择 | 收益 | 代价 |
|---|---|---|
| vendor 前端产物（7MB）进仓库 | 离线可构建、确定性、不依赖 npm/Node | 仓库体积增加；前端更新需重跑再生成脚本 |
| 固定前端 commit + patch（不 fork） | 维护面最小、可审阅、失败即报错 | patch 依赖文件结构稳定（仅对 4a74e8a8 重放，风险可控） |
| 公开自有仓库 | 更新检查真实可用 | 代码公开（上游本为开源项目，实质无新增暴露） |
| 保持 `1.4.3` 版本号不变 | 插件兼容与前端比较行为不变 | 我们自己发补丁版时必须手动递增 patch 位，否则提示不出现 |

## 6. 运维与回滚

- **部署契约不变**：`komari server`、`KOMARI_LISTEN`（默认 `0.0.0.0:25774`）、`./data` 为工作目录。Docker 用法同样不变（`Dockerfile:11-18` 只是把预编译二进制塞进 alpine）。
- **回滚点**：vendor 注入是纯新增文件；`git revert` 单个提交即可回到"仅后端、无主题"状态（注意此时 `go build` 会因 embed 缺失而失败，需重新注入或改用构建期拉取）。
- **推送前检查**：`git status` 干净、无 767MB 缓存、无 token/凭据文件。
- **后续升级路径未封死**：保留 `upstream` remote，将来若决定采纳某个上游修复，可有意识地 cherry-pick，而不是被动跟随。
