# 融入自维护主题与部署脚本（自 komari-own 并入）

## 背景

用户要求以 `zhemed/komari` 作为**唯一仓库**：内核 + 我们自维护的 ink 状态页主题与部署/构建脚本；
此前另建的 `komari-own` 单仓不再作为主仓（后续归档，不删除）。

## 需求

1. 把 `komari-own` 中以下内容并入本仓（提交信息注明来源，保留可追溯性）：
   - `themes/ink/`：自维护主题（manifest 版本 **0.0.1**，自有版本线，与上游主题 0.6.x 无关）；
   - `deploy/`：`Dockerfile.local`、版本化启动脚本、`deploy-theme.sh`（`--dry-run` + sha256 校验 + 备份/回滚）；
   - `docs/` → **`ops-docs/`**（本仓已有内核 `docs/`，按"同名冲突加 ops- 前缀"规则改名）；
   - `build.sh`：一键构建（内核二进制 + 本地镜像 + 主题 zip 与 sha256）。
2. README 增加一节，说明本仓同时承载内核与自维护内容。
3. **内核代码路径不得改动**，保持以后仍能与上游 merge。
4. 构建输出改到 `ops-dist/`，并加入 `.gitignore`。

## 验收标准

- 远端可见新目录：`gh api repos/zhemed/komari/contents/themes/ink` 等只读查询返回内容；
- 本仓根执行 `./build.sh` 能产出 `bin/komari`、本地镜像、`ops-dist/theme/ink-build-<sha>.zip` 与 `.sha256`；
- 主题 zip 内 `komari-theme.json` 版本 = `0.0.1`；
- `git status` 干净；**无 force-push、未改写历史**；未删除任何仓库；
- 线上不变（本轮不部署）。

## 非目标

- 不修改内核功能与内核代码路径；不删除/重命名既有仓库；不动线上服务、DB、nginx。
