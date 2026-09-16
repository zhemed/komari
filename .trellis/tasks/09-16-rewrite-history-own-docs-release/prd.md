# 以 0.0.1 重写历史与自有文档，并发布首个 release

## Goal

让仓库真正成为"我们自己的项目"：**提交历史里不再有非我们维护的上游提交**；
**移除上游 README 并重写为自维护文档**；清理上游 CI 与模板；补完遗留的规范任务；
默认分支改名为 `main`；并**发布 0.0.1 release**，使 `install-komari.sh` 真正可用。

## 背景与已确认事实（实测）

### 历史现状

- 远端只有两个 ref：`HEAD` 与 `refs/heads/komari-1.4.3`（均指向 `77f36da`）——**从未推送过任何 tag**。
- 本地历史 = 上游 835 个提交（至 `bf6b45ec`，tag 1.4.3）+ 我们的 7 个提交
  （`e70304f` → `5942199` → `a76cd2c` → `2d1c5ed` → `18305a9` → `49c7d22` → `77f36da`）。
- 本地有 68 个上游 tag（`0.0.3`…`1.5.0-fix1`）。注意上游历史里存在 `0.0.3`–`0.0.9`，
  与我们的 `0.0.x` 版本线数值重叠（我们只推自己的 tag，且会删除本地上游 tag）。

### 待移除/重写的上游资产

| 资产 | 内容 | 处理 |
|---|---|---|
| `README.md` | 上游徽章、socialify 图、Rainyun/1Panel 部署按钮、上游截图、上游功能列表 | **删除并重写** |
| `README_zh-cn.md` | 同上（中文版） | **删除**（合并进我们的单一 README） |
| `.github/workflows/*`（11 个） | `release.yml` 会从前端仓库**默认分支**构建前端（`action.yml:34`，正是我们已修复的不可复现点）；`release-docker.yml`/`docker-publish.yml` 推 `ghcr.io/${github.repository}`；`auto-merge-dev-to-main.yml`/`snapshot.yml` 基于上游分支模型 | **删除** |
| `.github/actions/*`（3 个） | 上游 composite actions（build/restore frontend、setup-zig） | **删除** |
| `.github/ISSUE_TEMPLATE/*`（3 个） | 指向上游反馈渠道 | **删除** |
| `LICENSE`、`NOTICE` | MIT 版权声明 + 第三方归属 | **必须保留**（许可要求） |

### 构建与发布现状

- 当前二进制**动态链接 glibc**（`ldd` 显示 `libc.so.6`），而 `Dockerfile` 基于 `alpine:3.21`（musl）
  → glibc 动态二进制在 alpine 中不可用，**发布产物必须解决这一点**。
- 实测 `-extldflags "-static"` 可产出静态二进制，但 **glibc 静态**在 `getaddrinfo`/NSS 上有运行时告警
  （DNS 依赖宿主 glibc 共享库）→ 不作为发布形态。
- 上游用 zig 做 musl 静态（`.github/workflows/release.yml:107-115`），本机原本没有 zig。
- `install-komari.sh` 期望的资产名为 `komari-linux-${arch}`，目标仓库 `zhemed/komari`、`REPO_TAG=0.0.1`。

### 遗留任务

- `.trellis/tasks/00-bootstrap-guidelines`（`trellis init` 播种）仍未完成：5 份 backend spec
  只有 `build-and-pinning.md` 是真实内容，其余是 `To fill` 模板。

## Requirements

- **R1 历史重写**：删除全部上游提交；以"上游 1.4.3 快照"作为新根提交（message 记录来源 `bf6b45ec`），
  把我们现有的 7 个提交重挂到新根上（**保留提交粒度**）；随后强制推送到 `origin`。
- **R2 分支改名**：本地与远端默认分支改为 `main`；删除远端旧的 `komari-1.4.3` 分支。
- **R3 README 重写**：删除 `README.md` 与 `README_zh-cn.md`，写一份我们自己的 `README.md`
  （中文为主）：项目定位与版本线、构建、运行、Docker、数据与备份、刻意偏离上游之处（插件系统已移除）、
  维护文档链接、来源与许可（明确派生自上游 1.4.3 并保留其版权）。
