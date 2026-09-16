# Journal - komari (Part 1)

> AI development session journal
> Started: 2026-09-16

---



## Session 1: 锁定 komari 1.4.3 为自维护分叉基线
<!-- trellis-session: v=2 fp=81e26ddfd4d7e731 -->

**Date**: 2026-09-16
**Task**: 锁定 komari 1.4.3 为自维护分叉基线
**Branch**: `komari-1.4.3`

### Summary

从上游 tag 1.4.3（bf6b45ec）建立自有主干 komari-1.4.3；vendor komari-web@4a74e8a8 前端产物使后端可离线构建（上游 tag 不锁前端版本，且 //go:embed defaultTheme 使后端单独无法构建）；新增 sync-frontend.sh（pin + 补丁 + npm ci + 目录树哈希门禁）与 build-komari.sh；补丁 0001 把后台更新检查改指 zhemed/komari，补丁 0002 让 vite 构建时间遵循 SOURCE_DATE_EPOCH 以修复产物不可复现（上游把构建时刻写入 Footer 所在 chunk，导致 chunk 哈希连锁漂移）；install-komari.sh 去上游化并修复 404 覆写运行中二进制的风险；推送至 github.com/zhemed/komari（公开）。AC1-AC7 全部验证：GOPROXY=off 离线构建、/install+assets 冒烟、两次再生成哈希一致、二进制内无上游 releases 地址。

### Git Commits

| Hash | Message |
|------|---------|
| `e70304f` | chore: 将仓库固定为只维护 komari 1.4.3 的自有基线 |
| `5942199` | docs: 记录 1.4.3 分叉的构建契约与推送陷阱 |

### Status

[OK] **Completed**


## Session 2: 0.0.1 自有基线 + 彻底移除插件系统
<!-- trellis-session: v=2 fp=770c991de2029d42 -->

**Date**: 2026-09-16
**Task**: 0.0.1 自有基线 + 彻底移除插件系统
**Branch**: `komari-1.4.3`

### Summary

把项目版本全面改为自有的 0.0.1（1.4.3 仅作为代码来源记录），并彻底移除插件系统：后端删除 internal/plugin 整包（21 文件/5414 行）与市场 API、插件 RPC、公开路由、模型与迁移注册、备份白名单条目、WebSocket 帧拦截器（约 -5.4k 行）；前端以补丁 0003 删除 5 个插件页面、路由与菜单组、types/plugin.ts、resolvePluginIcon、上传 purpose 与流量报告的插件市场 CTA（-1871 行）。保留 pkg/jsruntime（JS 消息发送器依赖）与主题系统。vendor 产物重新生成，FRONTEND_TREE_SHA256 由 eb98cd78 更新为 9bd12c8a（两次独立再生成一致）。AC1-AC7 全部实测通过，含真实升级路径验证：1.4.3 数据目录启动 0.0.1 触发一次 upgrade 备份且不重复。已推送 zhemed/komari。

### Git Commits

| Hash | Message |
|------|---------|
| `18305a9` | feat!: 以 0.0.1 为自有基线并彻底移除插件系统 |

### Status

[OK] **Completed**


## Session 3: 重写历史为自有基线并发布 0.0.1
<!-- trellis-session: v=2 fp=5296d7158882237c -->

**Date**: 2026-09-16
**Task**: 重写历史为自有基线并发布 0.0.1
**Branch**: `main`

### Summary

