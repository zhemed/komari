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


## Session 11: Session 10: 全面维护——说明口径换成我们自己的版本 + 新增仓库自检脚本
<!-- trellis-session: v=2 fp=f6b0c2a310db4165 -->

**Date**: 2026-09-16
**Task**: Session 10: 全面维护——说明口径换成我们自己的版本 + 新增仓库自检脚本
**Branch**: `main`

### Summary

用户要求全面维护 komari 仓库、把说明文件换成我们自己的版本，并点名了 GitHub 上那段 'Komari 1.4.3 self-maintained fork: pinned version, vendored frontend...' 描述。(1) 元信息：描述改为「自维护服务器监控：单二进制服务端 + 内嵌面板 + 14 平台 agent；源码全在本仓库，可离线构建」，补 8 个 topics；homepage 保持空（与其仓库一致）；确认许可识别为 MIT（gh repo view 的 licenseInfo 字段为空是映射问题，REST 里 spdx_id=MIT）。(2) 口径：README「本 fork 的取舍」→「这个版本与上游的差异」并改为“由 zhemed 独立维护的发行版”；MAINTAINING 开头同改；安装/构建脚本 5 处“本 fork”→“本仓库”，安装界面 (komari x fork) → (zhemed/komari x)；LICENSE 保留上游行并补我们的改动版权行。(3) 新增 scripts/check-repo.sh：快速 7 项（版本字面量、文档引用+file:line 锚点越界、脚本语法、脏文件、密钥扫描、前端产物哈希、构建期无克隆上游）+ --full 追加 Go 门禁/离线构建/agent 三道门禁；接进 MAINTAINING §2.1 与 §3.4 发布清单、规范 §2。(4) 用它扫出并修掉 9 处真实不一致：6 个规范文件仍有 0.0.3 已删除的 pkg/jsruntime、utils/messageSender 残留（含“保留物”“无测试目录”“panic 场合”“模块表两行”“Debug 用法”），三处数字漂移（测试分布改为实测 77 个测试文件、defaultTheme 受控文件 442→431、Debug 用法 1 处→0 处），一处 file:line 锚点漂移（migrations.go:417/420 → 315/318，文件只 387 行）。理念：删代码必须同时扫文档里的引用与数字，现在由脚本兜底。

### Git Commits

| Hash | Message |
|------|---------|
| `8cc30ef` | docs: 说明口径改成我们自己的版本，LICENSE 补上改动版权行 |
| `1ca69c4` | feat(tools): 新增 scripts/check-repo.sh 仓库自检，并接进发布清单 |
| `f1186a2` | docs(spec): 清掉体检抓出的过期规范（已删除模块与漂移数字） |

### Status

[OK] **Completed**


## Session 12: Session 11: 流量历史跨重启保存（服务端持久累计）+ 修 v2 上报缺 uptime 的增量清零
<!-- trellis-session: v=2 fp=7879ea70a1193509 -->

> ⚠ **2026-09-17 更正（后补，不重写原文）**：本节的因果结论有两处不成立——
> (1) "个别分钟流量增量偏低"**不是**双通道交错上报造成的（agent 同一时刻只走一条通道；
> 实测记录侧无丢失：636 个分钟逐分钟零误差、全窗口差 336 B；面板看到的偏低来自查询端 `avg`
> 聚合把点值除以桶内采样条数，已在 0.0.7 修复）；
> (2) `uptime` 缺陷**真实存在但从未被证实触发过**，当时写成"确因"属过度断言。
> 详见 Session 15 与 `.trellis/spec/guides/evidence-and-claims-guide.md`。

**Date**: 2026-09-17
**Task**: Session 11: 流量历史跨重启保存（服务端持久累计）+ 修 v2 上报缺 uptime 的增量清零
**Branch**: `main`

### Summary

