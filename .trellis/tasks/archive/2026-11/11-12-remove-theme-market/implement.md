# 实施记录：阉割主题商店

## 步骤

- [x] 后端删除：`web/api/admin/theme_market.go` + `theme_market_test.go` + `theme_market_ssrf_test.go`；
      `web/router/router.go` 移除 6 条 `/theme/market/*` 路由；
      `internal/config/settings.go` 移除 `ThemeMarketSourcesKey`；`git rm -r deploy/theme-market`
- [x] 前端删除：`frontend/src/pages/admin/market/`（页面）；`routes.ts` 市场路由；
      `config/menuConfig.json` 侧栏市场入口；`pages/admin/themes.tsx` 的「find more → 市场」链接
- [x] 命名清理：`isValidMarketShort` → `isValidThemeShort`（含测试）；`blockPrivateEndpoints` 注释去掉市场源
- [x] 重建：`./scripts/build-frontend.sh`（更新 `FRONTEND_TREE_SHA256=269c4df4…`）→
      `sync:admin`（ink 主题 admin-app 同步）→ `./build.sh --skip-server --skip-install`（主题 zip + 内置包刷新）
- [x] 文档：README、MAINTAINING、ops-docs/VERSION、INDEPENDENCE、BUILD、spec（build-and-pinning/quality-guidelines）
- [x] 本机实例部署新二进制（hash 62d18b5）并实测后台

## 验证证据（2026-11-12 本机实测）

| 项 | 命令/入口 | 结果 |
|---|---|---|
| Go 门禁 | `go build ./...` / `go vet` / `go test ./web/api/admin/` | 全绿 |
| 市场 API | `GET /api/admin/theme/market/catalog` | 返回 SPA 兜底 HTML（路由已删，非 JSON） |
| 主题设置 API | `POST /api/admin/theme/settings?theme=ink` | `{"status":"error","message":"Unauthorized."}`（路由仍在） |
| 后台 UI | 登录后 `/admin/themes` | 侧栏只有 Theme Management / 主题设置；无市场入口/「find more」链接（截图 `/root/komari-local/screenshots/admin-themes-no-market.png`） |
| 市场旧地址 | `/admin/market/themes` | 前端路由已删（404 页面） |
| 前端产物 | `check-repo` 第 6 项 | 哈希 = `269c4df4…`，与 `frontend-build.env` 一致 |
| 内置主题包 | `check-repo` 第 17 项 | 3.5M，761 文件，版本与 `themes/ink` 一致；产物内无市场 chunk |
| 仓库自检 | `./scripts/check-repo.sh --full` | **17 项全部通过**（第 16 项渲染层回归也通过 89/89） |

## 回滚

- 代码：`git revert` 本次提交即可（前端产物/内置包同属一个提交）。
- 本机实例二进制备份：`/opt/komari/komari.backup.20261112_061008`。