把仓库彻底变成自维护项目并发布首个版本。(1) 历史重写：用上游 1.4.3 的 tree 造单一快照根提交 76cd97f，rebase --onto 把我们的 11 个提交重挂上去——上游 835 个提交与上游作者全部消失；关键门禁 git diff backup/pre-rewrite HEAD 为空，证明只动历史未动代码；重写前 HEAD 9d2bb44 保留在本地分支 backup/pre-rewrite（未推送）；删除本地上游 tag 68 个。(2) 分支改名 komari-1.4.3 → main：强推、切换默认分支、删除远端旧分支。(3) 删除上游 README.md/README_zh-cn.md 并重写为我们的单份 README（143 行，含来源与许可）；删除上游 .github/workflows(10)/actions(3)/ISSUE_TEMPLATE(3)，保留 LICENSE 与 NOTICE。(4) build-komari.sh 支持 KOMARI_STATIC=1 与 KOMARI_GOARCH=arm64（zig musl 静态）：本机动态 37.7MB、静态 amd64 48.4MB、arm64 46.5MB。(5) 补完 trellis init 遗留任务：5 份 backend 规范按代码取证填充（942 行/307 锚点），并修掉 web/public/.gitignore 的死引用。(6) 发布 0.0.1：两个静态资产，实测 release 下载运行 + docker(alpine) 构建运行 + 远端全新克隆离线构建全部通过。两条教训：本仓库有两个 remote，gh 会解析到 upstream，必须显式 -R zhemed/komari；文档重命名要全仓 grep（含 .gitignore）。

### Git Commits

| Hash | Message |
|------|---------|
| `45f9f1c` | docs: 用分支引用替代硬编码的重写前 HEAD（避免提交后立即过期） |
| `e3a104f` | docs: 记录 gh 双 remote 陷阱（发布时需 -R zhemed/komari） |

### Status

[OK] **Completed**


## Session 4: 解除内网地址限制、移除 HTTPS 横幅并发布 0.0.2
<!-- trellis-session: v=2 fp=47758a25cb978ca3 -->

**Date**: 2026-09-16
**Task**: 解除内网地址限制、移除 HTTPS 横幅并发布 0.0.2
**Branch**: `main`

### Summary

修复内网部署下两个上游遗留限制并发布 0.0.2。(1) 主题市场被自家 SSRF 防护拦死：isPrivateIP() 用 net.LookupHost 且解析失败即 fail-closed（theme.go:315），默认主题市场源 raw.githubusercontent.com（theme_market.go:25）在部分网络被解析成私网/污染地址，直接报 requests to private or internal addresses are not allowed。改为新增 blockPrivateEndpoints()，默认不拦截，KOMARI_BLOCK_PRIVATE_ENDPOINTS=1 可恢复；新增回归测试 theme_market_ssrf_test.go 固化两种模式。(2) 移除非 HTTPS 全站红色告警（Index.tsx/AdminPanelBar.tsx/terminal/index.tsx 三处 + 5 个语言包文案），前端补丁 0004 纯删除 90 行。(3) 版本推进 0.0.2，前端产物重新生成 FRONTEND_TREE_SHA256 更新为 e8b3edf9（两次一致）。(4) 发布 0.0.2 静态资产（amd64 48443168 / arm64 46518400），本地实例改用 release 资产重新部署（sha256 与本地构建一致 40b02a34），实测首页渲染无红条、页脚显示 0.0.2 (241dddb)；两次版本标识变化各触发一次 ./data 自动备份（0.0.1→0.0.2、0.0.2-hashA→0.0.2-hashB），验证了升级备份机制在真实部署中生效。

### Git Commits

| Hash | Message |
|------|---------|
| `241dddb` | fix!: 默认允许内网地址下载，移除全站 HTTPS 告警横幅（0.0.2） |

### Status

[OK] **Completed**


## Session 5: 彻底移除通知系统与内嵌 JS 运行时，发布 0.0.3
<!-- trellis-session: v=2 fp=7363e34bc64531f4 -->

**Date**: 2026-09-16
**Task**: 彻底移除通知系统与内嵌 JS 运行时，发布 0.0.3
**Branch**: `main`

### Summary

