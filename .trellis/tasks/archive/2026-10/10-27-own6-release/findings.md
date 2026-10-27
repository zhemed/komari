# 发版记录：0.0.25+own.6 / 主题 1.0.4

## 一、结论

「彻底移除 Web 终端与远程执行（WebSSH + RCE）」已作为正式发行发布：

- 内核版本：**`0.0.25+own.6`**，GitHub Release tag `0.0.25`（Latest）、组合 tag `server/v0.0.25-own.6`；
- 主题版本：**`1.0.4`**，tag `theme-ink/v1.0.4`，滚动通道 `theme-latest`（prerelease）已覆盖更新；
- 镜像：`ghcr.io/zhemed/komari:0.0.25`、`ghcr.io/zhemed/komari-agent:0.0.25`（均含 `:latest`）；
- 发布前 `./scripts/check-repo.sh --full` **全部通过**（16 项）。

## 二、提交与标签（均已推送，远端 main = d4ab363）

| 提交 | 内容 |
|---|---|
| `942bb4a` | 版本线 0.0.24→0.0.25（own.6）+ 主题 1.0.4 + 新部署脚本 + 文档口径（发布 tag `0.0.25` 指向本提交） |
| `5c9142e` | 主题市场目录 `deploy/theme-market/v1.json` → 1.0.4 + 新 zip sha256 |
| `d4ab363` | 记录 zig 0.16.0 已装入 `.build/tools`（仅发版静态构建用） |

标签（远端已核对 SHA）：`0.0.25`→942bb4a（轻量）、`server/v0.0.25-own.6`→942bb4a（附注）、
`theme-ink/v1.0.4`→942bb4a（附注）、`theme-latest`→942bb4a（滚动，force 更新）。

## 三、构建产物与校验

| 产物 | sha256 |
|---|---|
| `komari-linux-amd64`（静态） | `b40c781fe70dd62dabf1c6fa65083f4b31c85ba51beac40c8c421549328abf73` |
| `komari-linux-arm64`（静态） | `5b29233beb923bbe75f89b70a68051d2d263d8d14144b33a55bb3dcd9900de1e` |
| `komari-agent-linux-amd64`（14 平台之一） | `7de6aeadc2081c82565a23d089e4daf62325cfde9a0aeefae1af41f2b953476a` |
| 主题 `ink-build-942bb4a.zip`（发布名 `ink-theme.zip`） | `137a0d2fca1404456fde28985eb848a1fca2573d1e68de0741c85308858a437f` |

- 静态构建工具：zig `0.16.0`（下载自 ziglang.org，sha256 与官方 index 一致
  `70e49664a74374b48b51e6f3fdfbf437f6395d42509050588bd49abe52ba3d00`，解压到 `.build/tools/`，已 gitignore）。
- 镜像 digest（推送返回）：服务端 `sha256:7ef9f4cab0345b67470fc856d8c691b84d896950efae83bc21f8d0c04ac6f7ac`；
  agent `sha256:bb690aa16e64b9394b24207c76338e0ddadd2e6abed2b8592cb7b0a3112c22b4`。

## 四、GitHub Release

- `0.0.25`（正式、Latest）：**18 个资产**——`komari-linux-amd64`、`komari-linux-arm64`、
  `komari-SHA256SUMS`、`komari-agent-SHA256SUMS`、14 个 `komari-agent-<os>-<arch>[.exe]`；
  匿名下载 `komari-SHA256SUMS` 与本地内容一致。
- `theme-latest`（prerelease 滚动通道）：`ink-theme.zip`（3,612,364 bytes）与 `.sha256` 已覆盖上传，
  sha256 与市场目录一致；release 正文更新为 1.0.4 说明。
- 匿名 ghcr manifest 校验：`komari:0.0.25`、`komari-agent:0.0.25` 均 HTTP 200。
- 市场目录 raw（main）已为 `version=1.0.4, sha256=137a0d2f…`。

## 五、验证与边界

- `check-repo.sh --full`：16 项全绿（版本字面量、Trellis 闸门、Go 质量门禁、离线构建、
  agent 门禁、前端产物哈希、2FA/upgrade/部署口径回归等）。
- 发布顺序遵守「先提交、后构建」：二进制内 `VersionHash=942bb4a…`，与 tag 指向一致。
- 未做：其它仓库/主机；未改动历史；site 专属值未写入仓库（release 说明用「本仓库」表述）。
- 生产部署（面板/agent/主题）、旧 DB 表清理与浏览器验收由主仓任务
  `10-27-komari-own6-release-cleanup` 执行并记录。
