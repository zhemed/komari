# 执行计划：面板一键升级服务器

> 顺序原则：服务端能力（含测试）→ RPC 与权限 → 发版流程配套 → 前端 → 端到端实测 → 文档/规范 → 发布。
> 每一步都给出**验证命令**；没有验证命令的步骤不许打勾。

## Step 1 · 服务端 `internal/upgrade`（新包 + 单测）

- [x] `releases.go`：release 列表、版本比较、按 GOOS/GOARCH 选资产、`komari-SHA256SUMS` 解析
- [x] `download.go`：流式下载 + SHA256 校验（含"校验和不符"与"资产缺失"两条失败路径）
- [x] `install.go`：前置检查（容器/systemd/可写）、`--version` 自检、backup + 原子替换
- [x] `state.go`：状态文件读写（重启后仍可读）
- [x] 单测：`internal/upgrade/*_test.go`
      —— 版本选择（含 prerelease 跳过）、平台资产名、校验和解析、校验失败、容器分支、
         "目录不可写"分支、状态文件往返
- 验证：`go test ./internal/upgrade/... -v`；`go vet ./internal/upgrade/...`

## Step 2 · RPC 与权限、审计、开关

- [x] `web/rpc/jsonrpc/admin.upgrade.go`：`admin:listServerReleases` / `admin:upgradeServer` / `admin:upgradeStatus`
- [x] 仅管理员（沿用 `/api/admin` 权限组）；重复触发返回 `already running`
- [x] 审计日志：发起人、from→to、结果（走 `utils/log`，不新建业务表）
- [x] 设置项 `update_repo`（默认 `zhemed/komari`）与开关 `upgrade_enabled`（默认开）
- 验证：`go test ./web/rpc/jsonrpc/...`；手工 `curl -X POST /api/rpc2`（未登录应为 403）

## Step 3 · 发版流程配套（必须先于 E2E）

- [x] 构建脚本产出 `komari-SHA256SUMS`（两行：amd64/arm64）
- [x] `docs/MAINTAINING.md` §3.4 第 6 步：资产清单/命令加它（17 → 18 个资产）
- [x] 说明"≤0.0.7 的 release 无此资产 → 不支持一键升级"
- 验证：本地构建后 `sha256sum -c dist/komari-SHA256SUMS` 通过

## Step 4 · 前端

- [x] `AdminPanelBar.tsx`：弹窗内按钮组（立即升级 / 安装此版本 / 复制 pull 命令）
- [x] 状态机：`upgradeStatus` 轮询 + 断连即"重启中" + `/api/version` 轮询至版本变化（60s 超时）
- [x] 失败展示：错误原文 + 手工恢复命令
- [x] i18n 5 语言（zh_CN/zh_TW/en/ja/id）
- [x] `./scripts/build-frontend.sh` 重新构建 → 更新 `scripts/frontend-build.env` 的 `FRONTEND_TREE_SHA256`
- 验证：`./scripts/check-repo.sh`（第 6 项前端产物哈希与记录一致）

## Step 5 · 端到端实测（本机部署，数据必须无损）

- [ ] 用面板从当前版本升到 0.0.8（本任务发版后）：按钮 → 下载 → 校验 → 重启 → `/api/version` 变化
- [ ] `sha256sum /opt/komari/komari` 与 release 资产一致
- [ ] 升级前后 `client_traffic_totals` 单调不减、`metric_rollups` 连续（对比截图/查询留证）
- [ ] 用"安装指定版本"把本机退回 0.0.7（回滚实测），再升回 0.0.8
- [ ] 构造失败用例：手工改坏 `komari-SHA256SUMS` 或截断下载 → 升级被拒绝、服务不受影响
- [ ] 容器分支：单元测试覆盖 +（可选）用 `/.dockerenv` 环境变量模拟
- 验证：以上每条命令的输出留档到 `.build/`（截图或日志）

## Step 6 · 文档与规范

- [x] `docs/MAINTAINING.md`：新增"面板一键升级"章节（支持矩阵、失败恢复、安全边界）
- [x] `README.md`：功能点一句（含"容器请 pull 镜像"）
- [x] `.trellis/spec/backend/` 新增 `server-upgrade.md`（RPC 契约、状态机、不变量、测试要求）
- 验证：`grep` 文档锚点；`./scripts/check-repo.sh` 路径校验通过

