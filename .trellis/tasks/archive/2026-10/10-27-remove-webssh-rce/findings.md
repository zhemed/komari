# 实施与验证记录：移除 Web 终端与远程执行（own.5）

## 一、结论

本仓已按用户 2026-10-27 授权完成 L4 移除：**前端 / 服务端 / agent 三层都不再包含 WebSSH（交互终端）与
RCE（远程命令执行）实现**。代码提交 `6fa338b`（主改动）+ `cdaa025`（admin-app 桥接修复）；
服务端镜像 `komari-own:0.0.24-own.5` 已在唯一服务器运行，agent 已替换并在线，
admin-app 已单独更新部署。监控数据持续上报，工作区干净。

## 二、改动清单（按层）

### 2.1 服务端（根 Go 模块）
- 删除：`web/api/terminal/`（4 文件）、`web/rpc/jsonrpc/admin.xtermjs.go`、`admin.xtermjs_test.go`、
  `admin.task.go`、`admin.clipboard.go`、`database/tasks/tasks.go`（ping 记录在 `database/tasks/ping.go` 保留）、
  `database/models/task.go`、`database/models/clipboard.go`、`database/clipboard/`。
- 修改：`web/router/router.go`（terminal 两条路由、xtermjs 设置、admin task 组、client task/result、
  clipboard 组全部删除）、`web/rpc/jsonrpc/admin.system.go`（adminExec 删除）、`web/rpc/jsonrpc/client.go`
  （taskResult 删除）、`database/dbcore/dbcore.go`（Task/TaskResult/Clipboard AutoMigrate 删除）、
  `internal/server/runtime.go`（任务结果清理删除）、`internal/config/settings.go`（XtermjsSettingsKey 删除）、
  `internal/migrations/timestamp.go`（clipboards/task_results 时间列迁移项删除）、
  `protocol/v2/jsonrpc.go`（exec/terminal 方法常量与结构删除）、`web/public/public.go`（/terminal 的
  default-theme SPA 特判删除）、`web/connection/safe_conn.go`（过时注释）、`web/rpc/jsonrpc/README.md`。
- 结果：新二进制内 `strings` 检查 `/api/clients/terminal`、`/api/admin/settings/xtermjs`、`admin:exec`、
  `agent.exec`、`agent.terminal.request` 全部 0 命中。

### 2.2 前端（`frontend/` → 默认主题产物 + ink admin-app）
- 删除：`pages/terminal/`（3 文件）、`pages/admin/exec.tsx`、`pages/admin/settings/xtermjs.tsx`、
  `contexts/TerminalContext.tsx`、`contexts/CommandClipboardContext.tsx`、`hooks/useXtermjsSettings.ts`。
- 修改：`routes.ts`、`config/menuConfig.json`、`components/admin/NodeTable/NodeFunction.tsx`、
  `pages/admin/index.tsx`（含两处安装命令生成器的 disableWebSsh）、`pages/admin/about.tsx`（许可证清单）、
  `vite.config.ts`、`package.json`/`package-lock.json`（@xterm 四依赖删除，离线锁同步）、
  5 个语言包（exec/terminal/settings.xtermjs/admin.nodeTable.disableWebSsh/command_clipboard）。
- 产物：`web/public/defaultTheme/` 重建（417 文件，哈希 `f2cf792721bf…`，两次构建一致）；
  `themes/ink/public/admin-app/` 由更新后的 frontend 重新生成（413 文件，无终端/执行 chunk），
  `sync-komari-admin.ts` 的桥接正则去掉 `/terminal`。

### 2.3 agent
- 删除：`agent/terminal/`（4 文件）、`agent/server/task.go`（远程执行）、exec 相关测试。
- 新增/迁移：`agent/server/ping.go`（原 task.go 中的 ping 实现迁出，`ping_test.go` 同步），capabilities 变为
  `["ping","message","event"]`。
- 修改：`websocket.go`（terminal/exec 分支、v2 分发、establishTerminalConnection、import）、
  `cmd/flags/flag.go`（DisableWebSsh）、`cmd/root.go`（flag 注册与警告调用）、`cmd/warn*.go`
  （WarnKomariRunning 删除，ShowToast 保留）、`protocol/v2/jsonrpc.go`、`go.mod/go.sum`（离线 tidy 移除
  creack/pty、conpty）、`agent/readme.md`、`docs/MAINTAINING.md`（§4/§11.5/§11.6 + 新增 §11.7）。

### 2.4 部署与配置
- `scripts/version.env`：`KOMARI_OWN_SUFFIX=own.5`；新增 `deploy/komari-v0.0.24-own.5-listen-local.sh`。
- 服务端镜像 `komari-own:0.0.24-own.5`；agent linux/amd64 `sha256=3acd08684df66ba3…`。

