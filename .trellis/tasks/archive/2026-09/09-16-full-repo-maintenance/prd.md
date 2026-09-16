# 全面维护：说明口径换成我们自己的版本 + 仓库自检工具

## Goal

对仓库做一次全面维护：对外说明（GitHub 描述、README、维护文档）改成**我们自己的版本**，
不再以"上游 1.4.3 的 fork + pin + vendored frontend"自我描述；并把"仓库是否自洽"的机械检查
固化成可重复执行的工具。

## Requirements

1. GitHub 仓库描述与 topics 换成我们自己的口径（描述已不含 pin/vendored frontend 等过期说法）。
2. README / docs/MAINTAINING.md / 安装脚本 / 构建脚本里的"本 fork"口径改为"本仓库 / 我们独立维护的发行版"，
   上游只标注为**历史来源**；安装界面里 `(komari <tag> fork)` 改成 `(zhemed/komari <tag>)`。
3. LICENSE 保留上游版权行，补上我们的改动版权行（GitHub 许可识别仍为 MIT）。
4. 新增 `scripts/check-repo.sh`：快速检查（版本字面量、文档引用与锚点、脚本语法、脏文件、
   密钥扫描、前端产物哈希、构建期无克隆上游）+ `--full`（Go 门禁、离线构建、agent 门禁）。
5. 用该脚本扫全仓，修掉所有真实不一致；把脚本接进 MAINTAINING §3.4 发布清单与规范 §2。

## Acceptance Criteria

- [x] `gh repo view` 描述与 topics 为新口径；homepage 保持为空（与其他仓库一致）
- [x] `./scripts/check-repo.sh` 与 `--full` 均全绿
- [x] 体检抓到并修掉 9 处真实不一致（6 个规范文件的已删除模块残留、测试分布/产物文件数/Debug 用法
      三处数字漂移、1 处 file:line 锚点漂移）
- [x] 发版清单第 4 步 = `./scripts/check-repo.sh --full` 必须全绿
- [x] 不产生新 release（无产物变化），0.0.5 仍是当前版本

## Notes

- 工具坚持"宁少报不误报"：只查我们自己的构建输入与目录，跳过 import 路径/HTTP 路由/slug、
  同行注明"删除/历史"的引用、删除线段落。
- 本轮不动的两处（属产品内容，需用户决定）：面板默认站点名/描述（`internal/config/settings.go`
  的 `Komari` / `A simple server monitor tool.`）、Go module path 仍是 `github.com/komari-monitor/komari`。
