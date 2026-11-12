# 实施记录：三网行丢包条改实线色块

## 改动

- `themes/ink/src/composables/useNodePingDisplay.ts`
  - `getLossToneClass(loss, options: { pattern?: boolean } = {})`：新增 `pattern` 选项，
    默认 `true` 保持聚合条/图表原行为；`pattern: false` 时返回纯 `bg-signal-*`。
  - `buildTaskHistoryBars` 的丢包分支改为 `getLossToneClass(value, { pattern: false })`，
    与延迟分支一致。
  - 注释同步：三网行延迟条与丢包条都关闭色觉友好纹理；聚合条/图表保留纹理。
- `themes/ink/komari-theme.json`：`1.0.5` → `1.0.6`
- `web/public/builtinThemes/ink.zip`：重建同步（1.0.6）
- `README.md`：主题当前版本 `1.0.6`

## 验证（2026-11-12）

| 项 | 结果 |
|---|---|
| 构建 | `bun run lint` + `./build.sh --skip-server --skip-install` 通过；zip 3,606,145 B |
| 产物代码 | 构建 JS 中两处 `pattern:!1`（延迟 `Ot(P,{pattern:!1})`、丢包 `xt(P,{pattern:!1})`）✓ |
| 本机实例 | 部署 1.0.6 + 切到「色觉友好」模式：三网行左右均为实线色块（截图） |
| 主题发布 | `theme-latest` 更新为 1.0.6（sha256 `1e6c4600…`），匿名下载校验一致 ✓ |
| 生产实例 | 备份 1.0.5 → 替换 1.0.6 → 站点 200、首页资源 `assets/index-C65cih5r.js`、截图确认无斜纹 ✓ |
| 仓库自检 | `check-repo.sh --full` 17 项全绿（含第 17 项内置包 1.0.6 与 manifest 一致） |

## 说明

- 生产实例 `colorVisionMode=色觉友好`，故 1.0.5 下丢包 >3% 才出现斜纹；标准模式不可见。
- 聚合条/图表仍保留色觉友好纹理，只有三网行历史条改为实线。
- 0.0.26 二进制内置主题仍为 1.0.5（发布时版本），下次服务端发版会内置 1.0.6；
  现有实例通过 `theme-latest` 更新即可。
- 生产主题备份：`/root/backups/theme-1.0.5-20261112T133954Z/`（回滚命令已写入生产运行手册）。