## 三、部署记录（唯一服务器）

1. 备份：`/root/backups/komari-data-before-own5-20261027044600.tgz`（data 目录）、
   `komari-container-before-own5-20261027044600.json`、`ink-admin-app-own5*.tgz`；
   agent 旧二进制 `agent.bak-own5-20261027044600`；主题旧 admin-app 备份为
   `/root/komari/data/theme/ink/dist/admin-app.bak-own5-*`。
2. 镜像：`docker save | ssh docker load` → `deploy/komari-v0.0.24-own.5-listen-local.sh` 重启面板容器。
3. agent：`scp` 新二进 制 → `systemctl stop` → 备份旧文件 → `install -m 0755` → `systemctl start`。
4. admin-app：打包新目录 → 上传 → 服务器侧 `mv` 旧目录备份 → 解包新目录（含桥接修复二次更新）。

## 四、验证结果（只读命令，服务器本机执行）

| 检查 | 结果 |
|---|---|
| 面板容器 | `komari` Up，image `komari-own:0.0.24-own.5`，RestartCount=0 |
| `/api/version` | version=0.0.24，hash=`6fa338b103d5…`（与提交一致） |
| 节点数据 | `/api/nodes` 返回 1 台节点；`/api/recent/<uuid>` 20 条记录，最新 `updated_at` 距检查时刻约 30s → 数据持续上报 |
| agent | `systemctl is-active komari-agent` = active；日志 `WebSocket connected using v2 protocol`，Basic info 上传成功 |
| 被删路由（POST） | `/api/admin/task/exec` → 404；`/api/clients/task/result` → 404 |
| 被删路由（GET） | 路由不存在，返回 SPA 兜底 HTML（content-type text/html，非 JSON/WS）；二进制中路由字符串 0 命中 |
| 默认后台入口 `/admin` | 返回 200，入口 chunk `entry-index-DSK2d6ec.js`（新构建），HTML 内 `terminal` 0 次 |
| `/admin-app/index.html` | 200，入口 `entry-index-CwfPQqM5.js`，HTML 内 `terminal` 0 次；桥接已无 /terminal |
| 旧终端资源 URL | `/assets/chunk-xtermjs-*`、`/admin-app/assets/chunk-xtermjs-*` 返回 HTML 兜底（非 JS），证明资源已不存在 |
| nginx | active，配置未改动；受云防火墙 hairpin 限制，从服务器自身经公网 IP 访问站点返回 000，未能从内部完成边缘 200 验证（面板 127.0.0.1:25774 正常） |

## 五、残留与未完成

1. **本任务不含 GitHub 推送/发版**：镜像与 agent 仅在本地/服务器生效；`install-agent.sh` 默认版本仍 pin
   0.0.24 的旧 release，新机器重装会拿到**未移除**的旧资产。若要对外生效，需按 INDEPENDENCE §3 走一次
   release（version.env 递增 + tag + releases 资产 + 镜像推送）——用户未授权，未执行。
2. **主题发行包未重建**：`themes/ink` 的 dist/zip 未重建（本次直接替换服务器 dist/admin-app）；
   下次主题发版构建会从更新后的 `public/admin-app` 生成干净包，但**当前仓库内 ink-build 旧 zip 仍含旧
   admin-app**（该 zip 被 .gitignore 忽略，不入库）。
3. 旧数据库表 `tasks` / `task_results` / `clipboards` 保留未删（新代码不再读写）；如需清理需单独授权。
4. `scripts/check-repo.sh` 仍有 2 项既有失败（与本任务无关）：`themes/ink/.env` 入库（既有）、
   `core.hooksPath` 指向全局 hooks、以及历史提交 177 个无任务锚点（上游快照历史）。
5. Windows agent 的 `--show-warning` Toast 保留（与远程控制无关）。
6. `frontend/src/utils/iconHelper.ts` 仍保留通用 `Terminal` 图标映射（图标库残留，无功能入口）。

## 六、回滚手册

1. 镜像回滚：`ssh <host> 'KOMARI_IMAGE=komari-own:0.0.24-own.4 bash -s' < deploy/komari-v0.0.24-own.4-listen-local.sh`
   （服务器上仍保留 own.4 镜像）。
2. agent 回滚：`ssh <host> 'systemctl stop komari-agent && install -m 0755 /opt/komari-agent/agent.bak-own5-20261027044600 /opt/komari-agent/agent && systemctl start komari-agent'`。
3. admin-app 回滚：把 `/root/komari/data/theme/ink/dist/admin-app.bak-own5-*` 移回 `admin-app`。
4. 数据库无 schema 变更（旧表保留），无需回滚数据。
