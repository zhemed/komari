# 三网行丢包条去掉斜纹纹理（与延迟条一致）

## Goal

用户 2026-11-12 反馈：生产节点卡片右侧丢包条出现黄/蓝斜纹，与左侧延迟条（实线色块）不一致，
要求「跟左边一样」。

## 根因（已定位）

- 三网行历史条由 `buildTaskHistoryBars` 构建：延迟条调用 `getLatencyToneClass(value, { pattern: false })`
  得到实线色块；丢包条调用 `getLossToneClass(value)`，而该函数没有 `pattern` 选项，loss >3% 时
  固定附加 `ping-signal-pattern-*` 纹理。
- 纹理 CSS 只在 `html.color-vision-friendly` 下生效（`src/styles/main.css`），所以：
  - 标准模式看不到斜纹（本机实例此前一直是标准模式，未复现）；
  - 生产实例 `colorVisionMode=色觉友好` → 丢包 >3% 的色块显示黄/蓝斜纹。

## Requirements

- **R1**：`getLossToneClass` 支持 `options.pattern`，默认保持原行为（聚合条/图表继续带纹理）。
- **R2**：三网行丢包条传 `pattern: false`，与延迟条一致使用实线色块。
- **R3**：不改聚合条、图表、列表视图的既有纹理行为；数值文字色阶不变。
- **R4**：主题版本 bump 到 `1.0.6`，重建 zip + 内置包，发布 `theme-latest`，更新本机与生产主题。

## Acceptance Criteria

- [x] **AC1**：构建产物里延迟/丢包两条构建分支都传 `pattern:!1`（构建后 grep 验证）。
- [x] **AC2**：本机实例（色觉友好模式）与生产实例（色觉友好模式）丢包条均为实线色块（截图）。
- [x] **AC3**：主题 `bun run lint` / `build` 通过；`check-repo.sh --full` 17 项全绿。
- [x] **AC4**：`theme-latest` 更新为 1.0.6（sha256 `1e6c4600…`，匿名下载校验一致）。
- [x] **AC5**：提交带 `[task:loss-bars-solid]`；生产主题已更新、旧主题可回滚。
