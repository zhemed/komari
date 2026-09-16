# 设计：agent 自有版本线（单仓库）

## 0. 变更边界（最小行为缺口 → 落点）

| 现状 | 目标 | 落在哪 |
|---|---|---|
| 上游 agent 自更新目标 = `komari-monitor/komari-agent`，默认开 | 目标 = `zhemed/komari`，默认关 | `update/update.go`（pin + 补丁）、`cmd/{root,flags}` |
| 同 release 混装两种资产时自更新会抓到服务器二进制 | 只允许命中 `komari-agent-*` | `update/update.go` 的 `selfupdate.Config{Filters: …}` |
| 安装命令指向上游脚本/镜像 | 指向我们的脚本/镜像 | `install-agent.sh`/`.ps1`（vendor+补丁）、前端补丁 0006 |
| 我们发不出 agent 二进制 | 14 个资产随 release 发布 + 自建镜像 | `scripts/build-agent.sh`、`scripts/build-agent-image.sh` |

**明确不做**：不改监控/上报/远程控制逻辑；不 fork；不新增 CI；不做服务器侧版本闸门。

## 1. 为什么"单仓库"是安全的（关键机制，已核实）

1. `update/update.go:22-23` 是包级 `var`：`-X .../update.CurrentVersion=` 与
   `-X .../update.Repo=` 都能注入（实测二进制里已生效）。
2. 稳定通道匹配资产走 `selfupdate/detect.go:69-75` 的**后缀**匹配
   （`linux-amd64` / `linux_amd64` + 归档扩展名），**不看前缀**。
   → `komari-linux-amd64`（服务器）与 `komari-agent-linux-amd64`（agent）都会命中。
3. 库支持资产正则过滤：`selfupdate.Config.Filters`（`updater.go:31-33,53-67`），
   命中任一 filter **且**命中后缀才选中。加 `^komari-agent-` 即可彻底隔离。
4. `findReleaseAndAsset` 会跳过没有匹配资产的 release（`detect.go:113-123`），
   所以"某个 release 忘了传 agent 资产"不会让老 agent 更新失败——只是被跳过。
5. agent 用 `selfupdate.Config{}`（无 Validator，`updater.go:71`），
   **不需要** `.sha256` 伴随资产。

## 2. agent 补丁系列（`scripts/patches-agent/0001-own-update-line.patch`）

上游 pin commit：`9e532e0429cd…`（浅克隆 main 当前 HEAD，记录进 `scripts/agent-pin.env`）。
补丁共 4 处逻辑改动，逐条给理由：

| # | 文件:行（上游） | 改动 | 理由 |
|---|---|---|---|
| 1 | `update/update.go:23` | `Repo = "zhemed/komari"` | 源码默认即正确；`-X` 只是双保险，别人不用我们的构建脚本也不会指回上游 |
| 2 | `update/update.go:324` | `selfupdate.NewUpdater(selfupdate.Config{Filters: []string{"^komari-agent-"}})` | §1.2 的误刷风险唯一正解 |
| 3 | `update/update.go:249`(`checkAndUpdateStable` 开头) | 复用 `isContainerAgent()`，容器内跳过并提示更新镜像 | 容器里替换二进制会随容器重建丢失，稳定通道原本漏了这个判断（只有 snapshot 通道有） |
| 4 | `cmd/flags/flag.go` + `cmd/root.go:121,163` | 新增 `EnableAutoUpdate bool`（`json:"enable_auto_update"`、`env:"AGENT_ENABLE_AUTO_UPDATE"`）、新标志 `--enable-auto-update`；判定从 `if !DisableAutoUpdate` 改为 `if EnableAutoUpdate`；`--disable-auto-update` 保留（隐藏 + deprecated 提示）；`-autoUpdate`/`--autoUpdate` 从"剥离丢弃"改为"置 `EnableAutoUpdate=true`" 并告警 | 默认关；旧的 `AGENT_DISABLE_AUTO_UPDATE=1` 语义不变（本来就是关）；旧脚本里的 `-autoUpdate` 意图是"开"，照旧生效 |

env/配置文件绑定是反射式的（`cmd/root.go:194-226` 遍历 `env:` 标签、`root.go:47` 走 JSON），
新增字段自动被 `AGENT_ENABLE_AUTO_UPDATE` 与 `enable_auto_update` 生效，无需改绑定代码。

补丁不含：readme、CI、`build_all.sh`（我们自己写构建脚本）、版本号字面量。

## 3. 新增/改动的仓库文件

