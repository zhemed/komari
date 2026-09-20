# 构建并发布 0.0.19

## 用户指令

2026-09-20「那我们就构建0.0.19版本」→ 追问后确认「推镜像，并把 :latest 移到 0.0.19」。

## 结果（全部已核对）

| 项 | 状态 |
|---|---|
| 版本字面量 | `scripts/version.env`、`install-komari.sh`、`install-agent.sh`、`install-agent.ps1` → **0.0.19**（自检第 1 项核过） |
| 服务器静态产物 | `dist/komari-linux-amd64` 35,511,576 / `dist/komari-linux-arm64` 34,375,488，均 `statically linked`；`--help` 含 `Komari Monitor 0.0.19`；内嵌 hash = `5722a4a`（= 源码 HEAD） |
| agent | 14 平台全部构建（构建脚本三道门禁通过） |
| GitHub release | **0.0.19**，18 资产，`Latest`，发布于 2026-09-20T00:51:37Z |
| 镜像 | `ghcr.io/zhemed/komari:0.0.19` + `:latest`（2 架构）、`ghcr.io/zhemed/komari-agent:0.0.19` + `:latest`（3 架构） |
| 线上实测 | `docker run ghcr.io/zhemed/komari:latest /app/komari --help` → `Komari Monitor 0.0.19 (hash: 5722a4a…)` |
| compose 残留 | 发布产物与线上镜像内 `com.docker.compose` / `compose up` 均 **0 命中** |
| 自检 | `./scripts/check-repo.sh --full` 全绿 |

## 过程记录（含两次自己的失误）

1. **资产漏传**：`gh release create` 用 `dist/agent/komari-agent-*` 通配符时漏了 agent 的校验和
   —— 它的文件名是 `SHA256SUMS`（不带前缀）。发现后补传，因 `gh release upload` 不改名而多出一个
   `SHA256SUMS` 资产（19 个），已删除并用 `file#name` 语法正确上传 `komari-agent-SHA256SUMS`；
   最终 **18 个资产，清单与 0.0.18 完全一致（diff 为空）**。
2. **文档更新被短路**：一条 `grep && python3 …` 里的 grep 返回 1，导致更新没执行（误以为已改）。
   已单独重跑并生效。另外一次 python 因中文引号写错报 SyntaxError，也已修正。

## 未做（等用户决定）

- **生产升级**：用户的容器仍在跑 0.0.18；升到 0.0.19 由用户决定（重建容器或面板升级）。
  注意 0.0.19 相对 0.0.18 少了"host 网络自识别 + compose tag 同步"，挂 socket 的容器升级会走容器内替换。
- 旧镜像清理（0.0.16/0.0.17/0.0.18 的 ghcr 版本标签是否保留）。
