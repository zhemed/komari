# 独立维护声明（内核与主题均脱离上游）

> 决策：本仓 `zhemed/komari` 是**我们自维护的唯一权威来源**；**内核与主题均不再跟随任何上游**。
> 本文是这条决策的规则文本，与 `ops-docs/VERSION.md`、`ops-docs/BUILD.md`、`ops-docs/DEPLOY.md`、`ops-docs/ROLLBACK.md` 配套。

## 1. 决策声明

- **唯一权威来源**：内核代码 + 主题（`themes/ink`，自有版本线 `1.x`）+ 部署与构建脚本（`deploy/`、`build.sh`）+
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
| 主题（`themes/ink`） | `1.x` | 完全自有线，**与上游主题 0.6.x 无语义关系**（0.6.14 只是历史起点）；1.0.0 起独立维护，更新通道（滚动 release `theme-latest`）与市场目录（`deploy/theme-market/v1.json`）均由本仓提供（见 `VERSION.md`） | `theme-ink/v1.x` |

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

### 6.5 修复结果（2026-10-23，同一轮授权内）

| 项 | 修复前 | 修复后 |
|---|---|---|
| 内核 `govulncheck ./...` | 13 项影响调用路径（退出码 3） | **0 项**（退出码 0，`No vulnerabilities found`；另有 2 项在依赖模块中但**不可达**） |
| Go 工具链 | go1.26.6 | **go1.26.10**（`go mod edit -toolchain=go1.26.10`，由 Go 自动下载；未替换系统 Go） |
| golang.org/x/net | v0.41.0 | **v0.60.0** |
| golang.org/x/text | v0.33.0 | **v0.42.0**（x/net v0.60.0 要求 ≥ v0.42.0） |
| 连带升级 | — | x/crypto v0.57.0、x/sync v0.23.0、x/sys v0.48.0；`go` 指令 1.25.0 → 1.26.0 |
| 主题 `bun audit` | 20 项（3 critical / 9 high / 7 moderate / 1 low） | **1 项（1 high）**：`braces@3.0.3` |
| 主题升级 | — | brace-expansion 1.1.21/2.1.7/5.0.12、postcss-selector-parser 7.1.7、shell-quote 1.12.0、source-map-js 1.2.2、vite 7.3.7、tinypool 2.2.0（经 `overrides`） |

**残留 1 项说明（可接受）**：`braces@3.0.3`（GHSA-vfj7-8cjw-p6xm，high，嵌套模式导致栈溢出 DoS）——
上游**最新发布版就是 3.0.3**，而公告影响范围是 `<=3.0.3` ⇒ **当前不存在可升级到的修复版本**。
影响面：仅出现在 `chokidar` / `micromatch`（devDependency 链：`vue-router`、`vite-plugin-static-copy`、`fast-glob`），
且其输入是我们自己的 glob 模式，不来自不可信来源 ⇒ 记为**可接受残留**，下次自查复查（上游一发修复版即可升级）。

**版本**：内核 `0.0.24+own.2`（镜像 `komari-own:0.0.24-own.2`）；主题仍 **0.0.1**（本轮只动 devDependency，线上主题产物无需变更）。

### 6.6 覆盖补齐（2026-10-23，trivy + frontend + agent）

**新增工具**：`trivy v0.76.1`（GitHub release 二进制 → `/usr/local/bin/trivy`）；既有 `go1.26.10`、`govulncheck v1.9.0`、`bun 1.4.2`、`npm`。
未装 `grype`/`syft`（trivy 已覆盖同类能力，避免重复）；未装 `zig`（本机走 glibc 构建路径，不需要 alpine 静态构建）。

#### a) frontend（内核自带 React 管理端，npm 生态）

| 步骤 | 结果 |
|---|---|
| `npm audit`（官方源）修复前 | 14 项（11 high / 2 moderate / 1 low）；**运行期 `--omit=dev` 8 项 high**：`braces`、`follow-redirects`、`http-proxy-middleware`、`micromatch`、`nanoid`、`postcss`、`source-map-js`、`vite` |
| `npm audit fix --package-lock-only` | **0 vulnerabilities**（运行期也 0） |
| `npm ci` + `npm run build` | 通过（8.05 s，`dist` 6.5 MB，PWA 426 条预缓存） |
| 入库内容 | 仅 `frontend/package-lock.json`；**未重新生成内嵌的前端产物**（见"未完成"） |

