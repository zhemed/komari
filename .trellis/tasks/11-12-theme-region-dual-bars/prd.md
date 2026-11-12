# ink 主题三网行改为双条版式（延迟条 + 丢包条并排）

## Goal

按用户参考图（2026-11-12 第二张截图）把节点卡片「三网」行改为**双条版式**：
每行左列 = 任务名 + 延迟值 + 延迟历史条；右列 = 丢包值 + 丢包历史条。
行内不再显示「丢包 」文字前缀（口径放 tooltip），与参考图一致。

## 背景与已确认事实

- 用户确认：参考图就是目标版式（2026-11-12 提问确认）。
- 现状：`themes/ink/src/components/NodeCard.vue` 每行 = 任务名 + 延迟 + 「丢包 x%」+ **单条**延迟历史；
  参考图为左右两列、两组历史条。
- 数据已具备：`NodePingHistoryPoint` 每个点同时含 `latency` 与 `loss`，无需改数据层。
- 构建坑（本次实测）：干净克隆直接 `bun run build` 时 `VITE_API_BASE` 未设置，`rpc.ts` 退回
  `/rpc2`（服务端路由是 `/api/rpc2`）→ 面板全量 RPC 404。发布机器此前依赖未入库的 `.env`。

## Requirements

- **R1 双列布局**：左列延迟（label + 延迟值 + 延迟历史条），右列丢包（丢包值 + 丢包历史条）；
  两列等宽、20 格对齐、沿用实线色块。
- **R2 色阶**：延迟条用 `getLatencyToneClass(..., { pattern: false })`；丢包条用 `getLossToneClass`；
  `null` 一律灰色占位。
- **R3 交互保持**：每格 tooltip（时间 + ms / %）、整行点击打开监测弹窗、aria-label、离线 blur 不变。
- **R4 占位行**：无任务数据时同样双列（延迟/丢包各自空点阵 + 「加载中 / --」文本）。
- **R5 构建可复现**：`rpc.ts` 默认基址对齐 `api.ts`（`'/api'`）；`build.sh` 构建主题时显式注入
  `VITE_API_BASE=/api`，不再依赖未入库的 `.env`。
- **R6 回归**：主题 `bun run lint` / `bun run build` / `bun run test:visual` 通过；
  仓库 `check-repo.sh` 快速检查通过。
- **R7 不发布**：不 bump 主题版本、不发 `theme-latest`、不动生产；仅本机实例验证。

## Acceptance Criteria

- [x] **AC1**：主题构建通过（type-check + vite build + zip），产物 RPC 基址为 `/api`。
- [x] **AC2**：本地实例部署新主题后，节点卡片显示双条版式（截图确认：左右两组历史条、无「丢包」前缀）。
- [x] **AC3**：`bun run test:visual` 8 项全部通过。
- [x] **AC4**：`./scripts/check-repo.sh` 快速检查全绿。
- [x] **AC5**：改动只含主题源码 + `build.sh` + `.env.example` + 任务记录；提交带
      `[task:theme-region-dual-bars]`。

## 非目标

- 不改数据层 / RPC 接口 / 后台 ping 任务配置。
- 不改列表视图（`NodePingListCell`）的上下双条样式。
- 不发布主题（版本仍 1.0.4；发布时按 docs §3.4.2 走 `bun run publish` + 市场目录）。
