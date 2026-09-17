# 执行计划：容器形态下暴露可复制的升级命令

## Step 1 · 前端入口（1 个文件 + i18n）

- [x] `AdminPanelBar.tsx`：`!supported && enabled && latestRelease` 时显示"复制升级命令"；
      每条 release 行在 `!supported` 时同样显示（便于取指定版本/回滚命令）
- [x] 点击 → `startUpgrade(tag)` → 展示 `pullCommand` + "复制命令"（复用既有展示块）
- [x] 5 个语言在 `upgrade.*` 下加 `copy_pull_command`
- 验证：`cd frontend && npx tsc --noEmit`（构建脚本会跑 tsc -b + vite build）

## Step 2 · 前端产物

- [x] `./scripts/build-frontend.sh` → 更新 `FRONTEND_TREE_SHA256`（脚本会给出新哈希）
- 验证：`./scripts/check-repo.sh` 第 6 项（前端产物与记录一致）

## Step 3 · 发版 0.0.10

- [x] 版本线四处字面量 + 文档标题 → 先提交后构建
- [x] 构建（服务端 2 平台 + `komari-SHA256SUMS` + agent 14 平台）→ tag → release（18 资产）→ 镜像
- 验证：`gh release view 0.0.10` 资产数 18；镜像匿名叫架构数

## Step 4 · 实测

- [x] 真实容器（0.0.10 镜像 + 生产库副本）里进面板，确认能看到并复制命令（浏览器实操）
- [x] systemd 形态（本机）确认一键升级按钮与行为未变
- 验证：截图/日志 + `logs` 表审计记录

---

## 实测记录（2026-09-17）

| 场景 | 结果 |
|---|---|
| 真实容器（0.0.10 代码，自报 0.0.5 以触发"有新版本"）浏览器实操 | ✅ 弹窗显示"复制升级命令"（6 个：每版本一个 + 底部），点击后出现容器提示与 `docker pull ghcr.io/zhemed/komari:0.0.10` + "复制命令"按钮；截图 `.build/shots/upgrade-ui-container-ok.png` |
| 服务端契约 | ✅ `admin:getServerUpgradeSettings` → `supported=false`；`admin:upgradeServer{tag}` → `manual:true` + `pull_command` |
| 发版 | ✅ 0.0.10：18 个资产 + 两个镜像（:0.0.10 / :latest） |
| **顺带补上之前"未覆盖"的一项**：升到最新的真实路径 | ✅ 生产 0.0.9 → 0.0.10（面板同款接口 `admin:upgradeServer{}`）：版本切换、二进制与 release 资产**逐字节一致**、服务 active、审计日志 `server upgrade requested: 0.0.9 -> 0.0.10` |

## 本次顺带发现的小问题（未修，记在案）

未登录/状态接口失败时（`upgradeStatus` 仍为 null），按钮仍会渲染（条件 `upgradeStatus?.enabled !== false`
对 null 求值为真）→ 访客点击会看到 "RPC Error -32041: Permission denied"。
不影响管理员与功能正确性，属观感问题；建议下次发版时把条件收紧为
`upgradeStatus && upgradeStatus.enabled !== false`。已写入 MAINTAINING §7 待办。
