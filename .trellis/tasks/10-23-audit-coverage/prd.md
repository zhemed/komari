# 补齐审计覆盖：frontend / agent / 容器镜像

## 需求
1. 安装 trivy（用户已授权补工具）；
2. 审计内核自带 `frontend/`（React 管理端，npm 生态）：区分运行期依赖 vs devDependency，运行期高危做最小升级并构建验证；
3. 审计 `agent/` 模块：`go mod verify` / `go list -m -u all` / `govulncheck ./...`，可修项最小升级 + `go build` 验证；
4. 审计容器镜像：`trivy image` 扫自建 `komari-own:0.0.24-own.2` 与官方 `ghcr.io/zhemed/komari:0.0.24`（OS 包与依赖），`trivy fs` 扫仓库；对比差异并给出可执行的最小修法；
5. 仅做最小修复；若镜像变化则备份→构建 own.3→版本化脚本重建→部署后核对；否则不部署；
6. 结果写入 `ops-docs/INDEPENDENCE.md` §6.6，提交并推送。

## 验收标准
- 每个子项贴命令与结论行；修复项有"升级→构建→验证"证据；
- 不改业务代码逻辑；不动 DB/nginx/证书/其它容器；
- 若涉及部署：`/api/version` 正常、面板双次 200、端口 loopback、DB 周期增长、主题入口与「本周期流量」不变。

## 非目标
- 不为清零而大改 Dockerfile 或更换基础镜像家族（如需更换先回报）。
