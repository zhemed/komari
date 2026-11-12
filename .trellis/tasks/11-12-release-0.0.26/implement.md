# 实施记录：发布 0.0.26

## 版本

- 内核：`0.0.25` → **`0.0.26`**（`own.6` → `own.7`）
- 主题：`1.0.4` → **`1.0.5`**
- 提交：`41c3a22`（版本 bump + 主题 1.0.5 构建产物 + 内置包刷新 + 新版本化部署脚本）

## 发布产物与证据（2026-11-12）

| 项 | 结果 |
|---|---|
| GitHub release | `https://github.com/zhemed/komari/releases/tag/0.0.26`（非 prerelease，Latest） |
| 服务端资产 | `komari-linux-amd64` 38,612,008 B，静态链接；sha256 `ddab2b83…` |
| 服务端校验和 | 匿名下载 `komari-SHA256SUMS` 与实算一致 ✓ |
| agent 资产 | `komari-agent-linux-amd64` 12,526,805 B；sha256 `94b76cf0…`，校验和一致 ✓ |
| tag | `0.0.26`、`server/v0.0.26-own.7`、`theme-ink/v1.0.5` 均指向 `41c3a22` ✓ |
| 镜像（服务端） | `ghcr.io/zhemed/komari:0.0.26` / `:latest`，匿名 manifest 200（linux/amd64） |
| 镜像（agent） | `ghcr.io/zhemed/komari-agent:0.0.26` / `:latest`，匿名 manifest 200 |
| 镜像内二进制 | `Komari Monitor 0.0.26 (hash: 41c3a22f4642be103bcbdcdb59254d5cc4e47ba2)` ✓ |
| 主题滚动通道 | `theme-latest`（prerelease）`ink-theme.zip` 3,606,158 B，sha256 `439cb768…`，包内 manifest 1.0.5 ✓ |
| 主题更新链路 | 本机实例面板「主题→更新」HTTP 200「主题更新成功」，manifest 1.0.4 → **1.0.5** ✓ |
| 发布版二进制 | 本机实例 `/api/version` = `0.0.26` / hash `41c3a22`；首页 200、三网双条正常（截图） |
| 仓库自检 | `check-repo.sh --full` 17 项全绿（tag 前，exit 0） |
| 版本字面量 | `check-repo` 第 1 项全绿：安装脚本/README/MAINTAINING 均为 0.0.26，已发布区间右端 0.0.25 ✓ |

## 发版环境（本机）

- zig 0.16.0（sha256 校验）解压至 `.build/tools/`：静态构建 `KOMARI_STATIC=1`。
- docker 29.1.3（apt 安装）+ buildx v0.39.0（GitHub release 下载）：镜像构建/推送；ghcr 登录用 `gh auth token`。
- 构建顺序遵循「先提交、后构建」：提交 `41c3a22` → 静态构建 → 镜像 → tag/release。

## 回滚

- 发布层面：`gh release delete 0.0.26 --cleanup-tag`，删除 tag；镜像 `latest` 回指上一版即可。
- 本机实例：`/opt/komari/komari.backup.20261112_102723`（0.0.26 之前）。
- 生产未动。
