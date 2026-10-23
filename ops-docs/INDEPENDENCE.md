# 独立维护声明（内核与主题均脱离上游）

> 决策：本仓 `zhemed/komari` 是**我们自维护的唯一权威来源**；**内核与主题均不再跟随任何上游**。
> 本文是这条决策的规则文本，与 `ops-docs/VERSION.md`、`ops-docs/BUILD.md`、`ops-docs/DEPLOY.md`、`ops-docs/ROLLBACK.md` 配套。

## 1. 决策声明

- **唯一权威来源**：内核代码 + 主题（`themes/ink`，自有版本线 `0.0.x`）+ 部署与构建脚本（`deploy/`、`build.sh`）+
  运维文档（`ops-docs/`）全部在本仓；不再存在第二个"自有仓库"。
- **不跟随上游**：不配置常驻的上游 remote，不做定时同步，不自动接受上游任何变更。
- **后果（必须知晓）**：
  1. 上游（Komari 内核、ink 主题）的**新功能、性能改进、安全修复都不会自动到达**本仓；
  2. 是否搬运某个上游提交，由我们自行判断；搬运属**非常态动作，需要授权**（见 §3）；
  3. 依赖层面的漏洞风险由我们自担（见 §4），没有上游兜底；
  4. 兼容性责任在我们：面板与 agent 的协议兼容需自测（agent 侧目前沿用上游产物，升级要自行验证）；
  5. 反过来说，我们对自己的改动拥有完全控制权，不会被上游推送覆盖。

## 2. 版本线规则

| 对象 | 版本形式 | 含义 | 标签 |
|---|---|---|---|
| 内核（`server`） | `0.0.24+own.N` | 前缀（`0.0.24`）**是我们最后一次对齐的上游版本**，用于追溯基线；`N` 是自维护迭代号，从 1 起递增 | `server/v0.0.24-own.N` |
| Docker 镜像 tag | `komari-own:0.0.24-own.N` | tag 不允许 `+`，故用 `-` 代替语义版本的元数据分隔符 | — |
| 主题（`themes/ink`） | `0.0.x` | 完全自有线，**与上游主题 0.6.x 无语义关系**（0.6.14 只是历史起点） | `theme-ink/v0.0.x` |

发布动作（每次发版固定四步）：改版本源（内核 `scripts/version.env` / 主题 `themes/ink/komari-theme.json`）→
`./build.sh` → 打标签 → 按 `ops-docs/DEPLOY.md` 部署并验证。

## 3. 逃生通道：确需上游某个提交/修复时（非常态，需授权）

仅当出现"上游有我们必须拿到的东西"（如关键安全修复）时才走，用完即拆：

```bash
git remote add upstream-tmp <上游仓库>     # 临时，命名带 -tmp
git fetch upstream-tmp --tags
git switch -c port/upstream-<日期>         # 临时分支
git cherry-pick <提交>                     # 或 git merge upstream-tmp/main（按需选择）
# —— 评审：看 diff、跑 ./build.sh、必要时本地冒烟 ——
git switch main && git merge --no-ff port/upstream-<日期>
git remote remove upstream-tmp             # 用完必须移除，避免误 fetch/误同步
```

约束：**不要**把上游设成常态 remote；**不要**做定时/自动同步；搬运的每个提交都要在 Trellis 任务与提交信息里写清来源与理由。

## 4. 依赖安全自查（脱离上游后由我们自担）

### 4.1 命令

内核（Go，模块定义在仓库根 `go.mod`）：

```bash
go list -m -u all        # 列出可升级依赖（含当前版本与可用版本）
go mod verify            # 校验本地模块缓存完整性
govulncheck ./...        # 已知漏洞扫描（按调用可达性判断，噪声低）
```

> **工具现状**：本机 `govulncheck` **尚未安装**（`command -v govulncheck` 为空）。安装方式：
> `go install golang.org/x/vuln/cmd/govulncheck@latest`（需联网，属需授权的动作）。
> 未安装时的替代做法：① `go list -m -u all` 找出版本落后的依赖；② 对其中直接依赖逐个查其
> 上游公告（如 `pkg.go.dev` / 项目 Release Notes）；③ 必要时用 `osv-scanner` 或
> GitHub 的 Dependabot 告警（若仓库启用）作为交叉验证。

主题 / 前端（Bun，锁文件 `themes/ink/bun.lock`）：

```bash
cd themes/ink && bun audit          # 本机 bun 1.4.2 已带该子命令
# 若改用 npm 流程（存在 package-lock.json 时）：npm audit --omit=dev
```

### 4.2 频率建议

- **每月一次**例行自查（可挂在月初）；
- 出现**重大 CVE 通告**（Go 标准库 / 关键直接依赖 / 前端框架）时**立即**执行一次；
- 每次内核或主题**依赖变更后**必须重跑一次。

### 4.3 发现问题怎么处理（三步）

1. **评估**：确认漏洞是否落在我们**实际使用**的代码路径上（`govulncheck` 的可达性结论、依赖是否为直接依赖、是否有可用修复版本）；
   不可达或仅开发期依赖 ⇒ 记录并降级处理。
2. **升级并构建验证**：改 `go.mod` / `bun.lock` → `./build.sh`（内核二进制 + 镜像 + 主题 zip）→
   主题侧 `cd themes/ink && bun run lint`；必要时在本地起容器冒烟（见 `ops-docs/BUILD.md`）。
3. **部署**：按 `ops-docs/DEPLOY.md` 先备份再替换/重建；异常按 `ops-docs/ROLLBACK.md` 秒级回滚。

> 依赖升级属"改代码/改依赖"的写入动作，需要当轮授权后再做；本文只定义规则与命令。

## 5. 责任与留痕

- 谁决策：内核代码改动、新增/升级依赖、部署与标签 **均需明确授权**；
- 怎么留痕：每个动作建 Trellis 任务（`.trellis/tasks/`），提交信息带 `[task:<slug>]`（闸门要求，见 `ops-docs/BUILD.md`「运维经验」）；
- 怎么回滚：主题与内核都保留"先备份再替换"的路径（`ops-docs/ROLLBACK.md`）。
