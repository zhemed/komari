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

## 6. 最近一次依赖安全自查

- **时间（UTC）**：2026-10-23T13:59:36Z
- **执行人**：我们（自维护；命令与原始输出见提交说明，摘要如下）
- **环境**：Go `go1.26.6` ｜ `govulncheck v1.9.0`（本次安装）｜ Bun `1.4.2` ｜ 默认 npm 源为镜像站（`bun audit` 需切官方源，见 §4.1 备注）
- **本轮未做任何升级**：仅记录，升级动作另行授权。

### 6.1 命令与关键输出

| 命令 | 结果 |
|---|---|
| `go mod verify` | `all modules verified`（本地模块缓存无损坏） |
| `go list -m -u all` | 73 行模块；**45 个有可升级版本**（例：`gin v1.10.0 → v1.12.0`、`klauspost/compress v1.17.11 → v1.20.3`、`jackc/pgx/v5 v5.10.0 → v5.11.0`、`validator/v10 v10.26.0 → v10.30.5`） |
| `govulncheck ./...` | 退出码 3：**13 项漏洞影响我们的调用路径**；另有 12 项在导入包、23 项在依赖模块中，判定为不可达 |
| `cd themes/ink && bun audit`（`npm_config_registry=https://registry.npmjs.org`） | **20 项**：3 critical ／ 9 high ／ 7 moderate ／ 1 low，涉及 **7 个包**（全部位于 devDependency / 构建工具链） |

### 6.2 内核：13 项可达漏洞

| # | ID | 模块 | 当前 | 修复版本 | 轨迹数 |
|---|---|---|---|---|---|
| 1 | GO-2026-6809 | crypto/x509（标准库） | go1.26.6 | go1.26.10 | 1 |
| 2 | GO-2026-6629 | golang.org/x/text | v0.33.0 | v0.41.0 | 1 |
| 3 | GO-2026-6617 | golang.org/x/net（HTTP/2） | v0.41.0 | v0.60.0 | 177 |
| 4 | GO-2026-6613 | net/http（标准库） | go1.26.6 | go1.26.9 | 51 |
| 5 | GO-2026-6612 | golang.org/x/net（HTTP/2） | v0.41.0 | v0.60.0 | 177 |
| 6 | GO-2026-6611 | golang.org/x/net（HTTP/2） | v0.41.0 | v0.60.0 | 177 |
| 7 | GO-2026-6610 | golang.org/x/net（HTTP/2） | v0.41.0 | v0.60.0 | 6 |
| 8 | GO-2026-6609 | net/http（Range 头） | go1.26.6 | go1.26.9 | 2 |
| 9 | GO-2026-6608 | net/textproto（MIME） | go1.26.6 | go1.26.9 | 2 |
| 10 | GO-2026-6607 | crypto/tls（ECH） | go1.26.6 | go1.26.9 | 5 |
| 11 | GO-2026-6605 | net/http（CONNECT） | go1.26.6 | go1.26.9 | 3 |
| 12 | GO-2026-6603 | golang.org/x/net（HTTP/2 Trailer） | v0.41.0 | v0.60.0 | 177 |
| 13 | GO-2026-5970 | golang.org/x/text | v0.33.0 | v0.39.0 | 3 |

修复路径：**升级 Go 工具链到 ≥ go1.26.10**（覆盖 6 项标准库问题）＋ **`golang.org/x/net` ≥ v0.60.0**（5 项）＋ **`golang.org/x/text` ≥ v0.41.0**（2 项）。

**影响面判断**：13 项里 10 项属 HTTP/2 服务端 / TLS / x509 路径；我们的部署形态是 nginx 终止 TLS、
回源 HTTP/1.1 到 loopback（见 `ops-docs/DEPLOY.md`），**未对公网直接暴露 Go 的 HTTP/2 服务端**【推断】，
因此实际可利用性低于"公网直连"场景；但 ① x509 证书校验（GO-2026-6809，出网抓取 GeoIP 库时用到）
② net/http 的 Range/CONNECT 解析问题仍应尽快修。

### 6.3 主题：7 个受影响包（全部在构建/测试工具链）

| 包 | 最高级别 | 代表公告 | 链条性质 |
|---|---|---|---|
| shell-quote@1.10.0 | **critical** | GHSA-pqg4-j6r4-53mv（`quote()` 命令注入） | `npm-run-all2`（构建脚本编排） |
| tinypool@2.1.0 | **critical** | GHSA-5gmw-xhrv-c9v3 / GHSA-85c8-ppgw-ccpr（原型污染 → RCE） | vitest 测试池 |
| brace-expansion（1.1.18 / 2.1.4 / 5.0.9） | high | GHSA-qhr7-859c-m2p7、GHSA-6j4f-fj2g-mc7p（递归 DoS） | eslint / archiver / minimatch 链 |
| braces@3.0.3 | high | GHSA-vfj7-8cjw-p6xm（嵌套模式栈溢出 DoS） | chokidar / micromatch 链 |
| source-map-js@1.2.1 | high | GHSA-68fv-2mgg-jv7q（source-map 段索引 DoS） | vue 编译器 / tailwind 链 |
| vite@7.3.6 | high（含 moderate/low） | GHSA-587h-7wmj-h5pm、GHSA-9jrq-w75r-8gcw、GHSA-rq7h-c2jc-7f22 等 | 构建/开发服务器（含 launch-editor） |
| postcss-selector-parser@7.1.5 | moderate | GHSA-rj75-hqrm-r3gf（选择器解析 CPU 耗尽） | PostCSS/Tailwind 链 |

**影响面判断**：主题的**运行期依赖**（`vue`、`pinia`、`echarts`、`reka-ui`、`vue-router` 等）**未出现公告**；
上述 7 个包只在我们**构建/检查/测试**时加载 ⇒ 生产站点不受直接影响；但在构建机或 CI 上处理不可信输入时需谨慎
（两个 critical 都在这一层）。

### 6.4 待办（本轮不执行，需授权后按 §4.3 三步走）

1. 升级 Go 工具链到 **≥ go1.26.10**（本地、构建脚本与将来的 CI 一起改）；
2. `go get golang.org/x/net@v0.60.0 golang.org/x/text@v0.41.0` + `go mod tidy` → `./build.sh` 验证 → 按 `ops-docs/DEPLOY.md` 部署；
3. 主题：`cd themes/ink && bun update`（或对上述 7 个包定向升级）→ `bun run lint` + `./build.sh` 验证；
4. 把"`bun audit` 需切官方源（默认镜像站不支持 advisories 接口）"补进 `ops-docs/BUILD.md`；
5. 下次自查：**一个月内**或出现重大 CVE 时立即执行，并在本节追加一行结果。
