# 主题更新与市场源改为自有仓库（komari-src 落地）

## Goal

主仓任务 `10-25-theme-own-repo` 的 komari-src 落地：
把 ink 主题的更新源与主题市场源都改为自有仓库 `zhemed/komari`，去掉对上游/空 URL/版本号压制的依赖。

## Requirements

1. `themes/ink/komari-theme.json`：`version` 1.0.0→1.0.1；`url` → 自有仓库滚动发布资产
   `https://github.com/zhemed/komari/releases/download/theme-latest/ink-theme.zip`。
2. 新增 `deploy/theme-market/v1.json`（catalog schema 1）：收录自维护 ink（1.0.1），
   `download` 同上稳定 URL，`sha256` 与实际资产一致（构建后回填）。
3. `web/api/admin/theme_market.go` 的 `defaultThemeMarketURL` 改为自有 catalog raw URL
   （未来新实例/内核重建默认即自有市场；本实例由配置直接生效）。
4. 文档同步：`ops-docs/VERSION.md`（更新通道/市场源/发版步骤）、`ops-docs/INDEPENDENCE.md`、
   `README.md`、`ops-docs/BUILD.md`（发版清单）。
5. 配合 Lead：构建 zip → rolling prerelease 发布（固定资产名 `ink-theme.zip`）→ catalog 回填 sha256 →
   提交/推送 → 部署 → 核对。
6. 不得重建/重启 Komari 容器（内核常量改动仅源码提交，下次内核构建生效）。

## Constraints

- 滚动 release 使用 tag `theme-latest` 且**必须标记 prerelease**（避免污染内核自升级检测）。
- 不改服务端其他行为；不改 agent；不写站点专属值进仓库。
- 网络写仅 GitHub（push/release）；服务器动作由 Lead 执行。

## Acceptance Criteria

- [ ] manifest `url` 指向自有 rolling release 资产，版本 1.0.1。
- [ ] `deploy/theme-market/v1.json` 存在且 sha256 与发布资产一致。
- [ ] 内核默认市场源常量为自有 catalog；`go build`（或包级编译）通过。
- [ ] 文档四处同步完成；改动提交（任务锚点）并配合 push。
- [ ] 未部署/未重启容器（部署由 Lead 在同任务内执行）。

## Notes

- 设计见主仓 `.trellis/tasks/10-25-theme-own-repo/design.md` 与 `implement.md`。
