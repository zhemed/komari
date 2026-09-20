# 移除面板「文档」入口（菜单 + 帮助按钮）并发 0.0.20

## 用户指令

「这个文档相关的所有内容是否可以彻底移除」→ 盘点后选定：**只清面板两处** + **发 0.0.20**。

## 盘点结论（只读阶段）

面板可见的「文档」入口共两处，另有 agent 与维护文档各一处（本次不动）：

| # | 位置 | 处置 |
|---|---|---|
| ① | `frontend/src/config/menuConfig.json` 底部菜单项 `common.documentation` → 上游文档站 | **删除** |
| ② | `frontend/src/pages/admin/settings/general.tsx` 自动发现旁的「帮助」按钮 → `…/install/agent-ad.html` | **删除** |
| ③ | `agent/cmd/warn_windows.go` Windows 卸载提示的 Help 按钮 | 未动（不在面板上；动它要重发 agent） |
| ④ | `docs/MAINTAINING.md`、`frontend/README.md` 的文字提及 | 未动（维护者资料；含"别照抄上游文档"的提醒） |

i18n：删除 `common.documentation`、`common.help`（5 个语言包各 2 行，逐行删无格式噪声）。

## 生产验收（全部实测）

| 项 | 结果 |
|---|---|
| 面板版本 | `version=0.0.20`，`hash=7f1ec2ceb154…`（= 提交） |
| 生产二进制 | sha256 `0b54fcbdb624…` 与 release 资产/镜像一致 |
| 「文档」痕迹 | 生产二进制内 `komari-document.pages.dev` / `common.documentation` / `"documentation"` 均 **0 处** |
| 面板可用性 | `/admin/dashboard` HTTP 200；agent `online (v2)`；本次启动 0 条 error |
| release 0.0.20 | 18 资产（含显式命名补传的 `komari-agent-SHA256SUMS`），digest 与本地产物逐一一致 |
| 镜像 | `komari:0.0.20`/`:latest` 已推，实测 `--help` 报 0.0.20 |
| 回滚点 | 旧二进制 `komari.backup.20260920_030111` |
| 自检 | `check-repo.sh --full` 全绿（15 项） |

## 备注

- 本次严格遵守"**先提交后构建**"（上次 0.0.19 违反过，导致产物内嵌 hash 与提交不一致，重新构建覆盖过）。
- release 资产上传时一次性把 `komari-agent-SHA256SUMS` 用 `路径#名称` 显式指定，避免再次漏传。
