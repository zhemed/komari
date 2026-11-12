# 实施记录：ink 主题三网行双条版式

## 步骤

- [x] `useNodePingDisplay.ts`：`NodePingRegionRow` 增加 `lossBars`；`buildTaskHistoryBars` 支持
      `latency` / `loss` 两种 metric（丢包条走 `getLossToneClass`）；`lossText` 去掉「丢包 」前缀
      （tooltip 仍为「任务名 丢包 x%」）；新增 `taskRowPlaceholderLossBars`
- [x] `NodeCard.vue`：三网行改为左右两列（左：任务名 + 延迟值 + 延迟历史条；右：丢包值 + 丢包历史条），
      占位行同样双列；交互（tooltip / 整行点击 / aria / 离线 blur）不变
- [x] 构建修复：`src/utils/rpc.ts` 默认基址对齐 `'/api'`；`build.sh` 构建主题时显式
      `VITE_API_BASE=/api`；`.env.example` 注释同步
- [x] 本机实例部署验证；`bun run lint` / `bun run build` / `bun run test:visual` 全通过

## 验证证据（2026-11-12 本机实测）

| 项 | 命令/入口 | 结果 |
|---|---|---|
| Lint | `bun run lint` | 通过（eslint --fix 无残留） |
| 构建 | `bun run build`（干净环境，无 .env） | type-check + vite build 通过；产出 `ink-build-5237564.zip`（3.45 MB） |
| RPC 基址 | 检查构建产物 JS | `const n="/api"; this.baseUrl=t.baseUrl||`${n}/rpc2`` ✅（修复前为 `/rpc2`，面板 RPC 全 404） |
| 视觉回归 | `bun run test:visual` | 8 项全部通过 |
| 本地实例 | 解压新 zip 到 `/opt/komari/data/theme/ink` | 首页 200；节点卡片显示双条版式（截图确认：左延迟条/右丢包条，无「丢包」前缀） |
| 旧主题备份 | `/root/komari-local/theme-backup-20261112_052003` | 可回滚 |
| 仓库自检 | `./scripts/check-repo.sh --full` | 16 项全部通过 |

## 说明

- 主题版本保持 1.0.4（未发布）；生产仍是已发布的旧 1.0.4，**未受影响**。
- 发布新主题时按 docs §3.4.2：`bun run publish`（会 bump 版本）→ 覆盖 `theme-latest` →
  回填 `deploy/theme-market/v1.json` 的版本与 sha256。
- 本机实例与生产的数据/二进制独立；本次只替换了本机实例的主题目录。