## Step 7 · 发布 0.0.8

- [x] 版本线四处字面量 + 文档标题 → **先提交后构建**（0.0.8，另修 0.0.9）
- [ ] 构建（服务端 2 平台 + agent 14 平台 + `komari-SHA256SUMS` + `komari-agent-SHA256SUMS`）
- [ ] tag → release（18 个资产）→ 两个镜像（4 标签）
- [ ] 本机从 release 资产升级并复跑 Step 5 的验收点
- 验证：`./scripts/check-repo.sh --full` 全绿；`gh release view 0.0.8` 资产数 18

## 风险文件与回滚点

| 文件 | 风险 | 回滚 |
|---|---|---|
| `install-komari.sh`（unit 不改） | 无需改动；若改动会影响所有安装 | `git revert` |
| `scripts/frontend-build.env`（树哈希） | 前端改动后必须同步，否则 check-repo 失败 | 重跑 `build-frontend.sh` 并更新哈希 |
| `scripts/build-komari.sh` + §3.4 | 资产数变化会影响发版脚本约定 | `git revert`，release 保留 17 资产形态 |
| 升级流程本身 | 最坏情况：服务 failed（有 backup 与 tag 回退） | `KOMARI_TAG=<旧版本> bash install-komari.sh` 或恢复 backup 文件 |

## start 前的最后检查

- [ ] 无阻塞的用户决策（本轮 3 个决策已拍板：范围 / 校验强度 / 失败处理）
- [ ] `implement.jsonl`、`check.jsonl` 各有真实条目（非空模板）
- [ ] 用户已明确批准**规划总结**（本文件与 prd/design 一起展示后）

---

## Step 5 · 端到端实测记录（2026-09-17，真实执行）

| 场景 | 结果 |
|---|---|
| 安装脚本升级到 0.0.8 / 0.0.9 | ✅ 打印"校验和匹配 (komari-linux-amd64)"，部署二进制 sha256 与 release 资产**逐字节一致** |
| 面板降级 0.0.9 → 0.0.7 | ✅ 版本与哈希切到 0.0.7（`fc7d6e8`），二进制 = 0.0.7 资产（`2cd36d38…`），备份 `komari.backup.0.0.9` |
| 面板降级 0.0.9 → 0.0.8 | ✅ 版本 `0.0.8`，二进制 = 0.0.8 资产（`80ec57e9…`），阶段 downloading→completed |
| 数据无损（两次往返） | ✅ 累计流量单调不减（3259→3541→3549 MB），`metric_rollups` 行数不减 |
| 已是最新（空 tag） | ✅ 明确返回"当前已经是最新稳定版"，不假装升级 |
| 缺校验和资产的旧版本 | ✅ 0.0.6 被拒并提示"请用 install-komari.sh 升级" |
| 关闭开关 | ✅ `upgradeStatus.enabled=false`；`upgradeServer` 返回 PermissionDenied（-32041） |
| 非法仓库名 | ✅ 设置接口拒绝"repo 必须形如 owner/repo" |
| 审计日志 | ✅ `logs` 表出现 `server upgrade requested: 0.0.9 -> 0.0.8` 等记录（API Key 调用无 user uuid 属预期） |
| 权限 | ✅ 清空 api_key 后旧 Key 调用被拒（Permission denied） |

**E2E 抓到的真缺陷（0.0.8 → 已修于 0.0.9）**：`Execute` 下载后未补执行位就自检，
`fork/exec … permission denied`，导致 0.0.8 的一键升级必然失败（替换在自检之后，故不破坏线上）。
修复 + 回归测试（注入探针断言"被自检文件可执行"，去掉修复即 FAIL，已验证）。
0.0.8 的公开发布说明已追加更正段。

## 未在本轮 E2E 覆盖（写清原因，不冒充已验证）

- **"升到最新"在真有更新版本时的点击路径**：需要一个比当前运行版本更高的 release 才能观察，
  本轮 0.0.9 已是最新（只能验到 `ErrUpToDate` 分支）。它与"安装指定版本"共用同一段
  `Execute` 代码，差异仅在目标选择（`LatestStable` vs `FindRelease`，均有单测）；
  下一次真实发版时会自然覆盖，届时补记。
- **容器分支**：只有单测覆盖（没有在容器里跑真实容器场景）。
