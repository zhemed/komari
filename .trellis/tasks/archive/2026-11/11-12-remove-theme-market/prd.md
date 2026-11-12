# 阉割主题商店（后端市场 API/目录 + 后台 UI 入口）

## Goal

ink 已内置为唯一主题（见 `11-12-builtin-theme-embed`），主题商店失去意义：
移除主题市场的后端 API/路由/配置/自有目录，以及后台 UI 的市场页面与菜单入口；
保留主题管理（上传/导入/激活/设置）与主题更新通道（`theme-latest`）。

## 背景与已确认事实

- 用户 2026-11-12 定稿：「选了 C 还可以把主题商店直接阉割掉」→ 先做方案 1（内置主题 + 阉割商店）。
- 现状：后端 `web/api/admin/theme_market.go`（509 行 + 2 个测试）、`/api/admin/theme/market/*` 6 条路由、
  `configs.theme_market_sources` 配置键、`deploy/theme-market/v1.json` 自有目录；
  后台 UI：`frontend/src/pages/admin/market/themes.tsx` 页面、`routes.ts` 路由、
  `config/menuConfig.json` 侧栏入口、`pages/admin/themes.tsx` 的「find more → 市场」链接。
- 后台 UI 是 `frontend/` 构建产物（`web/public/defaultTheme/`，有哈希门禁）；
  ink 主题的 `public/admin-app/` 是该构建的同步副本（`sync:admin`），需要一起刷新。

## Requirements

- **R1 后端移除**：删除 `theme_market.go` 及其测试；移除 6 条市场路由；
  删除 `ThemeMarketSourcesKey` 配置键；删除 `deploy/theme-market/` 目录。
- **R2 前端移除**：删除市场页面/路由/侧栏菜单项/主题页「find more」链接；
  顺手把 `isValidMarketShort` 更名为 `isValidThemeShort`（市场概念已不存在）并清理相关注释。
- **R3 产物重建**：重建 `web/public/defaultTheme/`（更新 `FRONTEND_TREE_SHA256`）；
  `sync:admin` 同步进 ink 主题；`build.sh --skip-server --skip-install` 重建主题并刷新内置包。
- **R4 文档同步**：README、MAINTAINING、ops-docs（VERSION/INDEPENDENCE/BUILD）、
  spec（build-and-pinning/quality-guidelines）。
- **R5 验证**：市场 API 路由消失（返回 SPA 兜底 HTML，非 JSON）；后台实测无市场入口/页面；
  主题管理/主题设置/主题更新仍可用；`check-repo.sh --full` 17 项全绿。

## Acceptance Criteria

- [x] **AC1**：`/api/admin/theme/market/*` 不再是 JSON API（返回 SPA HTML）；`theme_market.go` 等文件已删。
- [x] **AC2**：后台侧栏 Theme 下只有「Theme Management / 主题设置」；主题页无「find more → 市场」；
      `/admin/market/themes` 已是 404 页面（路由删除）。
- [x] **AC3**：主题设置接口仍存在（未登录返回 `Unauthorized.`）；主题管理页可正常打开（截图）。
- [x] **AC4**：前端产物哈希与 `frontend-build.env` 一致；内置主题包已刷新（761 文件，无市场 chunk）。
- [x] **AC5**：`./scripts/check-repo.sh --full` 17 项全通过。
- [x] **AC6**：提交带 `[task:remove-theme-market]`；生产未动。

## 非目标

- 不删除主题上传/导入/激活/更新能力（仍保留）。
- 不清除 `frontend/src/i18n/locales/` 里已无引用的 `market.*` 文案键（死字符串，无行为影响）。
- 不动生产实例。
