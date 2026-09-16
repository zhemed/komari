# 上游源码全部 vendor 进仓库：前端与 agent 变成完全自有

## Goal

把两个上游依赖的源码搬进本仓库，让 fork 从"pin + 补丁"变成**完全自有**：
- `frontend/` ← komari-web `4a74e8a8`（含我们 6 个补丁的效果）
- `agent/` ← komari-agent `1186aafb`（含我们 3 个补丁的效果）

此后改前端/agent 就是改本仓库的代码，不再需要联网克隆上游，也不再有"补丁上下文失效"这类问题。

## Requirements

1. 导入方式为**快照导入**（不带上游历史），与仓库既有的"上游代码以单个快照根提交引入"策略一致。
2. 我们此前的补丁内容**内联进源码**，补丁文件与 `sync-frontend.sh`/`build-agent.sh` 里的
   clone+apply 路径删除；`scripts/patches/`、`scripts/patches-agent/` 退场。
3. 构建脚本改为本地构建：`scripts/build-frontend.sh`（原 sync-frontend.sh）、`scripts/build-agent.sh`。
4. 保留所有仍有意义的门禁：前端产物目录树哈希、agent 的版本一致性、agent 的资产过滤与自更新目标断言。
5. 不再需要、应删除的门禁：agent 的上游 pin 校验（源码已在仓库，结构上不可能漂移）、
   安装脚本"补丁回放比对"（补丁没了）。
6. 文档/规范/README 同步：`docs/MAINTAINING.md`、`.trellis/spec/backend/build-and-pinning.md`、`README.md`。
7. **产物必须不变**：不因为这次改造而改动任何已发布的二进制/前端产物。

## Acceptance Criteria

- [ ] `agent/`：从 vendored source 构建出的 `linux/amd64` 二进制，与 `0.0.5` release 资产在
      相同 ldflags/`-buildvcs` 条件下**逐字节一致**。
- [ ] `frontend/`：从 vendored source 重建，`web/public/defaultTheme/` 的规范化目录树哈希仍为
      `af0bd793…`（与 `scripts/frontend-pin.env` 一致）。
- [ ] `scripts/build-agent.sh` 在**无网络**条件下可完成 linux/amd64 构建（源码本地；Go 模块缓存已预热）。
- [ ] `scripts/build-frontend.sh` 不再引用任何上游仓库 URL（除保留的溯源注释）。
- [ ] `go build ./... && go vet ./... && go test ./...` 全绿；`GOPROXY=off GOFLAGS=-mod=mod ./scripts/build-komari.sh` 成功。
- [ ] `grep -rn "git clone\|git fetch" scripts/` 只剩溯源注释，没有构建期克隆。
- [ ] 本次改造不产生新的 release（产物未变），0.0.5 仍是当前发布版本。

## Non-Goals

- 不把 agent 的 Go 依赖 vendor（`agent/vendor/`）——构建仍需 Go module proxy；如需要可另行处理。
- 不 vendor npm 依赖（`node_modules` 不入库）；改前端仍需 Node + npm 联网装依赖。
- 不引入上游历史，不做 `git subtree`。
