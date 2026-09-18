# 去掉安装向导的密码复杂度限制

## Goal

用户截图：安装向导第 2 步（创建管理员）输入密码后被
**"密码必须包含大写，小写字母和数字"** 拦住，要求"去掉这个限制"。

## Background（取证）

| 位置 | 现状 | 说明 |
|---|---|---|
| `web/install/install.go:201-206` | 长度 8–256 **且** `hasStrongPassword`（大写+小写+数字） | 真正拒绝请求的后端校验 |
| `web/install/install.go:219` | `hasStrongPassword` 唯一调用点就是上面那处 | 删除后即为死代码 |
| `frontend/src/pages/install.tsx:85-89` | 先 `length < 8` 报 `password_too_short_error`，再正则 `(?=.*[a-z])(?=.*[A-Z])(?=.*\d)` 报 `password_strength_error` | 用户截图里的报错来自这里 |
| `frontend/src/pages/admin/account.tsx:80-88` | 改密页有同样的长度+复杂度两条 | 后端改密路径 `web/api/admin/update.go:40` **只要求 ≥6 位**、从不要求复杂度 |
| 5 个语言包 `account.password_strength_error` | 一条文案 | 两侧校验都删掉后成为死键 |

结论：复杂度规则**只存在于安装向导的后端 + 前端两处**；改密后端本来就没有它。

## Requirements

- **R1** 后端安装校验去掉复杂度要求（`hasStrongPassword` 调用与函数一并删除），
  **保留**长度 8–256、用户名/站点名/DSN 的既有校验。
- **R2** 前端安装向导去掉复杂度正则；**保留**长度 ≥8 与"两次输入一致"校验
  （与后端下限对齐，纯防呆）。
- **R3** 前端改密页去掉复杂度正则，与后端（≥6 位）不再自相矛盾；长度 ≥8 的前端防呆保留。
- **R4** 5 个语言包删除因此变为死键的 `account.password_strength_error`，不得留悬空引用。
- **R5** 前端改动必须重建产物并更新 `FRONTEND_TREE_SHA256`，连续两次构建哈希一致（可复现）。
- **R6** `docs/MAINTAINING.md` §4（与上游解耦点）记录这处刻意分歧与原因；
  改动落到用户手里需要一次发版（安装向导内嵌在二进制里）——**发版属网络动作，先向用户申请**。

## Acceptance Criteria

- [ ] `go test ./web/install/...` 通过，且含**新契约**的断言：
      "≥8 位、仅小写+数字" → 200 且管理员已创建；"<8 位" → 400 且未创建
- [ ] `grep -rn "hasStrongPassword\|password_strength_error"` 在代码与 5 语言包中 0 命中
- [ ] `frontend/src/pages/install.tsx` / `admin/account.tsx` 仍校验长度与两次一致
- [ ] `./scripts/build-frontend.sh` 通过，哈希更新且两次构建一致
- [ ] `./scripts/check-repo.sh --full` 全绿
- [ ] journal + 归档

## Out of Scope

- 不改长度下限（≥8 安装 / ≥6 改密）——用户只要求去掉复杂度限制；如也要去掉下限，另开一条。
- 不改 2FA、会话、审计等其它安全机制。
