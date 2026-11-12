# 实施记录：内置 ink 主题进二进制

## 步骤

- [x] 复制主题构建产物到 `web/public/builtinThemes/ink.zip`（ink 1.0.4，含双条版式）
- [x] 新增 `web/public/builtin_theme.go`：`//go:embed builtinThemes/ink.zip` +
      `InstallBuiltinThemes()`（缺失/损坏时安全解压，拒绝路径穿越、忽略符号链接）+
      `ActivateBuiltinThemeIfFresh()`（主题配置未设置时设为 ink）
- [x] `internal/server/bootstrap.go`：DB 初始化后调用上述两步
- [x] `build.sh`：构建主题后自动同步 `web/public/builtinThemes/ink.zip`
- [x] `scripts/check-repo.sh` 第 17 项：内置包存在/结构/版本与 `themes/ink` 一致
- [x] 文档：MAINTAINING（§1 表/§3.1/§3.4 第 8 步）、ops-docs/BUILD.md、spec §1.7、README
- [x] 本机实例部署新二进制（29.4MB，+3.6MB），已有主题未被覆盖

## 验证证据（2026-11-12 本机实测）

| 项 | 命令/入口 | 结果 |
|---|---|---|
| 编译 | `go build ./...` / `go vet` | 通过 |
| 全新实例 | 临时目录 + `127.0.0.1:25775` 启动 | `data/theme/ink` 自动生成（765 文件）；安装完成后 `configs.theme="ink"` |
| 首页 | `GET /`（全新实例） | ink（`admin-app/manifest.json`、`officialAppRoute`） |
| 主题资源 | `/themes/ink/komari-theme.json` | 200 |
| 后台/安装 | `GET /admin`（全新实例） | 官方应用（`entry-index-DSK2d6ec.js`）——未受影响 |
| 已有实例 | 本机实例换新二进制 | `komari-theme.json` sha256 未变；服务 active；首页仍 ink |
| 仓库自检 | `./scripts/check-repo.sh --full` | **17 项全部通过**（新增第 17 项通过） |
| 体积 | 二进制 | 25.8MB → 29.4MB（+3.6MB 内置包） |

## 回滚

- 代码：`git revert`；已有实例的 `data/theme/ink` 不受影响（本改动只在缺失时安装）。
- 本机实例二进制备份：`/opt/komari/komari.backup.20261112_055145`。