```
scripts/agent-pin.env                 # KOMARI_AGENT_COMMIT / TREE_SHA256 / GO_VERSION
scripts/patches-agent/0001-own-update-line.patch
scripts/build-agent.sh                # pin 克隆 + 打补丁 + 14 平台交叉编译
scripts/build-agent-image.sh          # buildx 多架构构建 + 推送 ghcr
Dockerfile.agent                      # vendor 上游 Dockerfile（alpine pin 到具体 digest）
install-agent.sh                      # vendor 上游 install.sh + 补丁
install-agent.ps1                     # vendor 上游 install.ps1 + 补丁
scripts/patches/0006-agent-install-source.patch
docs/MAINTAINING.md                   # 新增 §11 agent 发行线；§3.4 发布清单加 agent 资产与镜像
README.md                             # 安装命令改指我们；说明 agent 默认关自更新
.trellis/spec/backend/build-and-pinning.md  # 增补 agent 契约；修正 jsruntime 已移除的过时描述
```

### 3.1 `scripts/build-agent.sh`

- 默认 `KOMARI_VERSION=0.0.4`，与 `build-komari.sh` 同一变量名。
- 克隆上游到 `.build/agent-src`，`git checkout $KOMARI_AGENT_COMMIT`，校验 commit；
  打 `scripts/patches-agent/*.patch`；`git diff` 为空则说明补丁未生效 → 报错。
- 矩阵与上游 `build_all.sh` 一致：`{windows,linux,darwin,freebsd} × {amd64,arm64,386,arm,loong64}`
  减掉上游排除的组合（windows/arm、darwin/{386,arm}、非 linux 的 loong64）＝ 14 个。
- `CGO_ENABLED=0 go build -trimpath -ldflags "-X ...update.CurrentVersion=$VERSION -X ...update.Repo=zhemed/komari"`。
- 输出 `dist/agent/komari-agent-<os>-<arch>[.exe]`，并写 `dist/agent/SHA256SUMS`。
- 纯 Go，不需要 zig；这是与服务器构建最大的不同点。

### 3.2 `install-agent.sh`（vendor + 补丁）

上游脚本 804 行、25KB，自带 arg 解析、systemd/launchd 单元、ghproxy、卸载。
**保留这些行为**，只改必须改的：

| 位置（上游 install.sh） | 改动 |
|---|---|
| `:325` snapshot 解析的 API URL | slug → `zhemed/komari`（我们不发 Snapshot，此分支只会报错退出，代码保留） |
| 版本解析（`:324-375`） | 删除上游 `resolve_snapshot_version`（我们不发 `Snapshot-*`，遇到 `--install-version snapshot` 直接报错退出）；**默认装脚本 pin 的版本**（`default_agent_version`，随发布同步），`--install-version <版本>` / `latest` 仍可用 |
| 头部说明/注释里的上游仓库名 | 改指我们 |

理由（与最初设想的差别，写实）：我原计划让脚本走 GitHub API 解析"含资产的最高版本"，
但 shell 里没有 jq 时可靠解析 `tag_name`+`assets` 需要脆弱的文本拼接，收益不抵复杂度；
改为**默认装脚本 pin 的版本**——脚本随 `main` 更新，用户拿到的就是当前发布的 agent，
确定、无需额外 API 调用、不会 404。`latest` 仍保留给显式要最新的人（`install.sh` 走 `releases/latest/download`，`install.ps1` 走
`/releases/latest` 取 tag 再下载；两者都依赖 §3.5 的发布纪律保证 agent 资产存在，
而 agent 自身的自更新是容错的——`findReleaseAndAsset` 会跳过没有匹配资产的 release）。参数面**不动**：`--install-dir` / `--install-service-name` / `--install-ghproxy` /
`--install-version` / `--install-no-mirror` 保持原样，其余参数原样透传给 agent 二进制
（`install.sh:105-118`），因此前端生成的命令不需要改参数格式，只改脚本 URL。

`install-agent.ps1` 同理只改 slug / 资产下载来源。

### 3.3 前端补丁 0006

上游 pin commit 不变（`4a74e8a8…`），新增 `scripts/patches/0006-agent-install-source.patch`：

| 文件:行 | 现状 | 改为 |
|---|---|---|
| `NodeFunction.tsx:90,96,106` | `raw.githubusercontent.com/komari-monitor/komari-agent/refs/heads/main/install.{sh,ps1}` | `raw.githubusercontent.com/zhemed/komari/refs/heads/main/install-agent.{sh,ps1}` |
| `index.tsx:320,1528` | `.../komari-agent/refs/heads/main/${scriptFile}` | `.../zhemed/komari/refs/heads/main/${scriptFile}`，`scriptFile` 由 `install.sh/install.ps1` 改为 `install-agent.sh/install-agent.ps1` |
| `index.tsx:378,1581` | `ghcr.io/komari-monitor/komari-agent:latest` | `ghcr.io/zhemed/komari-agent:latest` |
| `index.tsx:251,1463`、`NodeFunction.tsx:61` | `if (disableAutoUpdate) args.push("--disable-auto-update")` | `if (!disableAutoUpdate) args.push("--enable-auto-update")` |
| `index.tsx:211,1421`、`NodeFunction.tsx:46` | `disableAutoUpdate: false` | `disableAutoUpdate: true`（默认勾上＝默认关自更新，与二进制默认一致） |

