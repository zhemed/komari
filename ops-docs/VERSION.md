# 版本规则

## 主题（themes/ink）

- 版本号 **由我们自己的 1.x 线管理，当前 `1.0.3`**。
- **与上游 ink 的 0.6.x 无关**：上游 `jacob-bytes/komari-theme-ink` 0.6.14 只是我们这份代码的历史起点；
  从 0.0.1 起由本仓库单独发版、单独修复，不再跟随上游语义或版本号。
- **唯一版本源**：`themes/ink/komari-theme.json` 的 `version` 字段。
  > 说明：上游仓库规则明确「不要给 package.json 加顶层 version」（见 `themes/ink/AGENTS.md`），
  > 因此本仓库沿用该规则，**只改 manifest**；不新增 package.json 版本字段以免破坏上游构建/发布契约。
- **更新通道（自有，2026-10-25 起）**：manifest `url` 指向本仓滚动发布资产
  `https://github.com/zhemed/komari/releases/download/theme-latest/ink-theme.zip`；
  release tag `theme-latest` **必须标 prerelease**，每次主题发版覆盖上传 `ink-theme.zip` + `.sha256`。
  面板「主题→更新」从我们自己的仓库下载最新主题包（不再引用上游）。
  > prerelease 是硬约束：Komari 自升级（`internal/upgrade.StableReleases` 与前端更新提示）只认非 prerelease，
  > 主题通道若标成正式 release 会被误判为内核新版本。
- **发行方式**：改 manifest 版本 → 提交 → `./build.sh --skip-server --skip-install` →
  覆盖滚动 release（资产名固定 `ink-theme.zip`）→ 回填 `deploy/theme-market/v1.json` 的 sha256（提交）→ 部署。
- **主题市场（自有目录）**：`deploy/theme-market/v1.json` 是本仓自有 catalog；实例配置
  `configs.theme_market_sources` 指向其 raw URL，内核默认值 `web/api/admin/theme_market.go` 的
  `defaultThemeMarketURL` 同为此地址；官方目录不再使用。

## 服务端（本仓库内核部分）

- 版本**沿用上游 Komari 自身的版本号**，我们的改动以 SemVer 构建元数据标注：
  - 语义版本：`0.0.24+own.1`（`+own.N` = 我们第 N 次自维护构建）
  - Docker 镜像 tag：`komari-own:0.0.24-own.1`（tag 不允许 `+`，故用 `-`）
- 版本源：`scripts/version.env`（发版时按该文件注释同步三处面向用户的字面量）。
- 构建产物不打上游 tag、不推上游 registry；本地 tag 即可，跨机用 `docker save` / `docker load`。

## Git tag 约定（本地打并 push 到 origin，2026-10-25 起）

- 主题：`theme-ink/v1.0.3`
- 服务端：`server/v0.0.24-own.1`
- 一个可部署组合 = 主题 tag + 服务端 tag，记录在 `ops-dist/BUILD-INFO.txt`。

## 权威来源（我们自己的仓库）

- **本仓库 `zhemed/komari` 是我们自维护的权威来源**：内核 + 主题 + 部署/构建脚本 + 文档，全部在本仓；
  不再有第二个"自有单仓"（早先的 `komari-own` 已删除归档，本地目录仅作历史参考，其全历史已存为 bundle）。
- **主题不再跟随任何上游**：上游 `komari-theme-ink` 的 0.6.x 只是这份代码的历史起点；
  主题自身的提交历史已用 `git subtree` 并入本仓 `themes/ink/`（179 个提交），
  查法：`git log --oneline -- themes/ink`，或回溯导入合并提交的第二父：
  `git log --oneline "$(git log --format=%H --grep=\"Add 'themes/ink/'\" -1)^2"`。
- 构建与部署**一律使用本仓脚本**（`./build.sh`、`deploy/`），不要引用仓外路径。

## 可选：同步 Komari 内核上游（手动、不自动）

内核沿用上游版本号（当前 `0.0.24`），我方改动以语义版本元数据 `+own.N` 标注。
**当前内核代码路径 0 处我方改动**，与上游同步的冲突风险极低。需要时按下面步骤手动做：

```bash
git remote add upstream <上游 Komari 仓库>        # 只需一次
git fetch upstream --tags
git switch -c sync/upstream-$(date -u +%Y%m%d)   # 临时分支，别直接改 main
git merge upstream/main                          # 或 rebase，按当时情况选
./build.sh                                       # 重建内核镜像与主题 zip
# 按 ops-docs/DEPLOY.md 部署并在临时环境验证；确认无误后再并入 main
```

注意：**主题与部署脚本属于我们自己的改动范围**，与上游内核同步时不需要、也不应该被覆盖。
