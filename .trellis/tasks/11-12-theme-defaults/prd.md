# 主题默认值调整：comfortable / 色觉友好 / 访客信息关闭

## Goal

用户 2026-11-12 定稿主题默认值（新装/未保存设置的实例）：
- `nodeCardSize` 默认 `compact` → **`comfortable`**
- `colorVisionMode` 默认 `标准` → **`色觉友好`**
- `visitorInfoEnabled` 默认 `true` → **`false`**

## 实现

- **运行时兜底**（`themes/ink/src/stores/app.ts`）：
  - `nodeCardSize` 非法/缺失时返回 `comfortable`
  - `parseColorVisionMode` 缺失时返回 `accessible`（含别名兜底）
  - `visitorInfoEnabled` 的 `readBooleanSetting` 默认值改为 `false`
- **管理端表单默认值**（`themes/ink/komari-theme.json` 的 `configuration.data`）：
  三处 `default` 同步；`nodeCardSize` help 文案改为「默认 comfortable」。
- 主题版本 `1.0.6` → **`1.0.7`**；重建 zip + 内置包；README 同步。

## Requirements / 边界

- 已保存设置的实例**行为不变**（`theme_configurations` 优先于默认值）。
- 聚合条/图表的色觉友好纹理不受影响（仍保留）。
- 主题 1.0.7 发布到 `theme-latest`；本机与生产主题更新到 1.0.7。
- 0.0.26 二进制内置主题仍 1.0.5，下次服务端发版内置 1.0.7。

## Acceptance Criteria

- [x] **AC1**：manifest 三处默认值与代码兜底一致（comfortable / 色觉友好 / 访客关闭）。
- [x] **AC2**：全新实例（无 `theme_configurations` 行）实测：`html.color-vision-friendly` 存在、
      页面无「Your IP」访客条。
- [x] **AC3**：`theme-latest` = 1.0.7（sha256 `ecc2de1b…`，匿名下载校验一致）。
- [x] **AC4**：生产主题更新到 1.0.7（生产已保存设置本就匹配新默认值）；本机主题 1.0.7 且设置对齐。
- [x] **AC5**：`bun run lint` / `build` 通过；`check-repo.sh --full` 17 项全绿；提交带 `[task:theme-defaults]`。
