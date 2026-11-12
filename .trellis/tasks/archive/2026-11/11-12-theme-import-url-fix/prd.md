# 修复主题导入/更新对 GitHub release 下载 URL 的误判

## Goal

后台「主题→导入/更新」在遇到 **GitHub release 下载 URL**
（`https://github.com/<owner>/<repo>/releases/download/<tag>/<file>`）时，会把它误判成
GitHub **仓库地址**，转去 GitHub API 取 latest release 的第一个资产（实际拿到服务器二进制），
导致 `not a valid zip` 失败。修复判定逻辑，使具体资源 URL 走直接下载，并补回归测试。

## 背景与已确认事实（带证据）

- 本机实例（`/opt/komari`，0.0.25）实测：
  - `POST /api/admin/theme/import {"url":"https://github.com/zhemed/komari/releases/download/theme-latest/ink-theme.zip"}`
    → 400 `无法打开ZIP文件: zip: not a valid zip file`；
  - `POST /api/admin/theme/update {"short":"ink"}` → 同样 400（主题 manifest 的 `url` 就是该 release 下载 URL）。
- 根因：`web/api/admin/theme.go` 的 `isGitHubRepoURL()` 只要 host 是 github.com 且路径段数 `>= 2`
  就判为仓库地址（`parts[0]`/`parts[1]` 当 owner/repo），于是 `releases/download/...` 也被命中；
  随后 `getGitHubReleaseDownloadURL()` 取 `releases/latest` 的 `assets[0]`，下载到的不是主题 zip。
- 受影响入口：`ImportTheme`（`/api/admin/theme/import`）与 `UpdateTheme`（`/api/admin/theme/update`）
  都先走 `isGitHubRepoURL`；主题 manifest 的 `url` 自 2026-10-25 起指向本仓 `theme-latest` 资产。
- 生产面板（另一台机器）未动；本修复只在本机实例验证。

## Requirements

- **R1 判定收紧**：`isGitHubRepoURL` 只接受仓库根地址——`/owner/repo`、`/owner/repo.git`、
  可带末尾斜杠；路径段数不为 2 的一律返回 false。
- **R2 具体资源直接下载**：`releases/download/...`、`archive/...`、`tree/...`、`blob/...` 等
  返回 false，由调用方按普通 URL 直接下载（现有 `downloadThemeFromURL` 分支）。
- **R3 回归测试**：新增表驱动测试覆盖正例（仓库根地址）与反例（release 下载/archive/tree/非 GitHub/空）。
- **R4 本机实例验证**：用修复后的二进制替换本机实例二进制后，
  `POST /api/admin/theme/update {"short":"ink"}` 成功（下载 theme-latest zip 并更新主题），
  面板主题仍为 ink 1.0.4、资源 200。
- **R5 不扩大改动面**：不改 `getGitHubReleaseDownloadURL` 的资产选择策略（仍是第一个资产）、
  不改主题安装/解压/校验逻辑、不动生产。

## Acceptance Criteria

- [x] **AC1**：`go test ./web/api/admin/` 通过（含新增 `TestIsGitHubRepoURL`）。
- [x] **AC2**：`go build ./...` 与 `go vet ./...` 通过。
- [x] **AC3**：本机实例换成修复后二进制，`/api/admin/theme/update {"short":"ink"}` 返回成功，
      主题目录 `komari-theme.json` 版本仍为 1.0.4，`/themes/ink/komari-theme.json` 200。
- [x] **AC4**：`git diff` 只含 `web/api/admin/theme.go` + 测试 + 任务记录；提交带
      `[task:theme-import-url-fix]`。

## 非目标

- 不修 `getGitHubReleaseDownloadURL` 的“取第一个资产”策略（本次 URL 修复后该路径不再被主题 URL 触发）。
- 不动生产面板/生产容器；不发布新版本（修复随下次发版生效）。
- 不重构主题安装/市场其它路径。
