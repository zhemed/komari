# 实施清单：发布产物收敛到 linux/amd64

> 依赖：`prd.md`（R1–R6 / AC1–AC6）。改动全部是"注释停用 + 文档同步"，无架构决策。

## 1. 构建脚本（产出侧）

- [x] `scripts/build-agent.sh`
  - 头部注释：14 平台 → 只维护 linux/amd64（其余注释停用）
  - `OS_LIST`：保留 `linux`，注释 `windows` / `darwin` / `freebsd`
  - `ARCH_LIST`：保留 `amd64`，注释 `arm64` / `386` / `arm` / `loong64`
  - 矩阵段注释说明恢复方式；`--only` 逻辑不动
- [x] `scripts/gen-release-sums.sh`
  - `ASSETS` 只留 `komari-linux-amd64`，`komari-linux-arm64` 注释保留
  - 头部注释同步（不再"两个平台两次调用"）
- [x] `scripts/build-komari.sh`
  - 头部 arm64 静态用法行注释停用
  - zig target 的 `arm64` 分支注释停用；不支持时的报错文案改为"只维护 amd64"
- [x] `scripts/build-server-image.sh`
  - 默认 `PLATFORMS="linux/amd64"`；原多平台行注释保留
  - `need_asset()` 的 `linux/arm64` 分支注释停用
  - 头部用法/前置/QEMU 说明同步
- [x] `scripts/build-agent-image.sh`
  - 默认 `PLATFORMS="linux/amd64"`；原三平台行注释保留
  - `need_arch()` 的 `linux/arm64` / `linux/arm/v7` 分支注释停用
  - 头部用法说明同步

## 2. 文档与 spec（口径侧）

- [x] `docs/MAINTAINING.md`
  - §1 表：agent 资产"14 个平台" → 只维护 linux/amd64
  - §3.3：arm64 静态示例注释停用
  - §3.4：步骤 2 的 arm64 构建注释停用；步骤 3/3.5/6/7 的资产数与命令同步（4 个资产）
  - §11 agent 镜像行、§12 镜像平台表：单平台口径；多平台事故记录标注"多平台时期"
- [x] `.trellis/spec/backend/build-and-pinning.md`
  - §1.6：14 平台矩阵 → 只维护 linux/amd64（原矩阵注释保留）
  - §2 命令示例同步

## 3. 验证（可机械判定）

```bash
bash -n scripts/build-agent.sh scripts/gen-release-sums.sh scripts/build-komari.sh \
        scripts/build-server-image.sh scripts/build-agent-image.sh
# gen-release-sums 夹具（不依赖 go/zig）：
tmp=$(mktemp -d); : > "$tmp/komari-linux-amd64"
./scripts/gen-release-sums.sh "$tmp" && wc -l "$tmp/komari-SHA256SUMS"   # 期望 1
rm -rf "$tmp"
./scripts/check-repo.sh          # 快速检查全绿（含语法/文档引用/闸门/部署口径）
```

- [x] 上述命令全绿；`git diff --stat` 只含约定文件
- [x] 本机无 go / zig / docker：`check-repo.sh --full` 的 9–11 项记录为"发版机补跑"

## 4. 验证证据（2026-11-12 本机实测）

| 项 | 命令（用 /tmp 假 go/zig/docker/file 桩，不依赖真实工具链） | 结果 |
|---|---|---|
| 脚本语法 | `bash -n` × 5 个改动脚本 | 全通过 |
| agent 矩阵 | `./scripts/build-agent.sh`（假 go） | 只产出 `komari-agent-linux-amd64`（14→1），三道防线自检通过 |
| 服务端 amd64 | `KOMARI_STATIC=1 ./scripts/build-komari.sh`（假 go+zig） | 成功 |
| 服务端 arm64 | 同上 + `KOMARI_GOARCH=arm64` | 明确报错"只维护 amd64"（exit 1） |
| 校验和 | `./scripts/gen-release-sums.sh <夹具目录>` | 只生成 1 行 |
| 镜像默认平台 | `--push`（假 docker） | 两个镜像都只 `--platform linux/amd64` |
| 镜像负向 | `--platforms linux/arm64` / `linux/arm/v7` | 明确报错停用（exit 1） |
| 仓库自检 | `./scripts/check-repo.sh`（快速） | 全部通过（1–8、12–16） |
| 完整自检 | `./scripts/check-repo.sh --full` | **未执行**：本机无 go / zig / docker，第 9–11 项需在发版机补跑 |

## 5. 收尾

- [x] `./scripts/install-git-hooks.sh`（新克隆的 core.hooksPath 未装）
- [ ] 提交：`chore(build): 发布产物收敛到 linux/amd64，其余目标注释停用 [task:release-targets-prune]`
- [ ] `task.py finish` → `task.py archive ... --skip-branch-validation`，补 journal

## 回滚

全部是注释级改动：`git revert <commit>` 即恢复 14 平台 / 多架构口径，无数据迁移。
