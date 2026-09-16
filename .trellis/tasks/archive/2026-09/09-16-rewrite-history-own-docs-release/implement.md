# 执行计划：重写历史 + 自有文档 + 首个 release

对应任务：`.trellis/tasks/09-16-rewrite-history-own-docs-release`（需求见 `prd.md`，设计见 `design.md`）。

## Gate 0（前置）

- 必须已获得用户对最终规划摘要的**明确批准**后，再执行 `task.py start`。
- 起点：HEAD = `77f36da`，工作区干净，`origin` = `zhemed/komari`，远端只有 `refs/heads/komari-1.4.3`。
- 环境：`go1.26.6` / `node v22.23.2` / `gcc 11.4.0` / `docker 29.7.2`；zig 0.16.0 已放在
  `.build/tools/zig-x86_64-linux-0.16.0/`（不入库）。沙箱下需
  `ZIG_GLOBAL_CACHE_DIR` / `ZIG_LOCAL_CACHE_DIR` 与 npm/GOPATH 一并重定向到 `.build/`。

## 实施清单（按序，每步带门禁）

### 1. 补完 bootstrap 规范（遗留任务）— ✅ 已完成（2026-09-16）

- 已完成：5 份 backend spec 由子代理按真实代码取证填充（942 行、307 个已校验锚点，无占位符），
  我逐份复核抽查锚点真实性通过；`index.md` 状态改为 Filled、语言声明改为中文。
- 顺带修复子代理发现的真实缺陷：`web/public/.gitignore:4` 引用已重命名的
  `docs/MAINTAINING-1.4.3.md` → 改为 `docs/MAINTAINING.md`（上一任务门禁的漏网处）。
- 归档：`task.py archive 00-bootstrap-guidelines` 完成。
- 产出提交：`e941b79`（规范填充）、`e565a7b`（归档）。

### 2. README 重写 + 上游资产清理 + 文档同步

- 动作：
  1. `git rm README.md README_zh-cn.md`，新写 `README.md`（我们的）：项目定位与版本线（0.0.1 起自维护）、
     构建、运行（systemd/Docker）、数据与备份、刻意偏离上游之处（插件系统已移除、更新检查指向自有仓库）、
     维护文档链接、**来源与许可**（派生自上游 `komari@1.4.3` + `komari-web@1.4.3`，保留上游版权）。
  2. `git rm -r .github/workflows .github/actions .github/ISSUE_TEMPLATE`；**保留** `LICENSE`、`NOTICE`。
  3. `docs/MAINTAINING.md` 增补：历史重写说明（2026-09-16，旧 HEAD `77f36da` 与备份分支名）、
     重写方法与回滚、发布流程（含静态构建）、`upstream` remote 用途、静态构建与 Docker 的关系。
  4. `.trellis/spec/backend/build-and-pinning.md` 同步静态构建与"无 CI、手动发布"。
  5. **处理 `.github/**` 删除后产生的失效锚点**（实测 8 处，删目录后必须同步改写为
     "上游仓库的 …（本仓库已删除该目录）"，否则规范里会留下死引用）：
     `build-and-pinning.md:5,26`、`index.md:33`、`quality-guidelines.md:15,35,212`、`docs/MAINTAINING.md:25,40`。
- 门禁：`grep -rn "socialify\|rainyun\|1panel" README.md` 无结果；
  `grep -rln "ghcr.io/komari-monitor\|komari-web" .github/ 2>/dev/null` 无结果（目录已删）；
  `ls LICENSE NOTICE` 仍在。
- 回滚：`git checkout -- README.md docs/ .trellis/spec/ && git checkout HEAD~ -- .github/`（或 `git revert`）。

### 3. 静态构建支持（发布前提）

- 动作：`scripts/build-komari.sh` 增加 `KOMARI_STATIC=1` 与 `KOMARI_GOARCH`（默认 amd64）：
  静态模式用 `CC="zig cc -target <arch>-linux-musl"` +
  `-ldflags '-s -w -linkmode external -extldflags "-static"'`；zig 缺失或不可用时**明确报错**，
  不静默退化成动态链接（否则 alpine 部署会失败）。
- 门禁：`KOMARI_STATIC=1 ./scripts/build-komari.sh` 产出 `file` 显示 `statically linked`；
  运行该二进制并 `curl /install` 返回 200；`KOMARI_GOARCH=arm64 KOMARI_STATIC=1` 亦产出 arm64 静态二进制。
- 回滚：`git checkout -- scripts/build-komari.sh`。

### 4. 提交本轮内容 + 建立回滚引用

