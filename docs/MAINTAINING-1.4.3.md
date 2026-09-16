# 维护本仓库（komari 1.4.3 分叉）

本仓库是 **komari 1.4.3 的自维护分叉**，目标只有一个：**只维护 1.4.3 这一个版本**，
让它可离线复现、可长期自持，不被动跟随上游 1.5.x。

- 上游后端：<https://github.com/komari-monitor/komari>
- 上游前端：<https://github.com/komari-monitor/komari-web>
- 本仓库：<https://github.com/zhemed/komari>

---

## 1. 版本固定点（Pin）

| 组件 | 固定值 | 说明 |
|---|---|---|
| 后端 | tag `1.4.3` → commit `bf6b45ec3abfc56bba5e9223650a47a72f665371` | 本仓库主干分支 `komari-1.4.3` |
| 前端（默认主题） | tag `1.4.3` → commit `4a74e8a81e2e4b1c3da8ad795f9523151efb6b56` | 记录于 `scripts/frontend-pin.env` |
| 前端产物 | `web/public/defaultTheme/`（已 vendor 进仓库） | 目录树哈希记录于 `scripts/frontend-pin.env` |
| 版本号 | `CurrentVersion=1.4.3` | 构建时由 `scripts/build-komari.sh` 注入 |

**为什么必须自己 pin 前端**：`web/public/public.go:18` 是 `//go:embed defaultTheme`，
主题产物缺失时 `static()` 会在 `public.go:130` 直接 panic——即**本后端仓库单独无法构建**。
上游的 `.github/actions/build-frontend/action.yml:34` 在普通 tag（如 1.4.3）下会退化为克隆
komari-web 的**默认分支**，所以上游 1.4.3 二进制所用的前端本身就不是确定可复现的。

## 2. 工具链

| 组件 | 要求 | 实测 |
|---|---|---|
| Go | ≥ `1.25.0`（以 `go.mod:3` 为准） | `go1.26.6` 通过 |
| gcc | 必需（`CGO_ENABLED=1`，`mattn/go-sqlite3`） | `gcc 11.4.0` 通过 |
| Node / npm | 仅"重新生成前端"时需要 | `node v22.23.2` + `npm 10.9.8` 通过 |

> 上游 `.github/workflows/release.yml:105` 写的是 `go-version: "1.23"`，与 `go.mod` 的
> `1.25.0` 不一致；**以 `go.mod` 为准**。

## 3. 日常操作

### 3.1 构建（只需 Go，不需要网络与 Node）

```bash
./scripts/build-komari.sh          # 输出 bin/komari
```

主题产物已 vendor 在仓库内，因此克隆后即可直接构建。若 `web/public/defaultTheme/`
缺失，脚本会明确报错并指出这正是 `public.go:130` panic 的根因。

### 3.2 重新生成前端产物（需要网络 + Node）

```bash
./scripts/sync-frontend.sh
```

该脚本会：按 pin 的 commit 检出 komari-web → 应用 `scripts/patches/` 下全部补丁 →
`npm ci`（依 `package-lock.json` 锁定，而非 `npm install` 浮动解析）→ `npm run build` →
原子替换 `web/public/defaultTheme/` → 校验目录树哈希与上游地址残留。

哈希不一致时脚本会失败并给出实际值：确认接受后更新 `scripts/frontend-pin.env`
的 `FRONTEND_TREE_SHA256`，并在提交信息里说明原因。

**为什么需要补丁 0002（可复现性）**：上游 `vite.config.ts:53` 取
`new Date().toISOString()` 并经 `define.__BUILD_TIME__`（`vite.config.ts:125-126`）
注入产物，最终由页脚 `src/components/Footer.tsx:26` 显示。这个每次构建都不同的常量会让
承载它的 chunk 内容变化 → 该 chunk 文件名哈希变化 → 所有引用它的 chunk 连锁改名 →
`index.html` / `sw.js` 的预缓存 revision 也跟着变。于是**同一份源码、同一份 lockfile，
两次构建的产物哈希必然不同**。

