# 移除 Web 终端与远程执行（L4）

## Goal

按用户 2026-10-27 授权，从本仓（`zhemed/komari`，0.0.24+own 线）彻底移除 Web 终端（WebSSH）与
远程命令执行（RCE）能力：前端页面/入口/依赖、服务端 terminal/exec/task/xterm 子系统、agent 的
terminal 包与 exec 能力；重建服务端镜像与 agent 并部署到唯一服务器上验证。

## Requirements

- 前端：删 `pages/terminal/`、`pages/admin/exec.tsx`、`pages/admin/settings/xtermjs.tsx`、
  `contexts/TerminalContext.tsx`、`hooks/useXtermjsSettings.ts`；清理 `routes.ts`、`config/menuConfig.json`、
  `components/admin/NodeTable/NodeFunction.tsx`（终端按钮 + `disableWebSsh` 选项）、`pages/admin/index.tsx`
  的终端入口、`vite.config.ts` xterm workaround、5 个 locale 的 exec/terminal/settings.xtermjs 词条、
  `@xterm/*` 依赖；离线重建 `web/public/defaultTheme/` 并回填 `FRONTEND_TREE_SHA256`。
- 服务端：删 `web/api/terminal/`、terminal 两条路由、`admin.xtermjs.go` + xterm 设置键、
  `adminExec` 与任务查询、`clientUploadTaskResult`、Task/TaskResult 模型与 CRUD（保留 `database/tasks/ping.go`）、
  `runtime.go` 清理调用、`protocol/v2` exec/terminal 常量与结构。
- agent：删 `agent/terminal/`、`agent/server/task.go`；清理 websocket 分支/能力广告、`DisableWebSsh` flag、
  `agent/protocol/v2` 常量、安装脚本 `--disable-web-ssh`、文档相关表述。
- 版本：`KOMARI_OWN_SUFFIX` → `own.5`，新增 own.5 部署脚本；构建并部署验证。
- 提交：带 `[task:remove-webssh-rce]` 锚点提交（单次用 `.githooks` 覆盖全局 hooks）。

## Constraints

- 只改本仓与唯一服务器；不写站点专属值进仓库；不联网拉依赖（复用 node_modules / go 缓存）。
- 不改 `themes/ink`；不改 `.trellis/` 内文件（由 Lead 负责流程留痕）。
- 部署前备份；不使 `--no-verify`。

## Acceptance Criteria

- [ ] 三层移除完成，`go build ./...`（根模块 + agent 模块）通过，前端 `npm run build` 通过。
- [ ] 全仓残留检索：非文档/非 `.trellis` 处无 `xterm`、`admin:exec`、`MethodAgentExec`、`disable-web-ssh` 引用。
- [ ] 服务端镜像 `komari-own:0.0.24-own.5` 构建成功并部署运行；面板 200、节点在线。
- [ ] 被删接口在服务器上 404；agent 服务 active。
- [ ] 提交完成、工作区干净。