i18n 文案与 key 不改（`admin.nodeTable.disableAutoUpdate` = "禁用自动更新"，默认勾选后语义自洽）。
改完重跑 `sync-frontend.sh` 并更新 `FRONTEND_TREE_SHA256`（规范 §1.3 要求写明原因）。

### 3.4 镜像

`Dockerfile.agent`（vendor 上游，`alpine:3.21` 固定 digest，保留 `touch /.komari-agent-container`），
`scripts/build-agent-image.sh` 用 buildx 构建 `linux/amd64`、`linux/arm64`、`linux/arm/v7`
三平台，推 `ghcr.io/zhemed/komari-agent:<version>` 与 `:latest`。
登录用 `gh auth token`（token 已含 `write:packages`）：
`docker login ghcr.io -u zhemed --password-stdin`。
构建上下文由脚本在 `.build/agent-image/` 组装（Dockerfile 只 `COPY` 同名资产）。

### 3.5 发布纪律（写进 `docs/MAINTAINING.md` 与 checklist）

一个 tag = 一个 release = 整套栈：`komari-linux-amd64/arm64` + 14 个 `komari-agent-*`
+ `install-komari.sh`/`install-agent.sh`/`install-agent.ps1` 随仓库（不打进 release 也可以，
前端命令指向 `refs/heads/main`）。镜像推送与 release 同一版本号。
**agent 版本号与服务器同一条 `0.0.x` 线**，不做独立版本号（节点上看到的即我们的版本）。

## 4. 已装的上游 agent 怎么办（写实）

我们**无法**远程改变别人已装的上游 agent（它指向 `komari-monitor/komari-agent`，
默认每 6 小时自更新到上游 `1.5.10`）。可做的只有：文档给出"重装命令"，
并在服务器上不做版本闸门（上游 agent 仍能正常上报，协议 v1/v2 未变）。
本机此前只装了服务器，没有 agent，因此本地环境不存在这个历史包袱。

## 5. 验证设计（先验证再宣布完成）

1. **补丁可重放**：`build-agent.sh` 在干净 `.build/agent-src` 上克隆 + 打补丁 + 构建成功。
2. **注入正确**：`strings` 命中 `zhemed/komari`；无 `--enable-auto-update` 时启动日志
   不出现 `Checking update...`。
3. **资产过滤（关键，会真跑一次自更新）**：
   - 建临时 release：tag `0.0.99-decoy`（非 pre-release，semver 可解析），上传两个
     **能自证身份**的小 ELF：`komari-agent-linux-amd64`（打印 `I-AM-AGENT`）与
     `komari-linux-amd64`（打印 `I-AM-SERVER`）。
   - 用 `CurrentVersion=0.0.4` + `--enable-auto-update` 的形态跑一次更新检查，
     期望自更新后进程变体打印 `I-AM-AGENT`；再验一遍"去掉 filter"的对照组会抓到
     `I-AM-SERVER`（证明这条防线不是装饰）。
   - 清理：删除临时 release 与 tag。
4. **安装脚本 E2E（本机）**：`bash install-agent.sh -e 127.0.0.1:25774 <token|--auto-discovery> --install-dir /opt/komari-agent --install-service-name komari-agent`，
   验证 systemd 单元 active、日志有上报、面板节点列表出现且版本号为我们的版本。
5. **镜像**：`docker run` 我们推的镜像（prebuilt 资产）连本地服务器，verifying 上报。
6. **回归**：`go build ./... && go vet ./... && go test ./...`；
   `GOPROXY=off GOFLAGS=-mod=mod ./scripts/build-komari.sh` 仍成功；
   前端哈希门禁复现一致。

## 6. 回滚

- agent 补丁/脚本/镜像都是**新增文件**，回滚 = 删文件 + `git revert`；
- 前端补丁 0006 回滚 = 删补丁 + 重跑 `sync-frontend.sh`（哈希回到旧值）；
- 已发布 release 与镜像可 `gh release delete` / 镜像 tag 删除（token 有 `delete:packages`）；
- 本地 agent 回滚 = 停用 systemd 单元 + 删 `/opt/komari-agent`。
