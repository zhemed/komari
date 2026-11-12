# Journal - zhemed (Part 1)

> AI development session journal
> Started: 2026-11-12

---



## Session 1: 发布产物收敛：只维护 linux/amd64，其余目标注释停用
<!-- trellis-session: v=2 fp=c328bee508486c07 -->

**Date**: 2026-11-12
**Task**: 发布产物收敛：只维护 linux/amd64，其余目标注释停用
**Branch**: `main`

### Summary

按用户 2026-11-12 定稿，把 release 资产与容器镜像的构建矩阵从全平台收敛为只维护自有服务器目标 linux/amd64，其余目标注释保留（恢复=取消注释）。产出侧：build-agent.sh 的 OS_LIST/ARCH_LIST 收敛为 linux/amd64（14→1）；build-komari.sh 停用 arm64 静态分支；gen-release-sums.sh 只算 amd64；两个镜像脚本 PLATFORMS 与平台→资产映射收敛为 linux/amd64。口径侧：docs/MAINTAINING.md（§1/§3.3/§3.4/§11/§12）与 .trellis/spec/backend/build-and-pinning.md 同步，多平台事故记录标注为历史。以后一次发版 4 个资产（+2 源码包 = GitHub 显示 6）。

### Main Changes

- scripts/build-agent.sh: 构建矩阵收敛为 linux/amd64，其余 13 个目标注释停用
- scripts/gen-release-sums.sh: ASSETS 只留 komari-linux-amd64
- scripts/build-komari.sh: arm64 静态构建分支注释停用（明确报错）
- scripts/build-server-image.sh / build-agent-image.sh: 默认单平台 linux/amd64，arm64/armv7 映射注释停用
- docs/MAINTAINING.md + spec/backend/build-and-pinning.md: 发布口径与资产数同步（4 个资产）

### Git Commits

| Hash | Message |
|------|---------|
| `f7ac322` | chore(build): 发布产物收敛到 linux/amd64，其余目标注释停用 [task:release-targets-prune] |

### Testing

- [OK] check-repo.sh 快速检查全绿（1-8、12-16 项）
- [OK] 假工具链功能验证：build-agent 只产 komari-agent-linux-amd64；build-komari arm64 明确报错；两镜像 --push 仅 linux/amd64，--platforms arm64/armv7 明确报错
- [OK] gen-release-sums.sh 夹具只生成 1 行

### Status

[OK] **Completed**

### Next Steps

- 本机无 go/zig/docker：check-repo.sh --full 第 9-11 项需在发版机补跑；下次发版按 §3.4 只产 4 个资产


## Session 2: 本机部署 Komari 本地维护实例（二进制 + systemd）
<!-- trellis-session: v=2 fp=30f64f5adbcc7673 -->

**Date**: 2026-11-12
**Task**: 本机部署 Komari 本地维护实例（二进制 + systemd）
**Branch**: `main`

### Summary

按用户 2026-11-12 定稿，在本机（DSH 主机，linux/amd64）用已发布 0.0.25 二进制 + systemd 部署全新独立实例：下载 release komari-linux-amd64 并用 komari-SHA256SUMS 校验（sha256 b40c781f… 一致）；安装到 /opt/komari；unit komari.service（Restart=always、WorkingDirectory=/opt/komari、仅绑 LAN 地址:25774、开机自启）；systemd 已 enabled+active；通过 /api/install/complete 完成首装（管理员 admin、站点 Komari Local、监控库 ./data/metrics.db），初始凭据只写本机 600 文件、不进对话与仓库；/api/version=0.0.25、/api/login=200、GET /=200。运行手册在仓库外（含启停/日志/升级/备份/回滚/卸载）。VPS 生产零改动。

### Main Changes

- 本机部署（仓库外）：/opt/komari + /etc/systemd/system/komari.service + 运行手册 /root/komari-local/RUNBOOK.md
- 仓库内仅任务记录（.trellis），无代码/文档改动；站点专属值不写入公开仓库

### Git Commits

(No commits - planning session)

### Testing

- [OK] sha256sum -c（release 清单）→ komari-linux-amd64: OK；file → statically linked
- [OK] systemctl is-enabled= enabled / is-active= active；重启后恢复 active
- [OK] curl /api/version → 0.0.25；/api/install/complete → success；POST /api/login → 200；GET / → 200

### Status

[OK] **Completed**

### Next Steps

- 本机实例可直接使用；下次发版（0.0.26）按运行手册升级流程更新；VPS 生产保持不动


## Session 3: 修复主题导入/更新对 GitHub release 下载 URL 的误判（本机实例验证通过）
<!-- trellis-session: v=2 fp=658ada246aff323e -->

