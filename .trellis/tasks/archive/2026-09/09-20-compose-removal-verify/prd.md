# 核实 compose 彻底移除 + 0.0.19 的构成

## 用户问

「compose 彻底移除了吗，咱们的 0.0.18 居然还是为了这个专门维护的。下个版本应该就没有了吧」

## 核实结果（证据）

1. **源码/脚本里 compose 部署逻辑：零残留。**
   `git grep -inE 'com.docker.compose|docker-compose|compose\.ya?ml|SyncCompose'`
   （排除 `.trellis` 与 check-repo 的禁令模式）= **0 命中**。`compose.go`、`selfid.go`、
   `install-compose.sh` 都不在仓库（回滚时删除）。
2. **候选 0.0.19 二进制（本地构建，`KOMARI_VERSION=0.0.19`）扫描对比**：

   | 符号/字符串 | 候选 0.0.19 | 在跑的 0.0.18 |
   |---|---|---|
   | `com.docker.compose` | **0** | 2 |
   | `SyncComposeImage` | **0** | 2 |
   | `compose.go` | **0** | 3 |
   | `DetectSelfContainerID` | 1 | 2 |
   | 版本串 | `0.0.19` | `0.0.18` |
3. **两处非 compose-部署的残留（发现即报）**：
   - 前端面板一句提示文案（`frontend/src/components/admin/AdminPanelBar.tsx:690` +
     `frontend/src/i18n/locales/*.json:1261` zh_CN/zh_TW/en）写着
     "重建容器（`docker rm + run` / `compose up`）会退回镜像里的版本"——**提到了 compose**。
     改它要重建前端产物并更新 `frontend-build.env` 的目录树哈希（自检第 6 项）。
   - `internal/dockerapi/recreate.go` 的 `DetectSelfContainerID` 是**挂 socket 时的容器重建能力**
     （0.0.13/0.0.17 时期的既有功能，不是 compose 专用）：host 挂 socket 时面板会显示
     "立即升级（重建容器）"。要不要连它一起删，需用户决定（删=去掉一个功能）。

## 0.0.18 为什么"为它专门维护"

0.0.18 的**代码内容** = compose 部署线（install-compose.sh、compose tag 自动同步、
host 网络自识别修复）+ 重启策略定稿 + README 压缩。发布后按用户指令从**源码**回滚，
但用户保留发布物并部署了它。所以：**已发布的 0.0.18 资产与镜像里永远含 compose 代码**，
而仓库 `main` 已经不含——这是"发行物 vs 分支"的既定差异，不是不一致。

## 下一个版本（0.0.19）

- 从当前 `main` 构建 → **不含任何 compose 代码**（上面第 2 条的扫描就是证据）。
- 版本号必须从 0.0.19 起（0.0.18 已发布）。
- 注意：0.0.19 的源码基线是"回滚后的 0.0.17"，所以它比 0.0.18 **少**了那几个 compose 时代
  的产品化改动（重命名/写说明/压缩 README 等）——发版前建议先对 0.0.18 做一次真机端到端升级测试。

## Acceptance Criteria

- [x] 源码零残留（grep 0 命中）
- [x] 候选二进制扫描（compose 符号 0；对照组 0.0.18 有）
- [x] 找出并报告两处非部署残留（前端文案、socket 重建模式）供用户决定
- [x] 说明 0.0.18 保留的后果与 0.0.19 的构成
