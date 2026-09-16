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
