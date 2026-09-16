# 执行计划（含每步验证）

顺序按"先能证明机制、再铺开文件、最后发布"排。每步都有可执行的验证命令，失败就停下。

## Step 1 — 补丁与构建脚本成型

1. 取上游 agent 源码固定到 `scripts/agent-pin.env`（`KOMARI_AGENT_COMMIT`、tree 哈希、Go 版本）。
2. 写 `scripts/patches-agent/0001-own-update-line.patch`（design §2 的 4 处改动）。
3. 写 `scripts/build-agent.sh`。

**验证**

```bash
./scripts/build-agent.sh                       # 14 个产物
strings -a dist/agent/komari-agent-linux-amd64 | grep -c '^zhemed/komari$'   # ≥1
./scripts/build-agent.sh --only linux/amd64    # 单平台快跑
```

## Step 2 — 默认关自更新 + 资产过滤的实测

1. 用临时 release（decoy）跑真实自更新，验证 filter 生效（design §5.3）。
2. 验证默认关：不带 `--enable-auto-update` 启动，日志无 `Checking update...`。
3. 清理临时 release/tag，确认仓库 refs 只剩 `main` 与 `0.0.x` tag。

**验证**：decoy 测试输出必须为 `I-AM-AGENT`；对照组必须为 `I-AM-SERVER`（证明风险真实）。
`gh release list -R zhemed/komari` 里不再有 decoy。

## Step 3 — 安装脚本 vendor + 补丁

1. `install-agent.sh` / `install-agent.ps1` 入库（带上游出处与 commit 注释）。
2. 按 design §3.2 打补丁（slug、含资产版本解析、注释）。

**验证**

```bash
bash -n install-agent.sh
grep -n "komari-monitor" install-agent.sh install-agent.ps1    # 应为空
bash install-agent.sh --help                                    # 参数面与上游一致
```

## Step 4 — 前端补丁 0006 + 重建 vendor

1. 写 `scripts/patches/0006-agent-install-source.patch`（design §3.3）。
2. `./scripts/sync-frontend.sh` 重放全部补丁并重建，更新 `FRONTEND_TREE_SHA256`。
3. 重跑一次确认哈希稳定（两次一致）。

**验证**

```bash
grep -rn "komari-monitor/komari-agent" web/public/defaultTheme/ | grep -v '\.map'  # 应为空
grep -rn "zhemed/komari" web/public/defaultTheme/assets/*.js | head            # 命中安装命令
GOPROXY=off GOFLAGS=-mod=mod ./scripts/build-komari.sh                          # 仍可构建
```

## Step 5 — 镜像

1. `Dockerfile.agent` + `scripts/build-agent-image.sh`。
2. 本地先构建单平台验证运行，再构建三平台并推送 `ghcr.io/zhemed/komari-agent:0.0.4`、`:latest`。
3. `docker run` 拉回来的镜像连本地服务器，确认上报。

**验证**：`docker manifest inspect ghcr.io/zhemed/komari-agent:latest` 有三个平台；
容器日志出现成功上报，面板出现节点。

## Step 6 — 发布 0.0.4

1. 版本位提升：`scripts/build-komari.sh` 默认版本、`install-komari.sh` 的 `REPO_TAG`。
2. 构建服务器静态产物（amd64/arm64）+ 14 个 agent 资产 + SHA256SUMS。
3. `gh release create 0.0.4 -R zhemed/komari …`（务必带 `-R`，见 `docs/MAINTAINING.md` §3.4）。
4. 推镜像 tag `0.0.4`。

**验证**

```bash
gh release view 0.0.4 -R zhemed/komari --json assets -q '.assets[].name'   # 19 个资产
file dist/komari-linux-amd64                                              # statically linked
```

## Step 7 — 本机部署与端到端

1. 本地服务器升级到 0.0.4（用我们自己的 `install-komari.sh` 升级路径）。
2. 面板里取安装命令（应显示我们的脚本与镜像），在本机用**该命令**装 agent。
3. 验证：systemd active、节点列表出现、版本显示为 `0.0.4`、数据有刷新。

**验证**：截图面板节点详情（版本/在线/负载）；`systemctl status komari-agent`；
本地部署的 server 日志有该 client 上线记录。

## Step 8 — 文档与规范收口

1. `docs/MAINTAINING.md`：新增 agent 发行线章节 + 发布清单补齐 + "已装上游 agent 只能重装"的说明。
2. `README.md`：安装命令、agent 默认关自更新、镜像地址。
3. `.trellis/spec/backend/build-and-pinning.md`：增补 agent 契约（pin、补丁、14 平台、filter 的必要性）；
   修正 §1.5 里"`pkg/jsruntime/` 必须保留"的过时描述（0.0.3 已随通知系统移除）。

**验证**：文档里的每条命令实际跑一遍（复制粘贴可用）；`grep -rn "komari-monitor/komari-agent" README.md docs/` 只应出现在"上游出处/历史"说明处。

## 检查（提交前必跑）

```bash
go build ./... && go vet ./... && go test ./...
GOPROXY=off GOFLAGS=-mod=mod ./scripts/build-komari.sh
git status --short          # 确认没有漏提交的 vendor 产物
```