用户报“重启后流量历史不能保存”。根因实测确认：面板总流量=agent 开机计数器(/proc/net/dev)，机器重启必归零；agent 的落盘统计 net_static.json 只在 --month-rotate 非 0 时启用（默认没开，文件都不存在）；服务端 metric store 的增量跨重启保留但只喂后台 24h 图。按用户选择做服务端持久累计：新增 client_traffic_totals 表与仓储层（内存快照+SQLite，启动加载）、metricstore 批次写入钩子回传重置感知增量（不反向依赖 database/*）、首次见节点用当前计数器做基线、面板 getNodesLatestStatus 改读累计值（卡片总流量与阈值进度随之跨重启可用）、删除节点联动清理。不改 agent → 所有已部署节点立即生效。验证：单测 5 个（含计数器归零不回退、重启重加载）；本地实测累计与 metric store 增量之和差 18KB≈一个上报周期，重启 agent/服务端期间单调不减，面板 1.88GiB/681MB 与库中一致。顺带修掉实测抓到的真 bug：v2 报告没有 uptime 字段（读到 0），而重启判定用 uptime 回退，v1/v2 双通道切换时把该条上报增量清零——判据改为两次都需有效 uptime + 回归测试。残留：双通道上报导致个别分钟增量仍偏低（已记入 MAINTAINING §7，需加临时日志才能定性，后续在“v2 活跃时忽略 v1 指标”与“agent 只走一条通道”间选一个）。

### Git Commits

| Hash | Message |
|------|---------|
| `10e5605` | feat(traffic): 流量累计跨重启保存；修正 v2 上报缺 uptime 导致的增量清零 |
| `df4f08f` | docs(traffic): 记录流量累计的设计与残留噪声；check-repo 排除自身误报 |

### Status

[OK] **Completed**


## Session 13: 发布 0.0.6（流量跨重启）+ 本机部署改用 release 资产验证
<!-- trellis-session: v=2 fp=cf53dfc490cabd44 -->

**Date**: 2026-09-17
**Task**: 发布 0.0.6（流量跨重启）+ 本机部署改用 release 资产验证
**Branch**: `main`

### Summary

把流量持久累计这一轮维护收口成正式发布，并把本机部署从本地自建二进制切换为‘用户能下载到的那些 release 资产’，做到已发布=正在跑。版本线 0.0.5 → 0.0.6（四处字面量），提交后打 tag，服务端 amd64/arm64 静态资产 + agent 14 平台资产 + komari-agent-SHA256SUMS 共 17 个资产挂到 0.0.6 release；ghcr 推 zhemed/komari 与 zhemed/komari-agent 的 0.0.6/latest 四标签并匿名校验可拉取。随后按用户路径重装本机：服务器走 install-komari.sh 升级（下载产物 sha256 与本地构建产物逐字节一致，版本行 0.0.6 (hash: 7cbd25d…)），agent 走 install-agent.sh 并复用 auto-discovery 身份（版本 0.0.6，UUID 不变）。验证：DB client_traffic_totals 在服务端重启后仍在且继续增长，面板 getNodesLatestStatus 的 net_total_up/down 与库中数值完全一致（2324.2MB/827.6MB），面板卡片与概览均显示 2.27GB/829.4MB，节点在线、uptime 跟随系统而非被重启清零；/etc/motd 不存在（此前 agent 误写安全提醒的后遗症已清干净）。全量自检唯一失败项是上轮文档里把路径写成花括号合并形式 agent/protocol/{v1,v2,transport} 导致路径校验解析不了，已拆成三条真实路径并单独补一个 docs 提交推送。

### Main Changes

- 版本线 0.0.5 → 0.0.6，提交 7cbd25d 后再打 tag（保证二进制内嵌 hash 与 tag 指向的提交一致）
- release 0.0.6 挂 17 个资产：服务端 linux/amd64+arm64、agent 14 平台、komari-agent-SHA256SUMS
- ghcr 推送 zhemed/komari 与 zhemed/komari-agent 的 0.0.6 + latest，四个标签均匿名可拉取
- 本机部署改用 release 资产重装（服务器+agent），不再跑本地自建二进制，验证已发布=正在跑
- docs(maintaining) §13：把 agent/protocol/{v1,v2,transport} 拆成三条真实路径，修复 check-repo 路径校验失败

### Git Commits

| Hash | Message |
|------|---------|
| `7cbd25d` | chore(release): 版本线 0.0.5 → 0.0.6 |
| `5b5f5c5` | docs(maintaining): 修正 §13 协议实现路径写法，让 check-repo 路径校验可解析 |

### Testing

- [OK] 服务器/IP 二进制与 release 资产 sha256 逐字节一致；面板页脚显示 0.0.6 (7cbd25d)
- [OK] agent 已部署二进制 sha256 与发布清单 SHA256SUMS 中 komari-agent-linux-amd64 一行完全一致
- [OK] client_traffic_totals 跨升级重启存活并继续增长；面板 net_total_up/down 与库中数值一致
- [OK] check-repo.sh --full 九项除文档路径外全绿，修复后快速自检全部通过

### Status

[OK] **Completed**

### Next Steps

- 残留个分钟级流量增量噪声（60s 桶压成 300s）需要临时逐条上报日志才能定性，属独立小课题
- 面板未显示每节点协议版本（v1/v2），可选加一个字段方便判断节点走的哪条通道


## Session 14: 定位并修复流量图点值被除以采样条数（发布 0.0.7）
<!-- trellis-session: v=2 fp=ba1591e9ebd80471 -->

**Date**: 2026-09-17
**Task**: 定位并修复流量图点值被除以采样条数（发布 0.0.7）
**Branch**: `main`

### Summary

用户报面板流量图个别分钟偏低/偶发 2×。按先取证据再改语义的顺序查：用 60s 桶端点对齐逐分钟核对（本桶 Σ增量 vs 相邻桶计数器差值），636 个分钟逐分钟零误差、10.5 小时全窗口只差 336 B、计数器零回退——记录侧没有丢数；同时更正了 MAINTAINING §7 里先前那条错误观测（05:38 只记到 5.5 KB 而计数器涨 190 KB，实为混着 60s/300s 两种分辨率比出来的假象，实测两边都是 5,554 B）。真正根因在查询端：queryMetrics 对 traffic.up/down 采用客户端全局聚合偏好（面板默认 avg），而这两个指标每个采样点是两次上报之间的字节数，取平均等于再除以桶内采样条数——实测 60/60 分钟的 真实值÷点值 精确等于采样条数（20），条数不齐时缩放比变化即表现为噪声。修法：把 records 路径早就存在的按指标语义聚合约定抽成唯一来源 metricstore.SemanticAggregation，queryMetrics 优先级改为 按指标显式指定 > 语义默认 > 全局指定 > avg（traffic→sum、net.total→last）。验证不改线上：复制 data/ 到备用端口跑修复版，与线上 0.0.6 对拍面板同款参数——线上点值恰为真实值 1/20，修复版逐分钟精确相等。随后发布 0.0.7：版本线四处字面量+文档标题，提交 fc7d6e8 后构建（服务端 2 平台静态 + agent 14 平台 + SHA256SUMS 共 17 资产）→ 打 tag → gh release → 两个镜像四标签推 ghcr 并匿名校验（2/2/3/3 架构）；本机部署随即从 release 资产升级，服务器二进制与资产 sha256 逐字节一致（2cd36d38…，版本行 0.0.7 hash fc7d6e8），agent 0.0.7 复用原 UUID，实测修复生效：面板同款请求 5/5 分钟点值与库中真实量完全一致（含 count=19 的分钟），累计表跨升级继续增长到 2731.3MB/909.8MB。顺带修掉发版流程里的一个坑：check-repo --full 的 agent 门禁默认调用 build-agent.sh（该脚本会 rm -rf dist/agent），按 §3.4 的顺序跑会把刚建好的 14 个 agent 资产冲成 1 个——已改为构建到 .build/check-agent，并把 §3.4 第 6 步漏掉的 komari-agent-SHA256SUMS 补进清单。

### Main Changes

- metricstore.SemanticAggregation 成为指标语义聚合的唯一来源；queryMetrics 优先级改为 按指标显式 > 语义默认 > 全局 > avg
- 新增测试：internal/metricstore/semantic_aggregation_test.go、public_metric_test.go 的 TestResolveMetricAggregationPrecedence
- 文档：MAINTAINING §7 换成实测定位过程并更正旧错误观测；新增 spec/backend/metric-query-aggregation.md
- check-repo 的 agent 门禁改用 .build/check-agent，不再冲掉 dist/ 发版资产；§3.4 资产清单补全
- 发布 0.0.7：17 个资产 + ghcr 两个镜像四标签（匿名可拉取）

### Git Commits

| Hash | Message |
|------|---------|
| `48dca2e` | fix(metrics): traffic.up/down 查询按语义求和，修面板流量图点值被除以采样条数 |
| `fc7d6e8` | chore(release): 版本线 0.0.6 → 0.0.7（流量查询聚合修复） |
| `2109504` | fix(scripts): check-repo 的 agent 门禁改用 .build/check-agent，避免冲掉 dist/ 发版资产 |

### Testing

- [OK] 逐分钟对拍 636/636 分钟零误差；全窗口差额 +336 B；累计表与记录增量差 -0.33%（端点对齐精度内）
- [OK] 备用端口对拍：线上点值=真实值÷20，修复版逐分钟精确等于真实值；300s 桶=五分钟之和
- [OK] 升级后实测：版本 0.0.7/fc7d6e8、二进制与资产 sha256 一致、面板同款请求 5/5 分钟精确一致、节点在线（时延 2.3s）、累计表继续增长
- [OK] check-repo.sh --full 十项全绿，且 dist/agent 14 个产物完好（验证修好的门禁不再冲产物）

### Status

[OK] **Completed**

### Next Steps

- 面板不显示每节点协议版本（服务端内存里已有 protocolVersion，展示起来不大）
- 面板流量图两档分辨率（60s/300s）的点值量级差 5 倍；若要同量纲，需前端按响应 interval_seconds 归一化成速率


## Session 15: 补建 Trellis 任务 + 更正未验证因果结论（流程违规整改）
<!-- trellis-session: v=2 fp=cd832412eaee5ac2 -->

**Date**: 2026-09-17
**Task**: 补建 Trellis 任务 + 更正未验证因果结论（流程违规整改）
**Branch**: `main`

### Summary

用户当场判定两类严重错误：(1) 本轮（流量噪声排查→0.0.7 修复→发版）全程未调用 Trellis——没有 trellis-start/continue、没有 task.py、没有 PRD/design/implement、没有 trellis-check、没有 finish；只在事后补了 journal 与 spec，并被指出后以"小改动走直接改路线"淡化；(2) 把未经验证的因果结论当成"已定位的原因"写进了公开的 0.0.6 发布说明、MAINTAINING §7/§13.3、Go 代码注释与归档 PRD。整改：先按 break-loop 框架定性（E1 流程缺失是根因、E2 结论可信性是症状、E3 沟通淡化），再全落点带日期更正——代码注释改为带证据边界表述；MAINTAINING 把"确因"降级为"真实存在但从未被证实触发"；§3.4 第 1 步新增发布说明措辞门禁；backend/index.md 的 Pre-Development Checklist 指向证据指南；归档 PRD 与 journal Session 12 追加更正块；本地与**公开的 GitHub release 0.0.6 说明**追加更正段（原文保留，另存 backup 于 .build/rel-notes-0.0.6.published-backup.md）。新增 .trellis/spec/guides/evidence-and-claims-guide.md（观测 vs 推断、禁用措辞、判别性实验、结论被推翻时的全落点更正清单）并登记进 guides/index 与 backend/index。流程补齐：建并走完两个任务——09-17-traffic-query-aggregation-0-0-7（补登 0.0.7 修复，含补跑的验收证据：release 17 资产、镜像 2/3 架构、本机二进制与资产一致、面板同款请求 60/60 分钟点值精确等于库中真实量）与 09-17-trellis-process-and-claims-remediation（本次整改，prd/design/implement + implement.jsonl/check.jsonl + validate + start + 归档）。验证：残留特征词 grep 仅剩更正说明与指南禁用清单；gofmt 通过；go build/vet/test 全绿；check-repo --full 十项全绿。

### Main Changes

- 全落点更正未验证结论：report_batcher.go / report_test.go 注释、MAINTAINING §7 与 §13.3、归档 PRD、journal Session 12、.build/rel-notes-0.0.6.md、公开的 GitHub release 0.0.6 说明
- 新增 .trellis/spec/guides/evidence-and-claims-guide.md + 在两处索引登记；MAINTAINING §3.4 第 1 步加发布说明措辞门禁
- 补建并归档两个 Trellis 任务（0.0.7 补登、流程与可信性整改），均含 prd/design/implement 与上下文清单
- 0.0.7 补登任务补跑验收：release 17 资产、镜像匿名叫架构 2/2/3/3、本机 0.0.7 与资产 sha256 一致、面板同款请求 60/60 分钟精确一致

### Git Commits

| Hash | Message |
|------|---------|
| `245bb91` | fix(process): 更正未验证因果结论的全落点 + 补建 Trellis 任务记录 |
| `ec3deb1` | chore(task): 记录整改任务的验收勾选（prd/implement） |

### Testing

- [OK] 残留特征词复查：grep 命中项全部属于更正说明或指南禁用清单，无未更正的原始断言
- [OK] gofmt（改动的两个 Go 文件）通过；go build ./...、go vet、metricstore+jsonrpc 测试全绿
- [OK] check-repo.sh --full 十项全绿（含新的发版门禁与文档路径校验）
- [OK] gh release view 0.0.6 正文含 1 段「更正（2026-09-17」，原文完整保留

### Status

[OK] **Completed**

### Next Steps

- 把发布说明纳入版本管理（当前 .build/ 被 gitignore，公开说明的本地副本不受版本控制；建议 docs/releases/<版本>.md）
- 可选：check-repo 增加 --claims 开关，人工触发特征词扫描（当前不作为默认门禁，避免误报疲劳）


## Session 16: 面板一键升级服务器（0.0.8 功能 + 0.0.9 修 E2E 抓到的缺陷）
<!-- trellis-session: v=2 fp=a9f39b17243c9cf6 -->

**Date**: 2026-09-17
**Task**: 面板一键升级服务器（0.0.8 功能 + 0.0.9 修 E2E 抓到的缺陷）
**Branch**: `main`

### Summary

按批准的三项决策（范围=升到最新+指定版本/回滚；校验=SHA256+发版补服务端校验和资产；失败=不做自动回滚）实现面板一键升级。服务端新增 internal/upgrade（纯标准库，不引第三方自更新库）：Prepare 先判形态（容器→只给 pull 命令；无 systemd→仅下载；否则要求目录可写）再取 komari-SHA256SUMS；Execute 下载(流式 SHA256)→自检(新二进制 --help 输出含 Komari Monitor <tag>)→备份+原子替换；状态写 <二进制目录>/upgrade-state.json，Reconcile 把 restarting 收敛为 completed。RPC 四个（getServerUpgradeSettings/listServerReleases/upgradeServer/upgradeStatus），设置用扁平键 server_upgrade_enabled/server_update_repo 并在通用设置接口加校验钩子挡住非法仓库名。发版配套：新增 scripts/gen-release-sums.sh（资产 17→18），install-komari.sh 加尽力而为校验（旧版本无该资产时告警跳过，保住回滚路径）。前端：弹窗内立即升级/安装此版本/复制 pull 命令+进度与失败信息，系统设置页加开关与来源仓库，5 语言各 22 键，前端产物重建并更新 FRONTEND_TREE_SHA256（af0bd793→25e318e2）。E2E 实测（面板接口驱动）抓到真缺陷：Execute 下载后未补执行位就自检，fork/exec permission denied，导致 0.0.8 的一键升级必然失败（替换在自检之后，实测确认不破坏线上：版本与数据均未变）——修于 0.0.9 并加回归测试（注入探针断言被自检文件可执行，去掉修复即 FAIL，已验证不是永真测试），同时在 0.0.8 的公开发布说明追加更正段、MAINTAINING §14.4.1 记录窗口期绕行办法。实测覆盖：面板降级 0.0.9→0.0.7 与 0.0.9→0.0.8（二进制与 release 资产逐字节一致、数据单调不减、生成 komari.backup.<旧版本>）、已是最新分支返回明确提示、缺校验和资产被拒并提示用安装脚本、关闭开关后接口返回 PermissionDenied、非法仓库名被设置接口拒绝、审计日志 logs 表出现升级记录、清空 api_key 后旧凭据被拒。顺带给 0.0.7 的 release 补 komari-SHA256SUMS（按当时发布的二进制计算，哈希与发布记录一致），使回滚到 0.0.7 也可用。未覆盖并如实标注：真有更高版本时的"升到最新"点击路径（本轮 0.0.9 已是最新，只能验到 ErrUpToDate；与指定版本共用 Execute，差异在选择目标且有单测）、真实容器场景（仅单测）。

### Main Changes

- internal/upgrade/*：releases/download/install/state/upgrade 五个文件 + 单测；语义边界写进代码注释
- web/rpc/jsonrpc/admin.upgrade.go：四个 admin RPC + 审计 + 退出口；validateUpgradeSettingChanges 接入 admin:editSettings
- scripts/gen-release-sums.sh 新增；install-komari.sh 加 best-effort 校验（含 log_warn 助手）
- 前端：AdminPanelBar 弹窗按钮与状态机、设置页开关与仓库、5 语言 22 键、前端哈希更新
- 文档：MAINTAINING §14（能力/支持矩阵/不变量/救援命令/安全边界/0.0.8 缺陷记录）、§3.4 资产清单、README、spec/backend/server-upgrade.md
- 发布 0.0.8（功能）与 0.0.9（修复），各 18 个资产 + ghcr 两个镜像四标签

### Git Commits

| Hash | Message |
|------|---------|
| `c671364` | feat(upgrade): 面板一键升级服务器（含指定版本/回滚） |
| `6421ec9` | fix(upgrade): 自检前先补执行位，修 0.0.8 一键升级必然失败 |
| `8dea6a6` | docs(task): 记录一键升级的端到端实测结果与未覆盖项 |

### Testing

- [OK] E2E：面板降级 0.0.9→0.0.7 / 0.0.9→0.0.8 成功且哈希一致；数据单调不减；备份文件生成；审计日志有记录
- [OK] E2E 反例：0.0.6（缺校验和）被拒并给出可操作提示；已是最新返回明确提示；关闭开关后 PermissionDenied；非法仓库名被拒
- [OK] 单测：校验和不符/自检版本行不符均不替换原二进制；执行位回归测试去掉修复即 FAIL
- [OK] check-repo.sh --full 十项全绿；本机最终运行 0.0.9（已发布=正在跑），API Key 测试凭据已清空

### Status

[OK] **Completed**

### Next Steps

- 下一次真实发版时验证"升到最新"的点击路径（本轮 0.0.9 已是最新，只能验 ErrUpToDate 分支），并补记到任务/规范
- 可选：给升级接口加 2FA（rpc.MarkSensitive）需要前端补二次验证提示流程；或加签名校验（minisign/GPG）


## Session 17: 容器形态暴露可复制的升级命令（发布 0.0.10）
<!-- trellis-session: v=2 fp=93bbcc7122f83f9f -->

**Date**: 2026-09-17
**Task**: 容器形态暴露可复制的升级命令（发布 0.0.10）
**Branch**: `main`

### Summary

用户问"Docker 部署升级到 0.0.9 后能否在网页升级"，回答时发现 0.0.9 的实现缺口：容器形态下 supported=false，取回 pull_command 的唯一入口（升级按钮）被同一条件隐藏，用户只看得到"不支持"的提示、拿不到命令——与服务端能力（真实容器已验证返回 manual:true + pull_command）不一致。修法：弹窗底部与每条 release 行在"不支持一键升级但可拿到命令"时改为"复制升级命令"，复用同一接口与展示块；5 语言加 copy_pull_command；前端产物重建并更新 FRONTEND_TREE_SHA256（25e318e2→abd793b3）；版本线 0.0.9→0.0.10 并发布（18 资产 + 两个镜像）。实测：真实容器里用浏览器完成完整点击路径（弹窗显示 6 个"复制升级命令"，点击后出现容器提示 + docker pull ghcr.io/zhemed/komari:0.0.10 + 复制命令按钮，截图留证）；顺带把之前标为"未覆盖"的升到最新路径真实跑通（生产 0.0.9→0.0.10，二进制与 release 资产逐字节一致，审计日志有记录）。过程中还实测了容器升级形态本身：0.0.5 容器 → 0.0.9（pull + 重建）数据完整（节点/累计流量/指标），服务端自动生成 data/backup/upgrade-*.zip；并把该步骤写进 MAINTAINING §14.4.2。另记录一处未修的观感问题：未登录时按钮仍渲染（状态为 null 时条件求值为真），点击得到 Permission denied，已写入 §7 待办。踩坑记录：批量替换版本字面量时因匹配串写错中断、只改了一半就提交，被 check-repo 抓到 4 项不一致后补齐（b976210）。

### Main Changes

- 弹窗底部与每条 release 行：不支持一键升级时改为"复制升级命令"，复用 admin:upgradeServer 的 manual 返回与展示块
- 5 语言加 upgrade.copy_pull_command；前端产物重建并更新 FRONTEND_TREE_SHA256
- 0.0.10 发布：18 资产 + ghcr 两个镜像四标签；MAINTAINING §14.4.2 写清容器升级步骤

### Git Commits

| Hash | Message |
|------|---------|
| `f24b743` | fix(ui): 容器/无 systemd 形态下暴露可复制的升级命令（0.0.10） |
| `b976210` | chore(release): 补齐 0.0.10 版本字面量 |

### Testing

- [OK] 真实容器浏览器实操：复制升级命令 → 容器提示 + docker pull ghcr.io/zhemed/komari:0.0.10 + 复制按钮（截图）
- [OK] 升到最新路径真实跑通：生产 0.0.9 → 0.0.10，二进制与 release 资产一致，服务 active，审计有记录
- [OK] 容器 0.0.5 → 0.0.9（pull + 重建）数据完整 + 服务端自动备份 data/backup/upgrade-*.zip
- [OK] check-repo.sh --full 全绿；生产最终运行 0.0.10（= 最新发布），测试凭据已清空，测试容器与临时文件已清理

### Status

[OK] **Completed**

### Next Steps

- 收紧按钮条件为 upgradeStatus && upgradeStatus.enabled !== false（未登录时不该渲染按钮），下次发版带上


## Session 18: 容器一键升级：docker socket + helper 重建容器（0.0.11 引入，0.0.12 修缺陷）
<!-- trellis-session: v=2 fp=2a984fa4bd243aba -->

**Date**: 2026-09-17
**Task**: 容器一键升级：docker socket + helper 重建容器（0.0.11 引入，0.0.12 修缺陷）
**Branch**: `main`

### Summary

用户明确要求容器部署也能在网页里一键升级，并选定"挂 docker.sock + 面板拉镜像重建自身容器"（方案 B），同时指出我此前把"容器不能自升级"说成架构限制是错的——那只是取舍，已在文档更正。实现：新增 internal/dockerapi（标准库 + unix socket 直连 Engine API：Ping/版本协商、拉镜像含 200 里的 error 与流式进度、inspect、改名、建/起/停/删/等容器、日志解复用、ListHelpers），Recreate 实现"预检 → 旧容器改名 → 建同名新容器（逐字段沿用旧 Config/HostConfig/网络，仅换镜像）→ 停旧 → 起新"与全路径回滚；新增 helper 子命令 komari docker-self-recreate（跑在独立容器里，也可手工用于恢复）；internal/upgrade 增加 Mode（binary/docker-recreate/manual/download-only）与 CurrentMode；设置键 server_upgrade_docker_socket；RPC 暴露 mode/detail/image/digest 且 helper 接管时不自行退出；前端按 mode 切换文案（立即升级（重建容器））并加 socket 权限提示、顺带修掉"未登录也渲染按钮"的观感问题、5 语言补 2 键。E2E 抓到 0.0.11 的必然缺陷：helper 用目标镜像启动，而 helper 子命令是 0.0.11 才有的——降级到更早版本时 helper 秒退（docker events: create→start→die→destroy），又因 AutoRemove=true 现场被抹掉，父容器卡在 restarting 直到超时；0.0.12 修为：helper 用当前镜像、不自动删除并命名+打标签保留现场（下次升级前统一清理）、父进程监视 helper 退出并把日志尾部写状态回报面板。实测（真实容器 + 真实 socket）：0.0.12 → 0.0.11 重建约 4 秒完成；重建前后容器名/卷/端口/restart/网络/env/WorkingDir/Cmd 逐项一致，Hostname 正确重新分配；数据完整（节点、累计流量、rollups）；审计有记录；再升回 0.0.12 成功；失败注入（不存在的 tag）被拒且容器未动、服务 200；未挂 socket 回落 manual 返回可复制命令（无回归）。发布 0.0.11 与 0.0.12（各 18 资产 + 两个镜像），本机生产升到 0.0.12 且二进制与 release 资产一致；0.0.11 的公开发布说明已追加更正段。

### Main Changes

- internal/dockerapi：Engine API 最小客户端 + Recreate（含回滚）+ 单测（unix socket 上的假 daemon，含 create/start 失败两条回滚路径、预检失败不动容器、no-op）
- cmd/docker-self-recreate：helper 子命令；internal/upgrade：Mode/CurrentMode、docker-recreate 分支、helper 监视与清理
- 设置键 server_upgrade_docker_socket；RPC 暴露 mode/detail/image/digest；helper 接管时不退出进程
- 前端 mode 感知文案 + socket 权限提示 + 状态未知不渲染按钮；5 语言 25 键；产物重建与哈希更新
- 文档：MAINTAINING §14.6（容器一键升级与安全边界）与 §14.6.1（0.0.11 缺陷记录）、§14.2 支持矩阵更正、README、spec/backend/server-upgrade.md

### Git Commits

| Hash | Message |
|------|---------|
| `c663d96` | feat(upgrade): 容器部署也能一键升级（docker socket + helper 重建容器） |
| `c434af7` | fix(upgrade): 容器升级 helper 改用当前镜像 + 保留现场 + 监视退出（0.0.12） |

### Testing

- [OK] 真实容器 E2E：0.0.12→0.0.11 与 0.0.11→0.0.12 各约 4 秒完成；配置逐项一致；数据完整；审计有记录
- [OK] 失败注入：不存在的 tag 被拒、容器 ID 未变、服务仍 200
- [OK] 无 socket 容器：mode=manual、supported=false、返回 docker pull 命令（无回归）
- [OK] dockerapi 单测 73.6% 覆盖 + upgrade 侧 helper 参数/挂载测试；check-repo --full 十项全绿

### Status

[OK] **Completed**

### Next Steps

- 真实容器里的自动回滚注入（需要占端口/容器名，测试不具结论性，单测已覆盖两条路径）
- 可选：容器模式下把 helper 的执行摘要也写进审计日志（当前只有请求记录）


## Session 19: 容器零配置网页升级（0.0.13 引入，0.0.14 修两个缺陷）
<!-- trellis-session: v=2 fp=27c99acc4bb68ccf -->

**Date**: 2026-09-17
**Task**: 容器零配置网页升级（0.0.13 引入，0.0.14 修两个缺陷）
**Branch**: `main`

### Summary

用户明确表态：他给的任务只有"网页端升级"，而我此前把容器形态做成了"要么挂 docker.sock、要么只给命令"，都不满足需求。本任务补齐真正的零配置路径：容器 + 无 socket + 目录可写 → ModeContainerReplace（下载 release 资产 → 校验 komari-SHA256SUMS → 自检版本行 → 备份 + 原子替换 → syscall.Exec 原地重执行），re-exec 失败回落 exit(42)，只读 rootfs 回落 manual。E2E 发现两个缺陷并在 0.0.14 修复：(1) supported 判定有两处内联表达式漏掉新模式 → 面板不显示按钮（接口层可用，所以命令行实测是通的）；(2) SelfExec 的等值校验写错——二进制替换后 /proc/self/exe 跟随 inode 指向备份文件，导致 exec 永不执行、实际靠 docker restart 策略兜住（没有 restart 策略就升不动）；改为只校验目标文件存在且可执行。最终实测（真实容器、不挂 socket、用户原命令形态）：mode=container-replace、supported=true、0.0.14→0.0.13 约 6 秒、容器 ID 不变、重启次数 0→0、容器内二进制与发布资产 md5 一致、备份生成、数据单调不减、升回 0.0.14 成功。文档更正"必须挂 socket"的过头说法（README + MAINTAINING §14.4.2/§14.6 三条路径对照表）；0.0.13 的公开发布说明追加更正段。发布 0.0.13 与 0.0.14（各 18 资产 + 两个镜像），本机生产升到 0.0.14，check-repo --full 全绿。

### Main Changes

- internal/upgrade：ModeContainerReplace + Plan/Result.InContainer + selfexec.go（syscall.Exec）
- Prepare 容器分支改为三条：挂 socket→重建容器 / 无 socket 且可写→容器内替换 / 不可写→manual
- RPC：InContainer 时先 SelfExec 失败回落 exit；supported 统一走 upgradeSupports()
- 前端：立即升级（容器内替换）文案、原地重启阶段、"重建容器会回退"提示；5 语言各 3 键；产物重建与哈希更新
- 文档：README 与 MAINTAINING §14.4.2/§14.6 更正"必须挂 socket"的说法，给出三条路径对照表

### Git Commits

| Hash | Message |
|------|---------|
| `247aa9f` | feat(upgrade): 容器零配置网页升级（容器内替换二进制 + 原地重执行） |
| `5b478ee` | fix(upgrade): 修 0.0.13 容器零配置升级的两个缺陷（0.0.14） |

### Testing

- [OK] 真实容器（不挂 socket）：0.0.14→0.0.13→0.0.14 往返，容器 ID 不变、RestartCount 0→0、二进制与资产一致、数据完好
- [OK] supported=true / mode=container-replace（面板会显示按钮）；只读回落 manual 有单测
- [OK] check-repo.sh --full 十项全绿；0.0.13 公开说明已更正

### Status

[OK] **Completed**

### Next Steps

- 用户侧只需用 0.0.14 镜像重建一次容器（他的 0.0.5 容器本身没有升级功能），之后即可一键升级


## Session 20: 去掉升级弹窗里的 Github 按钮（0.0.15）
<!-- trellis-session: v=2 fp=fc0d91a52f333f83 -->

**Date**: 2026-09-17
**Task**: 去掉升级弹窗里的 Github 按钮（0.0.15）
**Branch**: `main`

### Summary

用户要求：升级弹窗里的 Github 按钮去掉（升级弹窗只保留真正要做的操作）。改动只在 AdminPanelBar 的"有新版本"弹窗底部——移除指向 latestRelease.html_url 的 Github 按钮；关于页里的仓库链接保持不变（那是仓库信息不是操作入口）。前端产物重建并更新 FRONTEND_TREE_SHA256（c92ceb77→6c158ec6）；版本线 0.0.14→0.0.15 并发布 0.0.15（18 资产 + 两个镜像）。验证：真实容器（不挂 socket）里用浏览器打开弹窗，底部按钮只剩「安装此版本」与「立即升级（容器内替换）到 0.0.15」，截图 .build/shots/no-github-button.png；弹窗正文里仍出现 Github 字样属发布说明文本，不是按钮。用户侧无需任何手工操作——直接在面板点按钮升到 0.0.15 即可。

### Main Changes

- frontend/src/components/admin/AdminPanelBar.tsx：移除升级弹窗底部的 Github 按钮
- 前端产物重建 + FRONTEND_TREE_SHA256 更新；版本线 0.0.15 与发布（含镜像）

### Git Commits

| Hash | Message |
|------|---------|
| `b6bc4d7` | feat(ui): 去掉升级弹窗里的 Github 按钮（0.0.15） |

### Testing

- [OK] 浏览器实测：弹窗按钮集合 = [安装此版本, 立即升级（容器内替换）到 0.0.15]，无 Github 按钮（截图留证）
- [OK] check-repo.sh 全部通过；容器形态 supported=true 仍可一键升级

### Status

[OK] **Completed**


## Session 21: 修"点升级后卡住、必须手动刷新"（0.0.16）
<!-- trellis-session: v=2 fp=734bb4c1cacb822d -->

**Date**: 2026-09-17
**Task**: 修"点升级后卡住、必须手动刷新"（0.0.16）
**Branch**: `main`

### Summary

用户实测反馈：点"立即升级"后弹窗停在"下载新版本中…"、按钮灰着不动，必须手动刷新页面。根因：容器内替换是 execve 替换当前进程，会**切断面板正在使用的那条 RPC2 连接**，而原实现是 await 状态调用 → 永久挂住（既不返回也不报错），于是轮询循环停死、也不会自动刷新；二进制 + systemd 形态的服务重启同样会断连，属同一类问题。修法：① 每次状态调用加 4s 超时；② 新增独立的 /api/version 轮询（普通 HTTP，不依赖会被切断的连接），版本变为目标值即自动刷新页面；③ 失败或 6 分钟超时都恢复按钮可用并给出提示，不再卡在灰按钮。实测（真实容器 + 浏览器，点完不再碰页面）：容器访问日志显示 13:54:49/50/52 有 /api/version 轮询、13:54:53 替换完成、13:54:55 页面自行重新加载 /admin/dashboard + 静态资源 —— 即点一下后约 2 秒页面自动刷新为新版本，全程无手动刷新；截图 .build/shots/upgrade-auto-reload.png。发布 0.0.16（18 资产 + 两个镜像）。

### Main Changes

- frontend/src/components/admin/AdminPanelBar.tsx：pollUpgrade → followUpgrade（每次调用带超时 + 双通道：状态轮询 + 独立版本轮询 + 自动刷新 + 失败/超时恢复按钮）
- 前端产物重建并更新 FRONTEND_TREE_SHA256（6c158ec6→b860e30d）；版本线 0.0.15→0.0.16 并发布（含镜像）

### Git Commits

| Hash | Message |
|------|---------|
| `585f02d` | fix(ui): 升级后自动跟随版本变化，修"卡在下载中、必须手动刷新"（0.0.16） |

### Testing

- [OK] 容器访问日志证据：升级完成后 2 秒内出现 /admin/dashboard 与静态资源的重新加载，未手动刷新
- [OK] 状态轮询带超时后界面不再永久卡住（观测到 downloading→completed 的阶段推进）
- [OK] check-repo.sh 全绿；容器 supported=true、原地替换后容器 ID 不变

### Status

[OK] **Completed**
