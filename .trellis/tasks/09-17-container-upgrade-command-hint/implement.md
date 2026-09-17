# 执行计划：容器形态下暴露可复制的升级命令

## Step 1 · 前端入口（1 个文件 + i18n）

- [ ] `AdminPanelBar.tsx`：`!supported && enabled && latestRelease` 时显示"复制升级命令"；
      每条 release 行在 `!supported` 时同样显示（便于取指定版本/回滚命令）
- [ ] 点击 → `startUpgrade(tag)` → 展示 `pullCommand` + "复制命令"（复用既有展示块）
- [ ] 5 个语言在 `upgrade.*` 下加 `copy_pull_command`
- 验证：`cd frontend && npx tsc --noEmit`（构建脚本会跑 tsc -b + vite build）

## Step 2 · 前端产物

- [ ] `./scripts/build-frontend.sh` → 更新 `FRONTEND_TREE_SHA256`（脚本会给出新哈希）
- 验证：`./scripts/check-repo.sh` 第 6 项（前端产物与记录一致）

## Step 3 · 发版 0.0.10

- [ ] 版本线四处字面量 + 文档标题 → 先提交后构建
- [ ] 构建（服务端 2 平台 + `komari-SHA256SUMS` + agent 14 平台）→ tag → release（18 资产）→ 镜像
- 验证：`gh release view 0.0.10` 资产数 18；镜像匿名叫架构数

## Step 4 · 实测

- [ ] 真实容器（0.0.10 镜像 + 生产库副本）里进面板，确认能看到并复制命令（浏览器实操）
- [ ] systemd 形态（本机）确认一键升级按钮与行为未变
- 验证：截图/日志 + `logs` 表审计记录
