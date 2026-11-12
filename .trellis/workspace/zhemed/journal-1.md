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