#### b) agent 模块（Go）

| 步骤 | 结果 |
|---|---|
| `go mod verify` | `all modules verified` |
| `go list -m -u all` | 36 个可升级 |
| `govulncheck ./...` 修复前 | **14 项影响调用路径**（标准库 5、`x/net` 6、`x/text` 1、`x/crypto` 1、`ulikunitz/xz` 1） |
| 最小升级 | toolchain `go1.26.10`、go 指令 1.26.0；`x/net` v0.38.0→**v0.60.0**、`x/text` v0.26.0→**v0.42.0**、`x/crypto` v0.39.0→v0.57.0、`xz` v0.5.9→**v0.5.15** |
| `govulncheck ./...` 修复后 | **1 项**：GO-2026-5932 `golang.org/x/crypto/openpgp`（该包已停止维护，**无修复版本 N/A**）——依赖链遗留，我们未使用其业务逻辑 |
| `go build ./...` | 通过 |

#### c) 容器镜像（`trivy image --severity HIGH,CRITICAL`）

| 镜像 | 合计 | Go 依赖项 | OS 包项 | 有修复版本 |
|---|---|---|---|---|
| 官方 `ghcr.io/zhemed/komari:0.0.24`（alpine 3.21.7） | 38 | **24**（`x/crypto`/`x/net`/`stdlib` 均为旧版） | 14 | 24 |
| 自建 `own.2`（debian 12.15） | 60 | **1**（`klauspost/compress` v1.17.11） | 59 | 1 |
| 自建 **`own.3`**（本次修复后） | **59** | **0** | 59 | **0** |
| 裸 `debian:bookworm-slim`（对照） | 57 | — | 57 | 0 |
| 裸 `debian:trixie-slim`（对照） | 43 | — | 43 | 0 |

**结论**：①我们镜像的 **Go 依赖漏洞已清零**（对照官方 24 项）；②剩余 59 项**全部是 Debian OS 包且 Debian 未发布修复版本**（含 1 项 CRITICAL `zlib1g` CVE-2023-45853，Debian 长期 wontfix），**裸镜像本身就有 57 项** ⇒ 与我们的 Dockerfile/构建方式无关；③换 `trixie-slim` 只能降到 43 项，且属于变更基础镜像版本 ⇒ **按约束先回报，本轮未换**；④`trivy fs`（仓库扫描）只有 **2 项**：`braces@3.0.3`（CVE-2026-93687，HIGH，**无修复版**），分别来自 `frontend/package-lock.json` 与 `themes/ink/bun.lock`。

**最小修复与部署**：内核 `klauspost/compress v1.17.11 → v1.18.7` ⇒ 重建镜像 **own.3** 并部署（备份 DB/inspect/旧脚本 → `docker save | ssh docker load` → 新版本化脚本 `deploy/komari-v0.0.24-own.3-listen-local.sh` 重建 → 核对通过）；标签 `server/v0.0.24-own.3`。

### 6.7 收尾修复：内嵌 admin 前端重建 + trixie 基础镜像（own.4）

#### a) admin 前端重建（让 frontend 的依赖修复真正进产物）

| 步骤 | 结果 |
|---|---|
| `scripts/build-frontend.sh`（首次） | 重建成功，但哈希门禁失败：期望 `f90849d1…`、实际 **`ab48b91d…`**（依赖解析变化，符合预期） |
| 更新 `scripts/frontend-build.env` | `FRONTEND_TREE_SHA256` → `ab48b91d6f164cad8ad458619ed7e7daa25c7eed0f8880aa5c5727b5ca327ecf`，并补写第十一次变更记录（说明是依赖安全修复导致） |
| 复跑自检 | ✅ `哈希校验通过（与 scripts/frontend-build.env 一致）` |
| 产物变化量 | `web/public/defaultTheme` 431 个跟踪文件里 **116 个内容变化**；体积仍 7.2 M（未变） |
| admin 入口 | `entry-index-BNk8udx-.js` → **`entry-index-Cuh5tdrm.js`**（部署后线上可验证） |
| frontend `npm audit` | 仍为 **0** |