补丁 0002 让 `buildTime` 优先读取 `SOURCE_DATE_EPOCH`（[reproducible-builds](https://reproducible-builds.org/docs/source-date-epoch/)
标准约定），未设置时保持上游原行为；`sync-frontend.sh` 会把它设为 **pin commit 的提交时间**，
因此产物是确定性的，`FRONTEND_TREE_SHA256` 才是有效门禁。

> 沙箱/受限环境下若 `~/.npm` 不可写，可加 `npm_config_cache=<某可写目录>` 前缀。

### 3.3 发一个自有补丁版

**版本号规则（重要）**：前端 `AdminPanelBar.tsx` 的 `parseSemver` 只取 `x.y.z` 三段，
`isNewerVersion` 要求三段严格递增。因此：

- 想让后台"发现新版本"提示生效，**必须递增 patch 位**（如 `1.4.4`）；
- `1.4.3-fix1` 这类后缀**永远不会**被判为更新（只影响提示，不影响功能）。

```bash
KOMARI_VERSION=1.4.4 ./scripts/build-komari.sh
```

发布时把 `bin/komari` 上传到本仓库的 release，tag 与 `KOMARI_VERSION` 保持一致
（`install-komari.sh` 默认按 `KOMARI_TAG=1.4.3` 拉取）。

## 4. 与上游的解耦点

| 位置 | 改动 | 原因 |
|---|---|---|
| `web/public/defaultTheme/` | vendor 进仓库 | 让后端无网络/无 Node 也可构建（见第 1 节） |
| `web/public/.gitignore` | 取消忽略 `defaultTheme/*` | 上游默认忽略该注入目录，不改则 vendor 产物根本提交不进去 |
| `scripts/patches/0001-update-check-repo.patch` | 更新检查由 `komari-monitor/komari` 改为 `zhemed/komari` | 否则后台持续提示升级到上游 1.5.x |
| `scripts/patches/0002-reproducible-build-time.patch` | 让 `vite.config.ts:53` 的构建时间可被 `SOURCE_DATE_EPOCH` 覆盖 | 上游把**构建时刻**写进产物（由 `src/components/Footer.tsx:26` 显示），导致每次构建的 chunk 哈希连锁变化，产物无法复现 |
| `install-komari.sh` | `REPO` 指向自有仓库；稳定版改用**锁定 tag** 而非 `releases/latest` | 否则一键安装会直接装上 1.5.x |
| `install-komari.sh` | 两处 `curl` 加 `-f`，升级改为先下临时文件再替换 | 404/失败时不再把错误页当二进制写入，也不再截断正在运行的自有二进制 |
| `.gitignore` | 忽略 `/.build/`、`/bin/` | 构建缓存与产物不入库 |

更新检查的目标仓库可用构建期变量覆盖（默认值即本仓库）：

```bash
VITE_KOMARI_UPDATE_REPO=owner/repo ./scripts/sync-frontend.sh
```

## 5. 已知遗留与注意事项

- **安全修复不会自动到来**：上游 1.5.x 之后的修复需要我们自行判断是否 backport。决定采纳时，
  有意识地 `git fetch upstream <ref>` 后 cherry-pick，而不是让上游分支被自动跟踪
  （`upstream` 的 fetch refspec 目前被 `--single-branch --depth 1` 锁在 tag 1.4.3，
  这是防止误引入 1.5.x 的**安全默认**，不要随手改掉）。
- **插件/主题兼容**：`internal/plugin/version.go:33-39` 会用运行版本校验插件声明的 komari
  约束；1.4.3 下，市场上要求更高版本的新插件会被拒绝加载。
- **关于页仍读取上游 README**：`src/pages/admin/about.tsx:19` 拉取上游仓库的 README 用于展示，
  属于信息展示而非升级路径，未做改动。
- **不要 `git push upstream`**：`upstream` 只作为只读参考。

## 6. 回滚

- **回滚前端 vendor**：删除 `web/public/defaultTheme/` 并 `git revert` 对应提交即可；
  注意此时 `go build` 会因 embed 缺失而失败，需重新运行 `sync-frontend.sh` 或恢复该目录。
- **回滚安装脚本/补丁**：`git checkout <commit> -- install-komari.sh scripts/`。
- **本仓库整体**：主干为 `komari-1.4.3`；上游原始状态即该分支的第一个提交（tag 1.4.3）。
