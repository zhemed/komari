# 面板一键升级服务器（含指定版本/回滚）

## Goal

让管理员在面板里直接完成服务器升级，不再需要 SSH 跑 `install-komari.sh`：
在现有"有新版本"弹窗里点一下 → 服务端自己下载对应平台资产、校验 SHA256、原子替换二进制、
退出交由 systemd 重启 → 面板显示新版本号。同时提供"安装指定版本"，既当回滚通道，
也让本功能能被端到端验证（无需等下一个新版本发布）。

## Background（已确认的事实，均有锚点）

| 事实 | 证据 |
|---|---|
| 当前"有新版本"检测在**前端**：浏览器直接请求 GitHub releases，按 `/api/version` 过滤 | `frontend/src/components/admin/AdminPanelBar.tsx:211` |
| 弹窗里的 Github 按钮只打开 `html_url`，无升级能力 | `frontend/src/components/admin/AdminPanelBar.tsx:374` 附近 |
| 检测目标仓库由构建期注入，默认自有仓库 | `frontend/src/components/admin/AdminPanelBar.tsx:43-46`；`scripts/frontend-build.env:12` |
| **服务端没有任何自更新代码**，主模块也没有 `selfupdate` 依赖 | `grep -c selfupdate go.mod` = 0 |
| 仓库内已有先例：agent 自更新（含容器检测、替换后 `os.Exit(42)`） | `agent/update/update.go:144`（`isContainerAgent`）、`:245`（`DoUpdateWorks`）、`:281`（`os.Exit(42)`） |
| systemd unit 由安装脚本生成，`Restart=always`、`User=root` | `install-komari.sh:395-400`；本机 unit 一致 |
| **systemd 自带启动限流**：`StartLimitBurst=5` / `StartLimitIntervalUSec=10s` / `StartLimitAction=none` → 坏二进制最多被拉起 5 次即进入 `failed`，不会无限重启 | `systemctl show komari -p StartLimitBurst -p StartLimitIntervalUSec` |
| 已有回滚通道：安装脚本支持任意 tag | `install-komari.sh:39`（`KOMARI_TAG`） |
| release 目前**只有** agent 的校验和资产 | `gh release view 0.0.7 --json assets` |
| 重启对数据安全：累计流量等状态跨重启存活已验证 | `docs/MAINTAINING.md` §3.6 与本轮实测 |

## Requirements

- R1 面板新增"立即升级"动作：一键从"当前版本"升到最新稳定版。
- R2 面板新增"安装指定版本"：列出 release 列表，可安装任意历史 tag（= 回滚通道）。
- R3 升级动作**必须**在服务端执行（下载/校验/替换），前端只触发与展示进度；
  目标仓库来自服务端配置，**不接受前端传入的 URL**。
- R4 完整性：下载后校验 SHA256；为此发版流程新增服务端校验和资产
  （`komari-SHA256SUMS`，与服务端两个二进制同发布）。签名校验不在 v1 范围（留接口）。
- R5 失败保护（**v1 不做自动回滚**，用户已决策）：替换前先自检新二进制
  （`<新文件> --version` 输出与目标 tag 一致的版本行）；替换时保留 `<二进制>.backup.<旧版本>`；
  任何一步失败都保留旧二进制并返回明确错误。人工兜底路径必须写进文档：
  systemd 启动限流（`StartLimitBurst=5`/10s → 进入 failed，不会无限重启）、
  `KOMARI_TAG=<旧版本> bash install-komari.sh`、以及 backup 文件恢复。
  不做自动回滚的理由：新二进制若在初始化阶段崩溃，它没有机会执行回滚逻辑，覆盖面本就有限，
  却新增"误判回滚"这一失败模式。
- R6 重启：校验通过后原子替换并退出进程，由 systemd（`Restart=always`）拉起；
  重启后版本号可由 `/api/version` 验证。
- R7 守卫部署形态：
  - 容器内（`.dockerenv` 等标记）→ 不替换二进制，改为展示 `docker pull ghcr.io/zhemed/komari:<tag>` 与复制按钮；
  - 无 systemd 监管（前台裸跑）→ 只下载 + 给出手工命令，不自杀式退出；
  - 二进制所在目录不可写或非 root → 明确报错，不做半途替换。
- R8 权限与审计：动作仅管理员可用（沿用 `/api/admin` 权限组）；每次升级/回滚写审计日志
  （发起人、从哪个版本到哪个版本、结果）。
- R9 可关闭：提供配置开关（系统设置里）禁用一键升级，禁用后按钮隐藏/接口拒绝。
- R10 前端：弹窗内展示升级状态（下载中/校验中/重启中/失败原因），重启后轮询
  `/api/version` 自动刷新；新增文案需 5 个语言（zh_CN/zh_TW/en/ja/id）。
- R11 文档与规范：`docs/MAINTAINING.md`（发布流程加校验和资产 + 新章节）、`README.md` 一句、
  `.trellis/spec/backend/` 新增升级契约规范。

## Acceptance Criteria

- [ ] 本机（systemd + 二进制安装）能通过面板从 0.0.7 升到 0.0.8：面板出现按钮 → 点击 → 下载/校验/重启 →
      `/api/version` 变为目标版本；`sha256sum` 与 release 资产一致
- [ ] 升级过程中 `client_traffic_totals`、`metric_rollups` 数据不丢（升级前后累计值单调不减）
- [ ] 用"安装指定版本"把本机从 0.0.8 退回 0.0.7（回滚路径实测）
- [ ] 篡改校验和/下载损坏时：升级被拒绝，旧二进制与运行中的服务不受影响（构造测试）
- [ ] 非管理员调用升级接口 → 403；禁用开关打开后接口与按钮同时不可用
- [ ] 容器检测分支：容器内不替换，返回 pull 命令文案（用 `/.dockerenv` 模拟或单元测试覆盖）
- [ ] 发版流程产出 `komari-SHA256SUMS` 资产，且 `install-komari.sh` 与自升级都校验它
- [ ] `./scripts/check-repo.sh --full` 全绿；新增单测（版本选择、校验失败、平台资产选择、容器分支）
- [ ] 面板升级流程有截图或日志留证（本轮不看"理想"，要实测）

## Out of Scope（v1 明确不做）

- 签名/密钥体系（minisign/GPG）：只留接口位置，文档写明"SHA256 防损坏、不防发布链被篡改"。
- 自动定时升级 / 无人值守批量升级节点：本次只做**手动触发**。
- Docker 内部自替换（架构上不可行，只给 pull 提示）。
- agent 侧任何改动（agent 已有自己的自更新开关，默认关闭）。
- 前端产物独立分发（仍内嵌在服务器二进制里）。

## Key Decisions（已由用户拍板）

1. **功能范围**：升到最新 + 安装指定版本/回滚（后者同时解决"带按钮的版本如何自证"的验证难题）。
2. **校验强度**：SHA256 + 发版新增服务端校验和资产；签名体系留接口不做，文档写明边界
   （防传输损坏与文件错位，不防发布链被篡改）。
3. **失败处理**：v1 不做自动回滚，靠"替换前自检 + 备份 + 指定版本回退 + systemd 启动限流"兜底。
