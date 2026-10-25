# ink 主题 0.0.2 发布：按 ping 任务分行版式（构建+部署）

## Goal

主仓任务 `10-25-theme-release-versioning` 的用户授权：**只发主题 0.0.2**（不新建 Komari 版本）。
本次版式改动为 `komari-src` 提交 `8688155`（NodeCard/useNodePingDisplay/useNodePingStats，+207/−111）：
首页节点卡片按 ping 任务分行显示任务名 + 延迟 + 丢包 + 每行历史点阵。

## Requirements

1. `themes/ink/komari-theme.json` 版本 `0.0.1 → 0.0.2`；同步 `ops-docs/VERSION.md` 的「当前 0.0.1」为 0.0.2。
2. 先提交版本 bump（保证 zip 名/BUILD-INFO 的 git sha 与代码一致），再构建。
3. `./build.sh --skip-server --skip-install` 只构建主题；产物 `ops-dist/theme/ink-build-<sha>.zip` + `.sha256` + BUILD-INFO。
4. 构建后只读核对：sidecar sha256、zip 内 manifest=0.0.2、zip 名前缀 == `git rev-parse --short HEAD`。
5. 本地 annotated tag `theme-ink/v0.0.2`（不 push）。
6. `deploy/deploy-theme.sh --dry-run` 后正式部署到唯一服务器；记录备份 tgz 与回滚命令。
7. 部署后只读核对：远端 manifest=0.0.2、远端入口/资源更新、`/api/version` 仍 `0.0.24+own.4`、
   容器 `Restarts=0` 且 `StartedAt` 未变、站点 `/` 与 `/admin` 返回 200。
8. 模拟验证（可选、只读）：首页新版式（可在部署后用浏览器只读查看，若无需登录态）。

## Constraints

- **不重建 Komari 服务端**：不 bump `version.env`/`KOMARI_OWN_SUFFIX`、不重建镜像/容器、不 `docker save/load`。
- **不 push**（main 与 tag 均本地）。
- 不联网下载（`--skip-install`）；部署仅访问唯一服务器（已获用户当轮授权）。
- 仓库内不写站点专属值；命令/输出中如需主机，用变量或占位符。

## Acceptance Criteria

- [ ] manifest 与 VERSION.md 已更新为 0.0.2，提交带 `[task:]` 锚点。
- [ ] 主题构建成功，zip 内 manifest=0.0.2，sidecar/sha256 一致，BUILD-INFO 记录新 git sha 与 theme 0.0.2。
- [ ] 本地 tag `theme-ink/v0.0.2` 指向 bump 提交。
- [ ] 部署成功（备份+sha256 双校验），线上 manifest=0.0.2，服务端版本/容器未变。
- [ ] 部署后核对全部通过；回滚命令已记录；未 push。

## Notes

- 发布策略依据：`ops-docs/VERSION.md`（主题自有 0.0.x 线、唯一版本源 manifest、发版=改 manifest→build.sh→部署）、
  `ops-docs/BUILD.md`（不可复现、必须固定同一 zip）、`ops-docs/DEPLOY.md` / `ROLLBACK.md`。
- 已知后续（不在本任务）：视觉快照过期待更新；后台「更新/市场」可能覆盖主题的加固另议。