把通知子系统与随之失去消费者的 pkg/jsruntime 整体删除，并发布 0.0.3。(1) 后端约 -14.4k 行：删除 utils/messageSender（框架+8 渠道）、utils/notifier（离线/负载/流量/流量报告/到期）、database/notification、通知模型与迁移步骤、通知 RPC 与路由、admin:testSendMessage、调度任务与消息发送器重载钩子、internal/config 的通知配置项；pkg/jsruntime（46 文件/11427 行）一并移除，go.mod 去掉 goja/goja_nodejs/base64dec。(2) 4 个跨模块接线点重接：agent 上/下线改 logger 日志；登录改 auditlog.EventLog(auth)，因为此前登录只有通知这一条痕迹；续费本就有审计日志。(3) 前端补丁 0005 纯删 2319 行/13 文件：删除 5 个通知页面、菜单组、路由、零消费者的 TrafficReportContext.tsx，并清理 5 个语言包中无引用的 notification/loadAlert/admin.notification/settings.notification 文案。(4) 修掉两个上游遗留：reg() 注册助手原定义在被删的 admin.notification.go 里（删文件会让整个包编译失败），已迁移到 registry_helpers.go；迁移测试中依赖已删模型的断言已移除。(5) 验收：build/vet/test 全绿；二进制 48443168 → 35210400 字节（-13.2MB / -27%）；POST 三个被删端点均 404（GET 被 SPA 兜底为 200 text/html，非真实端点）；线上 bundle 无通知菜单；前端哈希 0dda6f17（两次一致）；0.0.3 已发布并部署到本地，三次版本切换各触发一次自动备份。教训：删除包含通用助手的文件前要先确认它是否被全包共用。

### Git Commits

| Hash | Message |
|------|---------|
| `b397115` | feat!: 彻底移除通知系统与内嵌 JS 运行时（0.0.3） |

### Status

[OK] **Completed**


## Session 6: Session 6: agent 自有发行线（0.0.4）——单仓库发布 agent、默认关自更新、自建镜像
<!-- trellis-session: v=2 fp=c2f9524d05bd847e -->

**Date**: 2026-09-16
**Task**: Session 6: agent 自有发行线（0.0.4）——单仓库发布 agent、默认关自更新、自建镜像
**Branch**: `main`

### Summary

把 agent 纳入我们的发行线但不新增仓库：源码 pin 上游 9e532e04 + 补丁系列（自更新目标改指 zhemed/komari、selfupdate 资产过滤 ^komari-agent-、容器内跳过、默认关自动更新），agent 二进制作为本仓库 release 资产发布（14 平台，纯 Go 不需 zig），前端补丁 0006 把安装命令/镜像/关于页 README/GitHub 按钮改指本仓库，Dockerfile.agent + build-agent-image.sh 推 ghcr.io/zhemed/komari-agent。发布 0.0.4（17 个资产，tag==资产==镜像源码）并在本机端到端：用我们的安装脚本升级本地服务器（二进制 sha256 与 release 资产一致、触发自动备份），再用面板同款命令装 agent（自动发现）→ 节点 Auto-ubuntu 版本 0.0.4 在线、v2 协议、截图留档。关键发现：go-github-selfupdate 按下缀匹配资产，同一 release 混装服务器与 agent 二进制时 agent 会把自己刷成服务器二进制——用 0.0.99/0.0.98 两个临时 release 做了对照实验（带过滤 I-AM-AGENT / 去掉过滤 I-AM-SERVER）后把过滤写进补丁并加了构建期自检。教训：多架构镜像不能有 RUN（无 QEMU 时 exec format error）；raw.githubusercontent 的 refs/heads/main 对新文件会短暂 404（约 10 分钟自愈）；发版必须先提交再构建（二进制内嵌 VCS 信息）。

### Git Commits

| Hash | Message |
|------|---------|
| `2ba82cc` | feat(agent): 建立我们自己的 agent 发行线（0.0.4 基线） |
| `1a2288d` | fix(agent): 镜像改为纯 COPY 构建，避免多架构构建依赖 QEMU |
| `5d047d8` | docs(agent): 收尾 0.0.4——运维坑记录、过期注释修正与验收记录 |
| `4e88360` | test(agent): 构建时自检“资产过滤 + 自更新目标”两条关键防线 |

### Status

[OK] **Completed**


## Session 7: Session 7: README 改成产品视角短文 + 发布服务器镜像 ghcr.io/zhemed/komari
<!-- trellis-session: v=2 fp=fe7b5c91d91a845e -->

