# 容器形态下暴露可复制的升级命令

## Goal

Docker/无 systemd 的部署不能用一键升级，但**应该能拿到该复制的那条命令**。
0.0.9 的实现里，取回 `pull_command` 的唯一路径是点击升级按钮，而按钮在
`supported=false` 时被隐藏 → 用户只看得到"不支持"的提示，拿不到命令。

用户价值：容器用户不用去猜镜像名与版本标签，直接复制面板给的命令重建容器即可。

## Background（实测事实）

- 服务端能力已具备且**已在真实容器里验证**：`admin:upgradeServer{tag:"0.0.8"}`
  返回 `{"manual":true,"pull_command":"docker pull ghcr.io/zhemed/komari:0.0.8", ...}`，
  `admin:getServerUpgradeSettings` 返回 `supported=false`。
- 前端缺陷位置：`frontend/src/components/admin/AdminPanelBar.tsx`——
  "立即升级到 X"（:619 附近）与每条 release 的"安装此版本"（:558 附近）都以
  `upgradeStatus?.supported` 为前提；`pullCommand` 展示块（:604）因此永远不被赋值。

## Requirements

- R1 `supported=false` 且 `enabled=true` 且有可用版本时，界面上出现**"复制升级命令"**入口
  （至少针对最新版；每条 release 也可给同样的入口，便于拿到指定版本的命令）。
- R2 点击后调用 `admin:upgradeServer`（带目标 tag），从返回的 `manual/pull_command` 展示命令，
  并提供"复制命令"按钮（复用现有展示块与 `startUpgrade`，不新增接口）。
- R3 不得改变"真正支持一键升级"形态的行为（`supported=true` 时仍是升级按钮，无命令按钮）。
- R4 文案走 i18n（5 个语言各加 1 个键），沿用 `upgrade.*` 段。
- R5 前端产物重建并更新 `scripts/frontend-build.env` 的 `FRONTEND_TREE_SHA256`；发 0.0.10。

## Acceptance Criteria

- [ ] 真实容器（0.0.10 镜像）里，管理员在"有新版本"弹窗能看到并复制出
      `docker pull ghcr.io/zhemed/komari:<版本>`；界面上不出现会执行替换的按钮
- [ ] 二进制/systemd 形态行为不变（0.0.10 上仍是一键升级按钮，实测一次降级/回滚）
- [ ] `./scripts/check-repo.sh --full` 全绿；前端哈希与记录一致
- [ ] 0.0.10 已发布（18 资产 + 两个镜像四标签），本机部署与容器实测均有留证

## Out of Scope

- 不做容器内自替换（架构上不可行，仍是设计边界）；
- 不引入 Docker socket 挂载或 watchtower 之类的自动更新；
- 不改服务端接口（能力已具备，仅补前端入口）。
