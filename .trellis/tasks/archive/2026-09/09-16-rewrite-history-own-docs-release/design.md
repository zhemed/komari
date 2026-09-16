# 技术设计：重写历史 + 自有文档 + 首个 release

对应任务：`.trellis/tasks/09-16-rewrite-history-own-docs-release`（需求见 `prd.md`）。

## 1. 关键设计

### D1 历史重写方法：造"上游快照"根提交 + rebase 重挂我们的提交

选它而不是"压成单个提交"，理由是**保留我们自己的提交粒度**，同时让根提交语义清晰。

```bash
# 1) 用上游 1.4.3 的 tree 造一个新根（作者是我们的身份，message 记录来源）
UP_TREE=$(git rev-parse bf6b45ec3abfc56bba5e9223650a47a72f665371^{tree})
NEW_ROOT=$(git commit-tree "$UP_TREE" -m "chore: import komari 1.4.3 (upstream bf6b45ec) as our 0.0.1 code snapshot")
# 2) 把我们全部提交重挂到新根上
git rebase --onto "$NEW_ROOT" bf6b45ec3abfc56bba5e9223650a47a72f665371 komari-1.4.3
```

- 因为 `NEW_ROOT` 的 tree 与 `bf6b45ec` **完全相同**，`rebase` 应用的是我们对 1.4.3 的增量补丁，
  应**无冲突**；若出现冲突，说明我们对上游文件的改动与既有假设不符，需要人工复核（视为门禁失败）。
- 重写后所有提交 hash 变化 → **二进制必须在重写之后重新构建**（版本 hash 嵌入二进制）。
- 重写前后的对应关系会记录在 `docs/MAINTAINING.md`；
  `.trellis/workspace/komari/journal-1.md` 中早于重写的 hash 将指向废弃历史，需在新 journal 中说明。

### D2 清理上游 tag 与引用

本地有 68 个上游 tag（`0.0.3`…`1.5.0-fix1`）指向上游提交，会让上游对象继续留在 `.git` 中。
处理：**删除这些 tag**（它们从未推送到 origin），保留 `upstream` remote——
需要 backport 时 `git fetch upstream --tags` 即可重新获取。

注意：上游历史上存在 `0.0.3`–`0.0.9` 这类 tag，与我们的 `0.0.x` 版本线**数值区间重叠**。
我们只推送自己的 tag（`0.0.1` 起），且上游 tag 已从本地删除，不会造成混淆；
文档中需写明"我们的 0.0.1 与上游历史上的 0.0.x 无关"。

### D3 静态构建：优先 zig（musl），glibc 动态为退路

- 现状：本机 `gcc` 可编译，产物**动态链接 glibc**（`ldd` 显示 `libc.so.6`）；
  而 `Dockerfile` 基于 `alpine:3.21`（musl）——**glibc 动态二进制在 alpine 里跑不起来**。
- 实测 `-extldflags "-static"` 可产出 `not a dynamic executable`，但**glibc 静态**在
  `getaddrinfo`/NSS 上有运行时告警（DNS 解析依赖宿主 glibc 的共享库）→ 不适合作为发布产物。
- 因此：用 **zig 的 musl 目标**做静态构建（上游 `.github/workflows/release.yml:107-115` 正是如此），
  这也是交叉编译 arm64 的前提。若 zig 在本机不可获得 → 退化为 glibc 动态构建，
  **同时把 `Dockerfile` 基础镜像从 alpine 改为 debian-slim**，保证 Docker 路径仍可用。
- `scripts/build-komari.sh` 增加 `KOMARI_STATIC=1` 开关（默认仍走本机 gcc 动态构建，开发迭代快）。

### D4 保留 LICENSE / NOTICE，README 中显式声明来源

MIT 许可要求保留版权声明；`NOTICE` 还列了第三方组件归属。二者**必须保留**。
README 增加"来源与许可"段落，写明派生自上游 `komari@1.4.3`（`bf6b45ec`）与
`komari-web@1.4.3`（`4a74e8a8`），并保留上游版权行。

### D5 分支改名与历史重写一次推送

把重写后的分支**直接以 `main` 推送**并设为默认分支，然后再删除旧的 `komari-1.4.3` 远端分支，
避免出现"默认分支指向即将消失的名字"的中间态：

```bash
git branch -m komari-1.4.3 main
git push --force origin main
gh api -X PATCH repos/zhemed/komari -f default_branch=main
git push origin --delete komari-1.4.3
```

### D6 上游 CI / 模板清理边界

删除 `.github/workflows/*`、`.github/actions/*`、`.github/ISSUE_TEMPLATE/*`，理由：

- `release.yml` 会**从前端仓库的默认分支**构建前端（`action.yml:34`），正是我们已修复的不可复现问题；
- `release-docker.yml`/`docker-publish.yml` 推送 `ghcr.io/${github.repository}`，对我们是错误的镜像名；
- `auto-merge-dev-to-main.yml`、`snapshot.yml` 等基于上游分支模型，对我们无意义；
- Issue 模板指向上游的反馈渠道。

本仓库当前**没有** CI（本地构建 + `scripts/`），故直接删除，不新建占位 CI（新建 CI 属于另一次决策）。

## 2. 数据流与契约（本次涉及）

- 发布契约：`install-komari.sh` 从 `https://github.com/${REPO}/releases/download/${REPO_TAG}/komari-linux-${arch}`
  下载 → release 资产名必须与之一致（当前 `REPO=zhemed/komari`、`REPO_TAG=0.0.1`）。
- 版本契约：`CurrentVersion=0.0.1` + `VersionHash=<重写后 HEAD>`；升级备份按该标识判定。
- 构建契约：`web/public/defaultTheme/` 必须存在（`//go:embed`），vendor 产物已在仓库内。

## 3. 权衡

| 选择 | 收益 | 代价 |
|---|---|---|
| 重写历史（造根 + rebase） | 仓库里只有我们的提交；`git log`/`git blame` 干净 | 强制推送；已有克隆需重新获取；上游逐行归属不再体现在历史中（改由 LICENSE/NOTICE/文档体现） |
| 保留我们的 7 个提交（而非压成 1 个） | 保留变更粒度与提交信息 | 重写后 hash 全变，旧引用（journal 等）失效，需说明 |
| 删上游 CI 而非留用 | 避免误用会产生错误产物的流水线 | 暂无自动化；发布需手动执行 `scripts/` |
| 用 zig 做静态发布 | 与上游分发形态一致；兼容 alpine Dockerfile；可交叉 arm64 | 新增工具链依赖（约 50MB，放 `.build/tools/`，不入库） |
| **不改** Go module 路径 | 保住未来 cherry-pick 上游修复的能力 | 仓库名与 module 名不一致（`github.com/komari-monitor/komari`），见 Out of Scope |

## 4. 风险与回滚

- **强制推送不可逆地改变远端历史**：回滚方式是保留重写前的分支引用
  （推送前打一个本地备份 ref：`git branch backup/pre-rewrite <旧 HEAD>`，以及记录旧 HEAD hash 到文档），
  必要时可把旧历史重新推回。
- **DNS/静态链接**：若走 glibc 静态回退，必须同步改 Dockerfile，否则 alpine 部署会失败；
  验收阶段用 `docker run` 实测最稳（不实测则必须在文档中标注该限制）。
- **release 资产 hash**：二进制内嵌重写后的提交 hash；若之后继续改代码，
  应发新版本号而不是覆盖同一 tag。
- **Trellis journal 中的旧 hash**：在本次 journal 中显式记录"历史于 2026-09-16 重写"及旧 HEAD hash。
