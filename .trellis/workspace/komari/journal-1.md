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
