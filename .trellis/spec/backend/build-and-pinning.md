# 构建与版本固定（backend）

本文件记录**本 fork 特有**的构建契约。上游文档不覆盖这些约定，改动构建相关文件前必读。

适用范围：`scripts/`、`web/public/`、`install-komari.sh`、`.github/workflows/`。

---

## 1. 不可动摇的契约

### 1.1 默认主题必须存在于 `web/public/defaultTheme/`

`web/public/public.go:18` 是 `//go:embed defaultTheme`。该目录缺失时：

- `go build` 直接编译失败；
- 即使编译通过（例如通过其它方式绕过），`static()` 也会在 `web/public/public.go:130` panic，
  报错文案为 `you may forget to put dist of frontend to web/public/defaultTheme/dist`。

因此 `web/public/defaultTheme/` **是源码的一部分，必须提交进仓库**。
上游默认忽略它（`web/public/.gitignore` 的 `defaultTheme/*`），本 fork 已改为不忽略——
**不要把它改回去**，否则 vendor 产物会被静默排除在提交之外（本仓库首次提交时曾因此漏掉 448 个文件）。

### 1.2 前端版本以 `scripts/frontend-pin.env` 为唯一事实来源

- `KOMARI_WEB_COMMIT` 固定到具体 commit（当前 `4a74e8a8…`，即 komari-web tag 1.4.3）。
- **不要改为分支名或 `latest`**：上游 `.github/actions/build-frontend/action.yml:34` 在普通 tag 下
  就会退化为克隆默认分支，这正是上游 1.4.3 二进制不可复现的原因。
- 依赖安装一律用 `npm ci`（仓库内已提交 `package-lock.json`），**不要用 `npm install`**：
  `^` 浮动解析会破坏可复现性。

### 1.3 产物必须可复现（`FRONTEND_TREE_SHA256`）

`sync-frontend.sh` 对 `web/public/defaultTheme/` 计算规范化目录树哈希
（`tar --sort=name --mtime='@0' … | gzip -n | sha256sum`）并与 `frontend-pin.env` 比对，不一致即失败。

这条门禁能成立，依赖 `scripts/patches/0002-reproducible-build-time.patch`：
上游 `vite.config.ts:53` 取 `new Date().toISOString()`，经 `define.__BUILD_TIME__`
（`vite.config.ts:125-126`）注入产物并显示于 `src/components/Footer.tsx:26`。
该常量每次构建都不同 → 承载它的 chunk 改名 → 所有引用它的 chunk 连锁改名 →
`index.html`/`sw.js` 的预缓存 revision 同步变化。补丁让 `buildTime` 优先读
`SOURCE_DATE_EPOCH`，脚本把它设为 **pin commit 的提交时间**。

- **不要**为了让构建通过而清空或放宽 `FRONTEND_TREE_SHA256`；
  确认变化合理时更新它并在提交信息中说明原因。
- 若哈希再次漂移，先按"归一化后比对全部文件"的方式定位是**内容变化**还是**文件名连锁**，
  不要直接当作噪声忽略。

### 1.4 版本号语义

`CurrentVersion` 由 `scripts/build-komari.sh` 注入（默认 `1.4.3`）。
前端 `AdminPanelBar.tsx` 的 `parseSemver` 只取 `x.y.z` 三段且要求严格递增，
因此 `1.4.3-fix1` 这类 tag **永远不会**被判定为"可更新"。发自有补丁版必须递增 patch 位（`1.4.4`）。

## 2. 构建与验证命令

```bash
./scripts/build-komari.sh                 # 输出 bin/komari（仅需 Go）
./scripts/sync-frontend.sh                # 重新生成前端产物（需网络 + Node）
KOMARI_VERSION=1.4.4 ./scripts/build-komari.sh
```

改构建相关文件后的最小验证：

1. `GOPROXY=off GOFLAGS=-mod=mod ./scripts/build-komari.sh` —— 必须成功（证明 vendor 生效、无需网络）。
2. 启动二进制，日志须含 `Komari Monitor 1.4.3 (hash: <git rev-parse HEAD>)`。
3. `curl` 校验 `/install`、`/assets/*.js`、`/favicon.ico`、`/themes/default/komari-theme.json` 均 200，
   且 JS 字节数与 `web/public/defaultTheme/dist/` 下同名文件一致。
4. `grep -r "komari-monitor/komari/releases" web/public/defaultTheme/` 必须无结果。

## 3. 禁止事项

- 不要 `git push upstream`（`upstream` 仅作只读参考）。
- 不要扩大 `upstream` 的 fetch refspec 让它自动跟踪上游分支：当前只能取到 tag 1.4.3
  是防止误引入 1.5.x 的安全默认。需要上游修复时显式 `git fetch upstream <ref>` 后 cherry-pick。
- 不要把 `install-komari.sh` 的下载路径改回 `releases/latest`（那会安装上游 1.5.x）。
- 不要把 vendored 产物标记为生成物排除出 git——本仓库的可离线构建完全依赖它。
