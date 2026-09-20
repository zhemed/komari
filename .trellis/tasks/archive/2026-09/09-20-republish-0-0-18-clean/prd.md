# 走 C：用干净代码重发 0.0.18，替换含 compose 的那版

## 用户选择

「嗯，代码已经回滚了，那我们走c吧」——即：保留 0.0.18 这个版本号，但用**已移除 compose 的代码**
重新构建发布，把原先那版（含 compose 安装脚本 + tag 自动同步 + label 检测）换掉。

## 关键事实（动工前查清）

- 生产**不是容器形态**，是 systemd 二进制：`ExecStart=/opt/komari/komari`（面板报 0.0.18，
  sha256 `b5b024ac…` = 旧 0.0.18 资产，即**含 compose 那份**）。
- 回滚只回到了"main 分支代码"：`tag 0.0.18` 里仍有 `install-compose.sh`、`internal/upgrade/compose.go`、
  `internal/dockerapi/selfid.go`。

## 执行

1. **顺手补做面板文案清理**：新构建里一开始还扫到 `compose up` 2 处——是 panel 提示文案
   （回滚把它的删除也退掉了）。按用户上次的决定删除组件块 + 5 个语言包 `inplace_hint` 键，
   前端产物重建，`FRONTEND_TREE_SHA256` → `3b1c226b…`。
2. **构建**：服务器静态 amd64/arm64（`--help` 报 `Komari Monitor 0.0.18`，hash `bd3946a0…`）+ agent 14 平台
   + 两个 SHA256SUMS；扫描 `com.docker.compose` / `SyncComposeImage` / `compose up` / `inplace_hint` **全 0**。
3. **重发 release 0.0.18**：覆盖 18 个资产（digest 与本地产物逐一核对一致），标题与说明改为
   「0.0.18 — 容器一键升级与 host 网络自识别修复（无 compose 版）」，说明里写明"同一版本号重新构建"的原因。
4. **镜像**：`komari` 与 `komari-agent` 的 `:0.0.18` 与 `:latest` 均覆盖为干净内容。
5. **生产换二进制**：走用户验收过的部署命令（`KOMARI_TAG=0.0.18 bash install-komari.sh` → 菜单 2），
   旧二进制自动备份为 `komari.backup.20260920_021345`。

## 验收（全部实测）

| 项 | 结果 |
|---|---|
| 生产二进制 | sha256 `3c409e6abbac6a60`，与 release 资产**完全一致** |
| 面板 | `version=0.0.18`，`hash=bd3946a0…`（= 源码提交） |
| 生产二进制 compose 痕迹 | `com.docker.compose` 0、`compose up` 0 |
| 日志 | 本次启动后 0 条 error/fail/panic；agent `online (v2)`；节点数据完好 |
| 线上镜像 | `docker run ghcr.io/zhemed/komari:latest --help` → 0.0.18，compose 0 命中 |
| 回滚点 | 旧（含 compose）二进制仍在 `komari.backup.20260920_021345` |

## 遗留说明

同一版本号 0.0.18 在线上先后存在过两个构建：先发布的是含 compose 版（已无资产），现在是干净版。
以后审计若看到 0.0.18 的两种摘要，以本次为准（`.build/rel-notes-0.0.18.published-backup.md`
保留了旧说明原文）。
