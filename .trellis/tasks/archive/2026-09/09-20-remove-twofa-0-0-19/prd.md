# 彻底移除 2FA（两层）+ 发 0.0.19

## 用户指令

「建立新任务，查看账号的 2FA，关于相关的所有内容是否可以彻底移除」→ 截图指出**两处**（账户页 + 关于页）
→ 选定「两层一起删 + 发 0.0.19」。

## 移除范围（已全部落地）

**后端**
- 账号级：`database/accounts/2fa.go`、`web/api/admin/2fa.go`、路由 `/api/admin/2fa/{generate,enable,disable}`、
  `login.go` 的验证码分支、`update.go` 改密码时的敏感校验、模型 `User.TwoFactor` 字段
- 敏感操作验证：`pkg/rpc/sensitive.go`、`web/api/AuthSensitive.go`、`transport.go` 的
  `dispatchWithSensitive` + 取码助手、`X-2FA-Code` 请求头、`admin:exec` 与 terminal 路由的中间件
- CLI：`cmd/disable2FA.go`；依赖 `github.com/pquerna/otp`

**前端**（子代理执行，Lead 复核）
- `Login.tsx`、`RestrictedLoginDialog.tsx`、`account.tsx`（2FA 卡片 + 改密表单字段 + 三个 fetch + 二维码弹窗）、
  `exec.tsx`、`terminal/index.tsx`（OTP 弹窗与连接等待）、`AccountContext.tsx`
- 关于页许可清单去掉 `github.com/pquerna/otp`
- 5 个语言包各删 10 键（含子代理发现的第 10 个 `account.qr_fetch_error`——唯一使用者是已删的
  `TwoFactorDisabled`，删得正确）
- 子代理把只剩 SSO 的外层类名 `km-account-2fa` 改为 `km-account-sso-card`（仓内 0 引用旧类名，已复核）

## 验收（逐条实测）

| 项 | 结果 |
|---|---|
| `go build/vet/test` | 全绿 |
| 前端类型检查 | `tsc -p tsconfig.app.json`（非空跑）与 `tsc -b` 均 0 错误 |
| 源码/产物 2FA 命中 | 严格 case-sensitive = **0**（宽松 grep 的 1 处是 CSS 颜色 `#f2fafb` 误报） |
| 产品二进制 | `2FA` / `two_factor` / `pquerna/otp` / `disable-2fa` 均 **0 处** |
| 路由 | `/api/admin/2fa/*` 已不存在（返回 SPA 兜底 HTML，与任意不存在路径一致） |
| 登录 | 构造缺码请求返回 `Invalid credentials`，不再是 `2FA code is required` |
| release 0.0.19 | 18 资产，digest 与本地产物逐一一致（含补传的 `komari-agent-SHA256SUMS`） |
| 镜像 | `komari:0.0.19`/`:latest` 推送；内嵌 hash `39b2bb22…` |
| 生产 | systemd active，面板 `version=0.0.19 hash=39b2bb22…`，二进制 sha256 `c776b0ec…` = release 资产；agent `online (v2)`；日志 0 错误 |

## 过程中自己踩的坑（记录以免重犯）

1. **先构建后提交**：首版服务器产物内嵌 hash 是构建时的 HEAD（`7a7d1612…`），而 0.0.19 提交是
   `39b2bb2` → 与镜像/release 不一致。发现后**重新构建**并覆盖 release 资产与镜像。
   （这条自己文档 §3.4 里就写着"先提交后构建"，属于没照做。）
2. **`komari-agent-SHA256SUMS` 又漏传**：`gh release create dist/agent/komari-agent-*` 通配符匹配不到
   它（文件名不带前缀）→ 补传（与上次同款错误，已记在案）。
3. **`dist/` 整体 `rm -rf`** 时把刚建好的 agent 产物一起删了（顺序问题），已重建。

## 未做

- 数据库 `two_factor` 列**保留**（GORM AutoMigrate 不删列；不可逆且无必要，历史值无害）
- 未加"防止 2FA 被重新引入"的自检护栏（可另开任务）