#### b) 基础镜像 bookworm → trixie

`deploy/Dockerfile.local`：`FROM debian:bookworm-slim` → `FROM debian@sha256:62bd423aeee912639ca8ee681c2b762cb3e25137e0a92b20be455898a9f034ad`（= `debian:13/trixie-slim`，钉 digest；附注释说明原因）。镜像体积 164 MB → **163 MB**。

#### c) 镜像漏洞对照（`trivy image --severity HIGH,CRITICAL`）

| 镜像 | 合计 | Go 依赖项 | OS 包项 | 有修复版本 |
|---|---|---|---|---|
| own.3（bookworm） | 59（HIGH 58 + CRITICAL 1） | 0 | 59 | 0 |
| **own.4（trixie）** | **43（HIGH 43 + CRITICAL 0）** | **0** | 43 | **0** |
| 裸 `debian:trixie-slim`（对照） | 43 | — | 43 | 0 |
| 裸 `debian:bookworm-slim`（对照） | 57 | — | 57 | 0 |

**结论（关键）**：own.4 的 43 项与**裸 `debian:trixie-slim` 完全同源同数**（43 = 43），即全部来自 Debian 基础镜像本身，**每一个都没有 Debian 发布的修复版本**（`FixedVersion` 均为空）；我们的二进制与配置**没有引入任何额外漏洞**（Go 依赖项为 0）。CRITICAL 项已随 bookworm→trixie 归零。

#### d) `trivy fs`（仓库文件系统）

仍为 **2 项**：`braces@3.0.3`（CVE-2026-93687，HIGH，**上游无修复版本**），分别来自 `frontend/package-lock.json` 与 `themes/ink/bun.lock`。无新增。

#### e) 部署 own.4

备份（DB 在线 backup `/root/backups/komari-db-20261023142847.db`、inspect 快照、own.3 脚本）→
`docker save | ssh docker load` → 新脚本 `deploy/komari-v0.0.24-own.4-listen-local.sh` 重建 →
容器 `running`、`Restarts=0`、`/api/version` hash `fb9f973d…`（= 我方 HEAD）、端口 `127.0.0.1:25774`、
nginx `/` 与 `/admin` 双次 200、**`/admin` 入口已变为 `entry-index-Cuh5tdrm.js`**、主题入口仍 `index-08OsQRET.js`、
三处「本周期流量」不变、DB 周期增长且累计不回退、日志无异常。

### §6.8 安全基线（用户 2026-10-23 决定：接受并记为基线）

- **基线口径**：镜像以 `trivy image --severity HIGH,CRITICAL` 的结果为准；**"有修复版"的项必须为 0**；
- **当前基线（own.4，`debian:trixie-slim` 钉 digest）**：HIGH 43 ｜ CRITICAL **0** ｜ Go 依赖 **0** ｜ 有修复版 **0**；
  这 43 项与**空白 `debian:trixie-slim` 镜像的 43 项完全同源同数**、`FixedVersion` 全空
  ⇒ 全部是 **Debian 尚未发布修复**的 OS 包，**不属于我们的代码或配置问题**；
- **复查方式**：`trivy image --severity HIGH,CRITICAL --ignore-unfixed <image>` 应输出 0 项；
  待 Debian 发布修复后重建镜像即可自然下降；
- **已知上游无修复版残留**（记录在案，不阻塞）：
  - `braces@3.0.3`（CVE-2026-93687，dev/构建锁文件，`frontend/package-lock.json` + `themes/ink/bun.lock`）；
  - agent `x/crypto/openpgp`（GO-2026-5932，包已停维）；
- **决策记录**：不再为继续压 OS 项而更换发行版家族（alpine 静态构建需 zig）——如日后重启该议题，走本文档 §3 的授权流程。
