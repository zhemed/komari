# ink 1.0.3：三网行历史条去纹理（实线），保留其它色觉辅助

## Goal

主仓任务 `10-26-ink-solid-bars-font` 的落地：用户要求三网行历史条改为**实线**。
仅把「三网行（per-task latency）历史条」的纹理去掉；聚合/列表/图表仍保留
`ping-signal-pattern-*` 色觉辅助。manifest 版本 1.0.3，文档同步；配合 Lead 构建/发布/部署/push。

## Requirements

1. `themes/ink/src/composables/useNodePingDisplay.ts`：
   - `getLatencyToneClass(latency, options?: { pattern?: boolean })`：默认保持现状（带 pattern）；
     `pattern: false` 时只返回 `bg-signal-N`（不带 `ping-signal-pattern-*`）。
   - `buildTaskHistoryBars()` 调 `getLatencyToneClass(value, { pattern: false })`。
   - 其它调用点（`buildPingBars` 聚合、列表视图等）**不改**，继续保留纹理。
2. 不改字体、不改变色阶阈值、不改变 tooltip/aria、不改变空柱占位色。
3. Lead 负责 manifest 1.0.3 与文档（VERSION/README），worker 只改上述 1 个源码文件。
4. 验证：
   - `bun run type-check`、`bunx eslint <file>`（不加 --fix）、`bun run build-only` 通过；
   - 用既有 Playwright 临时配置（chromium-1243 executablePath）在 `colorVisionFriendly` 模式下断言：
     三网行 20 格背景 `background-image === 'none'`（实线）；列表/聚合条仍含 repeating-linear-gradient（保留）；
   - 不改测试 fixture/snapshot；临时文件跑完删除。
5. 不 commit/push/部署（Lead 负责）。

## Constraints

- 只改 1 个源码文件；不改其它组件/CSS/快照。
- 不联网安装依赖；不重启服务。

## Acceptance Criteria

- [ ] `useNodePingDisplay.ts` 改动完成，type-check/eslint/build 通过。
- [ ] 色觉友好模式下三网行历史条为实线（background-image none）；其它条仍有纹理。
- [ ] 无临时文件残留；git status 仅显示该源码文件（+任务目录）。
- [ ] 五小节总结回报 Lead。

## Notes

- 视觉核对脚本可参考此前 `prototype.spec.ts`/临时 config 模式（Chromium-1243）。
- 设计依据：主仓任务 `10-26-ink-solid-bars-font/prd.md`。
