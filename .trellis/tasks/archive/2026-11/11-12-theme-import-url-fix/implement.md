# 实施清单：修复主题导入/更新对 GitHub release 下载 URL 的误判

## 1. 代码改动

- [x] `web/api/admin/theme.go`：`isGitHubRepoURL` 收紧为**只认仓库根地址**
      （路径恰好 `/owner/repo`，允许末尾斜杠与 `.git`）；`releases/download/...`、`archive/...`、
      `tree/...`、`blob/...` 等返回 false → 调用方走普通 URL 直接下载
- [x] `web/api/admin/theme_test.go`：新增表驱动 `TestIsGitHubRepoURL`（5 正例 + 9 反例）

## 2. 环境准备（本机首次具备完整自检能力）

- [x] 安装 Go 1.26.11 到 `/usr/local/go`（go.dev 官方包 + sha256 校验；`go.mod` 要求 1.26.0 /
      toolchain 1.26.10，1.26.11 满足）；本机原有 gcc/make 可用
- [x] `PATH=/usr/local/go/bin:$PATH` 后 `go build ./...` / `go vet ./...` 通过

## 3. 验证证据（2026-11-12 本机实测）

| 项 | 命令/入口 | 结果 |
|---|---|---|
| 单元测试 | `go test ./web/api/admin/ -run 'TestIsGitHubRepoURL\|…' -v` | 4 个测试全 PASS（含 14 个子用例） |
| 全模块构建 | `go build ./...` | 通过 |
| 全模块 vet | `go vet ./...` | 通过 |
| 修复前基线 | 旧二进制（0.0.25 发布版）`POST /api/admin/theme/import|update` | 均 400 `not a valid zip`（问题复现） |
| 修复后·主题更新 | 新二进制（本机构建，hash `664efff`）`POST /api/admin/theme/update {"short":"ink"}` | **200 `主题更新成功`**（ink 1.0.4） |
| 修复后·主题导入 | `POST /api/admin/theme/import {"url":".../theme-latest/ink-theme.zip"}` | **200 `主题导入成功（已覆盖同名主题）`** |
| 主题状态 | manifest / `/themes/ink/komari-theme.json` / `/assets/*` / `/admin-app/` / `/` | 1.0.4；全部 200 |
| 仓库全量自检 | `./scripts/check-repo.sh --full` | **16 项全部通过**（Go 门禁 / 离线构建 / agent 门禁首次在本机跑通） |

## 4. 本地实例部署

- [x] 备份旧二进制：`/opt/komari/komari.backup.20261112_050317`（0.0.25 发布版，35,060,056 字节）
- [x] 替换为修复版（本机构建，25,808,504 字节，hash `664efff`），`systemctl restart komari`，
      `enabled` + `active`，`/api/version` = 0.0.25
- [x] 主题更新/导入在实例上验证通过（见上表）

## 5. 收尾

- [x] 提交：`fix(theme): 修正 GitHub release 下载 URL 被误判为仓库地址 [task:theme-import-url-fix]`
- [ ] 归档任务 + journal

## 回滚

- 代码：`git revert <commit>`。
- 本机实例：`cp /opt/komari/komari.backup.20261112_050317 /opt/komari/komari && systemctl restart komari`。
- 生产：未改动，无需回滚。