- **R4 上游 CI/模板清理**：按上表删除 `.github/workflows`、`.github/actions`、`.github/ISSUE_TEMPLATE`；
  **保留** `LICENSE`、`NOTICE`。不新建占位 CI。
- **R5 规范补完**：完成 `.trellis/tasks/00-bootstrap-guidelines`（5 份 spec 写入**真实代码取证**内容，
  含 `file:line` 锚点，无占位符），然后 finish + archive。
- **R6 发布 0.0.1**：从重写后的 HEAD 构建**静态**二进制，创建 tag `0.0.1` 与 release；
  资产名与 `install-komari.sh` 期望一致（`komari-linux-amd64`，可行时再加 `komari-linux-arm64`）。
- **R7 文档同步**：`docs/MAINTAINING.md` 增补"历史已于 2026-09-16 重写（不含上游提交）"、
  重写方法与回滚方式、发布流程、`upstream` remote 的用途（backport 用）、以及静态构建要求。
- **R8 验收与交付**：远端历史只含我们的提交；release 资产可下载且能运行；
  全新克隆可离线构建；工作区干净。

## Acceptance Criteria

- **AC1 历史干净**：`git log --oneline main | wc -l` = 我们的提交数（根快照 + 既有提交），
  且 `git log --format='%H %an'` 中不含任何上游提交 hash（如 `bf6b45ec`）与上游作者名。
- **AC2 分支**：`gh repo view zhemed/komari --json defaultBranchRef` 显示 `main`；
  `git ls-remote --heads origin` 只剩 `main`。
- **AC3 README**：仓库根无 `README_zh-cn.md`；`README.md` 中不含上游徽章/`socialify`/Rainyun/1Panel
  链接与上游截图，且包含我们的构建命令与"来源与许可"段落。
- **AC4 上游 CI 清理**：`.github/workflows`、`.github/actions`、`.github/ISSUE_TEMPLATE` 均已不存在；
  仓库内不再有指向 `ghcr.io/komari-monitor` 或"从 komari-web 默认分支构建"的流水线；
  `LICENSE` 与 `NOTICE` 仍在。
- **AC5 规范**：`.trellis/tasks/00-bootstrap-guidelines` 已归档；5 份 spec 文件均为真实内容
  （每份至少 3 个可复核的 `file:line` 锚点，全文无 `TBD`/`To fill`）。
- **AC6 发布**：`gh release view 0.0.1` 存在且资产 `komari-linux-amd64` 可下载；
  下载后的二进制能启动并打印 `Komari Monitor 0.0.1`；`file` 显示静态链接。
- **AC7 可复现交付**：从远端 `main` 全新 `--depth 1` 克隆后，离线构建成功
  （`GOPROXY=off ./scripts/build-komari.sh`）。
- **AC8 收尾**：所有提交已推送；`git status` 干净。

## Technical Notes

- **必须保留 LICENSE/NOTICE**：MIT 许可要求保留版权声明；`NOTICE` 覆盖第三方组件归属。
- **不改 Go module 路径**（`github.com/komari-monitor/komari`）：改名会触及几乎所有 import，
  并使未来从上游 cherry-pick 补丁全部冲突；收益不足以抵偿（见 Out of Scope）。
- **上游 tag 处理**：本地删除这 68 个 tag（让 `.git` 里不再长期留存上游对象）；
  `upstream` remote 保留，需要时 `git fetch upstream --tags` 可重新获取。
- **重写前先备份引用**：推送前建 `backup/pre-rewrite` 分支并把旧 HEAD hash 写入文档，作为回滚依据。
- **二进制必须在重写之后构建**：版本 hash 嵌入二进制，重写会改变所有 hash。

## Out of Scope

- 不重建 CI/CD（删除上游流水线后暂不新建；将来要建是独立决策）。
- 不重命名 Go module 路径（理由见 Technical Notes）。
- 不把上游 1.5.x 的功能/修复 backport 进来。
- 不重写 `docs/` 之外的历史文档内容（仅做必要同步）。
- 不给 arm64 之外的新架构做交叉编译（除非 zig 使 arm64 顺带可行）。