- 动作：提交步骤 1-3 的全部改动；`git branch backup/pre-rewrite <当前 HEAD>` 作为本地回滚锚点。
- 门禁：`git status` 干净；`git rev-parse backup/pre-rewrite` 有输出；
  `go build ./... && go vet ./... && go test ./...` 全绿（构建脚本改动不影响 Go 代码，但需确认）。
- 回滚：`git reset --hard backup/pre-rewrite`。

### 5. 历史重写

- 动作（`design.md` D1）：
  ```bash
  UP_TREE=$(git rev-parse bf6b45ec3abfc56bba5e9223650a47a72f665371^{tree})
  NEW_ROOT=$(git commit-tree "$UP_TREE" -m "chore: import komari 1.4.3 (upstream bf6b45ec) as our 0.0.1 code snapshot")
  git rebase --onto "$NEW_ROOT" bf6b45ec3abfc56bba5e9223650a47a72f665371 <当前分支>
  ```
  随后删除本地上游 tag：`git tag | xargs git tag -d`（仅删本地，未推送过）。
- 门禁：
  - `git log --format='%H %an' | grep -c bf6b45ec` 为 0；
  - `git log --oneline | wc -l` = 1（根快照）+ 我们的提交数（预期 8-9）；
  - `git log --format=%an | sort -u` 不含上游作者（如 `Akizon77`）；
  - `git diff backup/pre-rewrite HEAD --stat` **为空**（证明重写没有改变任何代码内容——这是最关键的门禁）；
  - `git tag | wc -l` = 0。
- 回滚：`git reset --hard backup/pre-rewrite`（重写全部作废，旧历史仍在备份引用上）。

### 6. 分支改名 + 强制推送

- 动作：
  ```bash
  git branch -m main
  git push --force origin main
  gh api -X PATCH repos/zhemed/komari -f default_branch=main
  git push origin --delete komari-1.4.3
  git branch --set-upstream-to=origin/main main
  ```
- 门禁：`gh repo view zhemed/komari --json defaultBranchRef` 为 `main`；
  `git ls-remote --heads origin` 只剩 `refs/heads/main`；`git status -sb` 与 origin/main 同步。
- 回滚：把 `backup/pre-rewrite` 重新推为新分支（`git push origin backup/pre-rewrite:main --force`）。

### 7. 发布 0.0.1

- 动作：在重写后的 HEAD 上 `KOMARI_STATIC=1 ./scripts/build-komari.sh`（amd64），
  需要时再构建 arm64；把二进制重命名为 `komari-linux-amd64`（arm64 同理）；
  `git tag 0.0.1 && git push origin 0.0.1`；
  `gh release create 0.0.1 --title "0.0.1" --notes "..." komari-linux-amd64 [komari-linux-arm64]`。
- 门禁：`gh release view 0.0.1` 列出预期资产；`curl -fL` 下载资产后 `file` 显示静态、能启动并打印 `0.0.1`。
- 回滚：`gh release delete 0.0.1 --yes && git push origin --delete 0.0.1`；重新发版需递增版本号（不要覆盖同一 tag）。

### 8. 端到端验收（AC1-AC8）

1. **AC1/AC2**：按第 5、6 步门禁复核。
2. **AC3/AC4**：README 与 `.github` 检查（第 2 步门禁）。
3. **AC5**：`task.py list-archive` 含 `00-bootstrap-guidelines`；spec 无占位符。
4. **AC6**：`gh release view` + 下载资产实测运行。
5. **AC6b（Docker 实证）**：用发布资产构造 `komari-linux-amd64` 并 `docker build` 出镜像、
   `docker run` 起容器，`curl` 容器内服务 → 验证 alpine + 静态二进制的完整路径可用
   （这是选择 musl 静态的直接目的，必须实测）。
6. **AC7**：`git clone --depth 1` 远端 `main` → `GOPROXY=off ./scripts/build-komari.sh` 成功。
7. **AC8**：`git status` 干净、`git ls-remote` 与本地上限一致。
- 回滚：各步骤回滚点；Docker 测试使用本地 `docker build`，不推镜像仓库。

### 9. 收尾

- 动作：`add_session.py` 记录本次会话（**必须写明**：历史已于 2026-09-16 重写，
  早于该时点的 commit hash（`18305a9`、`77f36da` 等）指向已废弃历史，备份分支名为 `backup/pre-rewrite`）。
- 门禁：journal 提交已推送；`git log --oneline` 只含我们的提交。

## 复核门（`task.py start` 之前）

- PRD 的 R1-R8 与 AC1-AC8 一一可测；无阻塞性未决问题。
- `design.md` 的 D1-D6 与本计划步骤 1-9 对应。
- 破坏性操作（重写 + 强制推送）已在设计中给出方法与回滚，并由用户在规划摘要中确认。
