# 发布产物收敛：仅保留自有服务器目标平台

## Goal

把 release 资产与容器镜像的构建矩阵，从"全平台"收敛为**只维护自有服务器对应的 linux/amd64**；
其余目标在构建脚本与发布文档里**注释保留**（不删除，恢复时取消注释即可）。
收敛后一次发版只产出 4 个 release 资产：服务端 `komari-linux-amd64` + `komari-SHA256SUMS` +
agent `komari-agent-linux-amd64` + `komari-agent-SHA256SUMS`（GitHub 页面另加 2 个源码包，显示 6 个）。

## 背景与已确认事实（带证据锚点）

- 0.0.25 release 实际 18 个资产：服务端 2（`komari-linux-amd64`、`komari-linux-arm64`）+
  `komari-SHA256SUMS` + agent 14 + `komari-agent-SHA256SUMS`（`gh release view` 实测）。
  用户截图里的 Assets 20 = 18 + 2 个 GitHub 自动源码包。
- 生产服务器跑的是 linux/amd64：`docs/CLEANUP-2026-09-19.md:56` 记录 `/opt/komari/komari` 的
  sha256 = 已发布 0.0.17 的 `komari-linux-amd64`。
- 生产部署主方案是 Docker 镜像（`docs/MAINTAINING.md` §3.4.1）。两个镜像当前多平台：
  `scripts/build-server-image.sh:39`（amd64+arm64）、`scripts/build-agent-image.sh:30`
  （amd64+arm64+armv7）。
- 资产由三处产出：agent 矩阵 `scripts/build-agent.sh`（OS_LIST/ARCH_LIST，14 个组合）、
  服务端两次 `build-komari.sh` 调用（`docs/MAINTAINING.md` §3.4 第 2 步）、
  校验和清单 `scripts/gen-release-sums.sh`（两行）。
- 用户 2026-11-12 定稿：只维护自有服务器对应版本，其余全部注释掉（已确认**含镜像平台**，
  选择"服务端 + agent 都只保留 linux/amd64"）。

## Requirements

- **R1 只维护 linux/amd64**：agent 构建矩阵、服务端发布构建、校验和清单都只产出 linux/amd64。
- **R2 注释而非删除**：其余目标保留为注释，恢复某个平台 = 取消对应注释，不改构建逻辑。
- **R3 镜像同步收敛**：两个镜像脚本的默认 `PLATFORMS` 与"平台→资产"映射只保留 linux/amd64，
  arm64 / armv7 映射注释停用。
- **R4 文档与 spec 同步**：`docs/MAINTAINING.md`（§1 表、§3.3、§3.4、§11、§12）与
  `.trellis/spec/backend/build-and-pinning.md` 不得再声称 14 平台 / 18 资产 / 多架构镜像；
  多平台事故的历史记录保留，但要标注"多平台时期"，避免被读成现行口径。
- **R5 零行为改动**：不改服务端/agent 运行代码、不升版本号、不发布、不推送。
- **R6 机械门禁可过**：改动脚本 `bash -n` 全通过；`./scripts/check-repo.sh` 快速检查全绿；
  `gen-release-sums.sh` 用夹具验证只产出一行。

## 非目标（明确不做）

- 不动 `agent/build_all.sh` / `agent/build_all.ps1`：它们是 vendored 上游 agent 快照自带的构建脚本，
  我们的发布链路不使用；改它们等于额外偏离上游快照（`docs/MAINTAINING.md` §11 的溯源口径）。
- 不改 `install-komari.sh` / `install-agent.sh` / `install-agent.ps1` 的运行时平台探测：
  它们不是构建矩阵；非 amd64 机器上会在下载资产时明确 404（因为不再发布对应资产）。
- 不删除已发布的 0.0.25 及更早 release 的历史资产（历史 release 保持原样）。
- 不修改 `agent/` 源码内联逻辑（自更新、资产过滤等契约不动）。

## Acceptance Criteria

- [ ] **AC1**：`scripts/build-agent.sh` 默认只构建 linux/amd64；`--only linux/amd64` 行为不变；
      其余 13 个目标以注释形式存在，取消注释即可恢复。
- [ ] **AC2**：`scripts/gen-release-sums.sh` 只生成一行（`komari-linux-amd64`），夹具目录验证通过。
- [ ] **AC3**：两个镜像脚本默认 `PLATFORMS="linux/amd64"`；arm64 / armv7 映射注释保留。
- [ ] **AC4**：文档与 spec 里不再有未标注的"14 个平台 / 18 个资产 / 多架构镜像"现行口径；
      历史段落明确标注为历史。
- [ ] **AC5**：`bash -n` 改动的脚本全部通过；`./scripts/check-repo.sh` 快速检查全绿。
      本机无 go / zig / docker，`--full` 的 9-11 项无法执行，需在发版机补跑（记录在案）。
- [ ] **AC6**：`git diff --stat` 只涉及约定文件；提交消息带 `[task:release-targets-prune]`。

## Notes

- 只改构建/发布口径，不碰运行时逻辑；`docs/MAINTAINING.md` §3.4 的发布顺序（先提交、后构建）不变。
- 现有生产服务器与节点均为 linux/amd64，收敛不改变生产行为。
