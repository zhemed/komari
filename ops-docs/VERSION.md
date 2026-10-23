# 版本规则

## 主题（themes/ink）

- 版本号 **由我们自己的 0.0.x 线管理，当前 `0.0.1`**。
- **与上游 ink 的 0.6.x 无关**：上游 `jacob-bytes/komari-theme-ink` 0.6.14 只是我们这份代码的历史起点；
  从 0.0.1 起由本仓库单独发版、单独修复，不再跟随上游语义或版本号。
- **唯一版本源**：`themes/ink/komari-theme.json` 的 `version` 字段。
  > 说明：上游仓库规则明确「不要给 package.json 加顶层 version」（见 `themes/ink/AGENTS.md`），
  > 因此本仓库沿用该规则，**只改 manifest**；不新增 package.json 版本字段以免破坏上游构建/发布契约。
- 发版动作：改 manifest 的 `version` → `./build.sh` → 得到 `ops-dist/theme/ink-build-<sha>.zip` 与 `.sha256` → 部署。

## 服务端（本仓库内核部分）

- 版本**沿用上游 Komari 自身的版本号**，我们的改动以 SemVer 构建元数据标注：
  - 语义版本：`0.0.24+own.1`（`+own.N` = 我们第 N 次自维护构建）
  - Docker 镜像 tag：`komari-own:0.0.24-own.1`（tag 不允许 `+`，故用 `-`）
- 版本源：`scripts/version.env`（发版时按该文件注释同步三处面向用户的字面量）。
- 构建产物不打上游 tag、不推上游 registry；本地 tag 即可，跨机用 `docker save` / `docker load`。

## Git tag 约定（本地打，不 push）

- 主题：`theme-ink/v0.0.1`
- 服务端：`server/v0.0.24-own.1`
- 一个可部署组合 = 主题 tag + 服务端 tag，记录在 `ops-dist/BUILD-INFO.txt`。
