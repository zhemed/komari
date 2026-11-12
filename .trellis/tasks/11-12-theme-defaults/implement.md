# 实施记录：主题默认值调整（1.0.7）

## 改动

| 文件 | 改动 |
|---|---|
| `themes/ink/src/stores/app.ts` | `nodeCardSize` 兜底 `comfortable`；`parseColorVisionMode` 兜底 `accessible`；`visitorInfoEnabled` 兜底 `false` |
| `themes/ink/komari-theme.json` | 三处 `default` 同步（comfortable / 色觉友好 / false）；`nodeCardSize` help 文案；版本 `1.0.7` |
| `web/public/builtinThemes/ink.zip` | 重建同步（1.0.7） |
| `README.md` | 主题当前版本 `1.0.7` |

## 验证（2026-11-12）

| 项 | 结果 |
|---|---|
| 构建 | `bun run lint` + `./build.sh --skip-server --skip-install` 通过；zip 3,606,196 B |
| 全新实例 | 临时数据目录 + `0.0.26` 二进制完成安装、换 1.0.7 主题（`theme_configurations` 0 行）：`htmlClass="dark color-vision-friendly"`、`hasVisitor=false` ✓ |
| 主题发布 | `theme-latest` = 1.0.7（sha256 `ecc2de1b…`），匿名下载校验一致，包内 761 文件 ✓ |
| 本机 | 主题 1.0.7；设置对齐为 comfortable / 色觉友好 / 访客关闭（POST 后读回一致） |
| 生产 | 主题 1.0.7（备份 1.0.6 后原子替换）；生产已保存设置本就是 comfortable / 色觉友好 / 访客关闭 |
| 仓库自检 | `check-repo.sh --full` 17 项全绿（内置包 1.0.7 与 manifest 一致） |

## 说明

- 默认值只影响「未保存过设置」的实例；已保存实例（生产/本机）保持其保存值（生产刚好已匹配）。
- 0.0.26 二进制内置主题仍是 1.0.5；下次服务端发版会内置 1.0.7。
- 同一轮另有一个生产 SQLite WAL 事故与修复（见 ops 工作区任务 `wal-repair-2026-11-12`），与本改动无关。
