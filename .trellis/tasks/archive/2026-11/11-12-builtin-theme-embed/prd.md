# 内置 ink 主题进二进制（首次启动自动安装 + 新库默认激活）

## Goal

把 ink 主题作为**内置主题**融进服务端二进制：构建产物提交进仓库并 `//go:embed`；
首次启动若 `data/theme/ink` 缺失自动解压安装；全新数据库默认激活 `ink`。
效果：二进制 / Docker 镜像 / 安装脚本装完即带主题，不需要再手动安装。

## 背景与已确认事实

- 现状：核心内置的是官方默认 React 主题（`web/public/defaultTheme`，承担 `/`、`/admin`、`/install`），
  ink 是独立安装包（`data/theme/ink`）；Dockerfile 只 COPY 二进制 → 新部署必然裸主题。
- 用户 2026-11-12 定稿：选方案 1（内置主题 + 后续阉割主题商店）；方案 2（替换 defaultTheme）留待办。
- 主题包结构：`komari-theme.json` + `preview.png` + `dist/`（含 `admin-app/`），构建产物约 3.5MB。
- 启动顺序：`Bootstrap()`（DB 初始化）→ 安装向导（如需）→ 正常服务。

## Requirements

- **R1 内置包入库**：`web/public/builtinThemes/ink.zip`（themes/ink 构建产物）提交进仓库，
  `//go:embed` 进二进制；纯 Go 离线构建不依赖 Node。
- **R2 首次启动自动安装**：`data/theme/ink` 缺失（或损坏缺 manifest）时从内置包安全解压安装；
  已有安装（含用户更新的版本）**不覆盖**。
- **R3 新库默认激活**：主题配置从未设置过时设为 `ink`；已有显式设置（含 `default`）不覆盖。
- **R4 不替换官方默认主题**：`web/public/defaultTheme` 不动——`/admin`、`/install`、guide 页面零风险。
- **R5 构建与门禁**：`build.sh` 构建主题后自动同步内置包；`check-repo.sh` 第 17 项校验
  包存在、含 `dist/index.html`、`short=ink`、版本与 `themes/ink/komari-theme.json` 一致。
- **R6 文档同步**：`docs/MAINTAINING.md`（§1 表/§3.1/§3.4）、`ops-docs/BUILD.md`、
  `.trellis/spec/backend/build-and-pinning.md`（§1.7）、`README.md`。
- **R7 验证**：全新临时实例（新数据目录 + 独立端口）装完即 ink；已有实例升级不覆盖主题；
  `check-repo.sh --full` 全绿。

## Acceptance Criteria

- [x] **AC1**：`go build ./...` / `go vet` 通过；二进制体积增加约 3.6MB（内置包）。
- [x] **AC2**：全新实例：`data/theme/ink` 自动生成、`configs.theme=ink`、`/` 为 ink（Vue assets）、
      `/themes/ink/komari-theme.json` 200。
- [x] **AC3**：全新实例 `/admin` 仍为官方应用（entry-index）→ 安装向导/后台不受影响。
- [x] **AC4**：已有实例换新二进制后 `data/theme/ink` 的 manifest 未被覆盖，首页仍正常。
- [x] **AC5**：`./scripts/check-repo.sh --full` 17 项全部通过（含第 17 项内置包守卫）。
- [x] **AC6**：提交带 `[task:builtin-theme-embed]`；生产未动。

## 非目标

- 不阉割主题商店（单独任务，见用户 2026-11-12 决定）。
- 不替换 `web/public/defaultTheme`（方案 2 留待办）。
- 不改变主题更新通道（`theme-latest` 仍可用；`data/theme/` 优先于内置包）。