**Date**: 2026-09-16
**Task**: Session 7: README 改成产品视角短文 + 发布服务器镜像 ghcr.io/zhemed/komari
**Branch**: `main`

### Summary

用户看到 zhemed/new-api-own 的 README 风格（定位→特性→部署→维护）表示很喜欢，按这个风格重写本仓库 README：114 行，特性 10 条 + 本 fork 取舍（无插件/无通知/默认不自动升级）+ 部署四条路（systemd 脚本 / docker run / 源码构建 / 节点 agent）+ 维护与许可；原来的维护者长文（构建契约、发布清单、CLI 子命令、数据目录、容器镜像）下沉到 docs/MAINTAINING.md，新增 §3.5 命令行子命令、§3.6 数据与备份、§12 容器镜像。为了让 README 的 Docker 一节是真命令，新增 scripts/build-server-image.sh 并发布 ghcr.io/zhemed/komari:0.0.4/:latest（amd64+arm64，静态产物校验、缺产物即报错），两个 Dockerfile 补 OCI source 标签。踩到的坑：用户级 ghcr 包的可见性无法用 API 改（PATCH /user/packages/... 一律 404，连已公开的 litepan 也一样），新包默认 private，只能网页点 Public——已写进 MAINTAINING §12，并把匿名可拉验证留作待办。实测：服务器镜像 /install 200、日志 0.0.4、数据落卷；agent 镜像入口正常；README 四条命令逐条跑通。

### Git Commits

| Hash | Message |
|------|---------|
| `fc7cf35` | docs: README 改为产品视角短文；发布服务器镜像 ghcr.io/zhemed/komari |

### Status

[OK] **Completed**


## Session 8: Session 7 收尾: 镜像公开验证通过（README 的 docker run 已可匿名使用）
<!-- trellis-session: v=2 fp=0130d97aaf322d8d -->

**Date**: 2026-09-16
**Task**: Session 7 收尾: 镜像公开验证通过（README 的 docker run 已可匿名使用）
**Branch**: `main`

### Summary

用户把 ghcr 两个包手点成 Public 后完成验证：komari / komari-agent 的 :0.0.4 与 :latest 四个 tag 用空凭据 DOCKER_CONFIG 匿名 manifest inspect 全部成功；服务器镜像用空凭据 docker run 后 /install 返回 200、日志版本 0.0.4、数据落进挂载卷 /app/data，agent 镜像匿名 --help 正常。已把 MAINTAINING §12 的状态改成“两个包均已人工设为 public”并留下发版后必跑的匿名可拉验证脚本片段；任务 09-16-readme-product-style 验收通过并归档。

### Git Commits

| Hash | Message |
|------|---------|
| `fc7cf35` | docs: README 改为产品视角短文；发布服务器镜像 ghcr.io/zhemed/komari |
| `dbc0410` | chore: record journal |

### Status

[OK] **Completed**


## Session 9: Session 8: agent 从 1.5.10 退回 1.4.3 同期血统（发 0.0.5），motd 告警事件复盘
<!-- trellis-session: v=2 fp=04ae88c07655353b -->

**Date**: 2026-09-16
**Task**: Session 8: agent 从 1.5.10 退回 1.4.3 同期血统（发 0.0.5），motd 告警事件复盘
**Branch**: `main`

### Summary