**Date**: 2026-11-12
**Task**: 修复主题导入/更新对 GitHub release 下载 URL 的误判（本机实例验证通过）
**Branch**: `main`

### Summary

本机实例实测后台「主题→导入/更新」对 https://github.com/zhemed/komari/releases/download/theme-latest/ink-theme.zip 报 400 not a valid zip：根因是 web/api/admin/theme.go 的 isGitHubRepoURL 把任何 github.com 且路径≥2段的 URL 当仓库地址，于是 release 下载 URL 被送去 GitHub API 取 latest release 的第一个资产（服务器二进制）。修复：只认仓库根地址 /owner/repo[.git][/]，具体资源路径返回 false 走直接下载；新增表驱动 TestIsGitHubRepoURL（5 正例 + 9 反例）。本机首次安装 Go 1.26.11（go.dev 官方包 + sha256）后：go build/vet/test 全绿、check-repo.sh --full 16 项全通过（Go 门禁/离线构建/agent 门禁首次跑通）。用修复版二进制（hash 664efff）替换本机实例：主题更新 200 主题更新成功、主题导入 200（已覆盖同名主题），ink 1.0.4 资源全部 200。旧二进制已备份。生产未动。

### Main Changes

- web/api/admin/theme.go: isGitHubRepoURL 收紧为仓库根地址判定（releases/download、archive、tree、blob 等不再误判）
- web/api/admin/theme_test.go: 新增 TestIsGitHubRepoURL 表驱动回归测试

### Git Commits

| Hash | Message |
|------|---------|
| `4a0632b` | fix(theme): 修正 GitHub release 下载 URL 被误判为仓库地址 [task:theme-import-url-fix] |

### Testing

- [OK] go test ./web/api/admin/ 全 PASS（含 14 个子用例）；go build ./...、go vet ./... 通过
- [OK] 本机实例：修复前 import/update 均 400 not a valid zip；修复后 update 200 主题更新成功、import 200 主题导入成功（已覆盖同名主题）
- [OK] check-repo.sh --full：16 项全部通过（首次在本机跑通 Go 门禁/离线构建/agent 门禁）

### Status

[OK] **Completed**

### Next Steps

- 修复随下次发版（0.0.26）生效；生产面板届时才具备主题更新能力；本机实例已是修复版


## Session 4: 本机注册为本地实例节点（komari-agent 0.0.25）+ ink 主题部署与主题导入 bug 修复落地
<!-- trellis-session: v=2 fp=5e12b4ad79750e82 -->

**Date**: 2026-11-12
**Task**: 本机注册为本地实例节点（komari-agent 0.0.25）+ ink 主题部署与主题导入 bug 修复落地
**Branch**: `main`

### Summary

接续上一段：本地实例（/opt/komari，0.0.25）此前只有内置默认主题；本次部署 ink 1.0.4（release theme-latest 的 ink-theme.zip，解压到 data/theme/ink + set 接口激活，首页已是 ink 渲染）。随后用 admin:addClient 注册节点 ubuntu-dsh，下载 release 0.0.25 的 komari-agent-linux-amd64（sha256 校验）装到 /opt/komari-agent + komari-agent.service（-e 本机实例 -t token，Restart=always、开机自启）；日志 Basic info uploaded successfully，admin:listClients 与 /api/nodes 均可见，首页节点卡片渲染真实数据。另：上一任务的修复版二进制已部署到实例（hash 664efff），主题更新/导入在本机实测 200；旧二进制备份保留。凭据只在本机 600 文件，未进对话/仓库；生产未动。三网行待配 ping 任务。

### Main Changes

- 本机实例（仓库外）：ink 主题 1.0.4 部署激活；komari-agent 0.0.25 安装并注册节点 ubuntu-dsh；运行手册补充主题与节点章节
- 仓库内仅 .trellis 任务记录（local-node-agent 归档 + journal）

### Git Commits

(No commits - planning session)

### Testing

- [OK] 主题：/themes/ink/komari-theme.json 200、/assets/* 200、/admin-app/index.html 200、首页 200；截图确认 ink 渲染
- [OK] agent：sha256 OK、statically linked、服务 enabled+active、Basic info uploaded successfully
- [OK] 节点：admin:listClients → ubuntu-dsh/0.0.25；/api/nodes 返回；首页节点卡片有 CPU/内存/流量数据

### Status

[OK] **Completed**

### Next Steps

- 三网行需在后台加 3 个 ping 任务（电信/联通/移动 目标）；修复随 0.0.26 发版生效到生产
