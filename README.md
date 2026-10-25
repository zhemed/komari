# Komari

> **独立维护**：本仓是我们自维护的唯一权威来源，**内核与主题均不再跟随任何上游** —— 规则见 [ops-docs/INDEPENDENCE.md](ops-docs/INDEPENDENCE.md)。

自用服务器监控：单个 Go 二进制 + 内嵌面板，节点装轻量 agent 上报。版本线 `0.0.x`（当前 0.0.24）。

## 部署

```bash
docker run -d --name komari --restart always \
  --network host \
  -v ./data:/app/data \
  ghcr.io/zhemed/komari:latest
```

数据在 `./data`；面板 `http://<主机>:25774`（首次进安装向导）。不用 host 网络就把
`--network host` 换成 `-p 25774:25774`。**升级**：面板「有新版本」点一下即可（容器内替换，零配置）。

## 节点 agent

```bash
wget -qO- https://raw.githubusercontent.com/zhemed/komari/main/install-agent.sh | sudo bash -s -- \
  -e <面板地址> -t <节点Token>

docker run -d --name komari-agent --restart always \
  ghcr.io/zhemed/komari-agent:latest -e <面板地址> -t <节点Token>
```

面板「节点 → 添加」会直接给出带密钥的现成命令。

## 自检

```bash
./scripts/check-repo.sh          # 发版前用 --full
```

其余（构建、发版、部署口径、回滚、维护规范）：[docs/MAINTAINING.md](./docs/MAINTAINING.md)
与 `.trellis/spec/`。

---

## 本仓库的自维护内容（内核 + 主题 + 部署脚本）

本仓库在承载 Komari 内核的同时，也承载我们自维护的主题与运维脚本：

| 路径 | 内容 |
|---|---|
| `themes/ink/` | 自维护的 ink 状态页主题源码（**版本线 1.x，当前 1.0.2**；更新通道与市场目录均指向本仓，与上游主题 0.6.x 无关） |
| `deploy/` | 部署脚本：`deploy-theme.sh`（静态部署，支持 `--dry-run`）、版本化启动脚本、`Dockerfile.local` |
| `ops-docs/` | 构建 / 部署 / 回滚 / 版本规则说明 |
| `build.sh` | 一键构建：内核二进制 + 本地镜像 + 主题 zip（含 sha256） |

约定：

- **内核代码路径保持与上游一致**，以便持续 merge 上游更新；自维护内容集中在上述新目录；
- 主题版本以 `themes/ink/komari-theme.json` 为唯一版本源；
- **主题更新/市场源均指向本仓**：更新走滚动 release `theme-latest` 资产 `ink-theme.zip`；市场目录为 `deploy/theme-market/v1.json`（实例配置与内核默认源都指向其 raw URL），不再引用上游目录；
- 构建产物（`bin/`、`ops-dist/`、`themes/ink/dist`、`themes/ink/node_modules`）一律不入库；
- 详细规则见 `ops-docs/VERSION.md`、`ops-docs/BUILD.md`、`ops-docs/DEPLOY.md`、`ops-docs/ROLLBACK.md`。
- **动本仓前先看** `ops-docs/BUILD.md` 末尾「运维经验」：① GitHub 推送首推常 408，需 HTTP/1.1 + 单线程并预期重试；② 提交必须带 `[task:<slug>]`（全局钩子更严时用 `git -c core.hooksPath=.githooks` 单次覆盖）。
- **权威来源**：本仓即我们自维护的权威来源（内核 + 主题 1.x + 部署/构建脚本）；主题不再跟随任何上游，可选的内核上游同步流程见 `ops-docs/VERSION.md`。
