# 主题 1.0.2：作者改自有 + 预览图更新 + catalog 修正

## Goal

用户看市场截图后要求把元信息改成我们自己的：manifest `author` → `zhemed`、版本 1.0.2；
用当前版式重新生成 `themes/ink/docs/preview.png`；修正自有 catalog 的 preview 路径（原路径不存在）
与版本/sha；配合 Lead 构建、发布滚动 release、部署主题、push。

## Requirements

1. `themes/ink/komari-theme.json`：`author` → `zhemed`；`version` → `1.0.2`（保留其他字段）。
2. 重新生成 `themes/ink/docs/preview.png`：当前 UI（按 ping 任务分行的卡片）截图，16:9、mock 数据、无真实站点值。
3. `deploy/theme-market/v1.json`：版本 1.0.2；`preview` →
   `https://raw.githubusercontent.com/zhemed/komari/main/themes/ink/docs/preview.png`；
   `updated_at` 更新；sha256 待构建后由 Lead 回填（提交 B）。
4. 文档：`ops-docs/VERSION.md` 与 `README.md` 的主题当前版本号改 1.0.2。
5. 不重建/重启 Komari 容器；不改服务端行为。

## Acceptance Criteria

- [ ] manifest author/version 已改；`docs/preview.png` 为当前 UI 且被构建打包。
- [ ] catalog 版本 1.0.2、preview 路径正确（公网 200）、sha256 与滚动资产一致（Lead 回填）。
- [ ] 变更提交（任务锚点），配合 push 与部署。
- [ ] 无临时文件残留、工作树干净。

## Notes

- 打包规则：`themes/ink/vite.config.ts` 从 `docs/preview.png` 打包为 `preview.png`。
- 主仓任务：`.trellis/tasks/10-25-theme-owner-metadata/`。