用户在本机 /etc/motd 看到 '[Komari] Remote control is enabled on this device' 告警并怀疑我们拉到了 1.5。查证：告警不是入侵（是 agent 自己写的，cmd/warn_linux.go 注入，触发条件是远程控制开着），但版本确实漂了——0.0.4 的 agent 误 pin 上游 agent tag 1.5.10（2026-09-15），比服务器/前端 1.4.3（2026-08-13）晚一个月，motd 注入来自 79d8d45 增强安全提醒（2026-09-14）。(1) 处置：pin 退回 1186aafb（2026-08-07，1.4.3 之前最后一个 agent 提交，协议 v2/自动发现/参数面齐全）；安装脚本补丁 0002/0003 按旧版重做（旧版是 #!/bin/bash、无 snapshot 通道），install-agent.sh/.ps1 重新 vendor；发 0.0.5（17 资产 + 镜像 :0.0.5/:latest）。(2) 机制化防线：build-agent.sh 新增 pin 日期硬校验（不得超过 KOMARI_AGENT_MAX_COMMIT_DATE=2026-08-13），反例已验证退出码 1。(3) 本机：清 /etc/motd、重装 agent（身份复用同一 uuid）、服务器升 0.0.5（二进制 sha256 与 release 资产一致）、面板 0.0.5/节点在线。(4) 退回代价逐条核对：丢 3 个检测修复（AMD GPU sysfs/Android FUSE/macOS nullfs）+ 安装脚本两处改进；文件访问/终端重连/上传链路我们调不到，不算损失且攻击面更小。(5) 两次踩坑记录进文档：MAINTAINING §11.6 讲这条告警是什么、§11.5 讲为什么不跟 1.5、§7 记升级重启后 komari.db-wal 偶发 unlink 导致外部读到旧数据（已排除外部只读连接与升级代码，处置=重启一次服务）；同时修正 README 错写的文件管理（我们这条血统没有）。

### Git Commits

| Hash | Message |
|------|---------|
| `3ab871e` | fix(agent): agent 退回 1.4.3 同期代码，去掉 1.5 时代的行为；发 0.0.5 |
| `7b61d17` | fix(guard): 禁止 agent pin 晚于服务器基线；记录升级后 WAL 偶发 unlink 现象 |

### Status

[OK] **Completed**


## Session 10: Session 9: 前端与 agent 源码全部 vendor 进仓库（完全自有）
<!-- trellis-session: v=2 fp=5986615f78c69f45 -->

**Date**: 2026-09-16
**Task**: Session 9: 前端与 agent 源码全部 vendor 进仓库（完全自有）
**Branch**: `main`

### Summary

用户问“服务端和客户端现在都是我们自己维护的吧”，我给出精确回答（服务器源码在仓库内；前端/agent 此前是 pin+补丁的上游依赖），并按其选择把两个上游源码都搬进仓库做成完全自有。(1) agent：快照导入 74 个文件到 agent/（上游 komari-agent@1186aafb + 我们三个补丁的改动内联），build-agent.sh 重写为本地构建（去掉 clone/patch/源码树哈希/补丁回放比对，保留并强化资产过滤+自更新目标+安装脚本版本一致性三道门禁，新增 -buildvcs=false），agent-pin.env → agent-build.env。(2) 前端：快照导入 467 个文件到 frontend/（komari-web@4a74e8a8 + 六个补丁内联，排除上游 .github/），sync-frontend.sh → build-frontend.sh（本地 npm ci+build，不再依赖 git，新增“不得再出现 komari-monitor/komari-agent”检查），frontend-pin.env → frontend-build.env；顺手修掉上游 .gitignore 忽略 package-lock.json 的坑。(3) 等价性证明：agent 老路径与新路径在相同 flags 下产物逐字节一致，且老路径用默认 buildvcs 能复现 0.0.5 release 资产（证明导入零改动，唯一差异来自 buildvcs）；前端重建后目录树哈希仍为 af0bd793…，产物 git 无改动。(4) 文档/规范/README 全量同步：源码 vendor 化后的固定点表、构建脚本名、门禁含义、回滚路径、“不要重新引入 clone+打补丁”的禁令。本次改造不产生新 release（产物未变）。

### Git Commits

| Hash | Message |
|------|---------|
| `bfb96ad` | chore(vendor): import komari-agent@1186aafb as agent/, 打补丁改为直接改源码 |
| `ea28dc0` | chore(vendor): import komari-web@4a74e8a8 as frontend/, 补丁内联进源码 |
| `7759a77` | docs: 源码 vendor 化后的文档、规范与 README 同步 |

### Status

[OK] **Completed**
