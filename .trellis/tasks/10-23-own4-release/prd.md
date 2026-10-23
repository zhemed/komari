# 发布 own.4：重建内嵌 admin 前端 + trixie 基础镜像 + 部署

## 需求
1. 跑 `scripts/build-frontend.sh` 重建 `web/public/defaultTheme`（让 frontend 依赖修复真正进产物），更新 `scripts/frontend-build.env` 的哈希门禁并记录变更；验证自检通过、frontend `npm audit` 仍 0；
2. `deploy/Dockerfile.local` 基础镜像 `debian:bookworm-slim` → `debian:trixie-slim`（钉 digest + 注释），其余构建逻辑不变；
3. `KOMARI_OWN_SUFFIX` → own.4，`./build.sh` 出镜像；`trivy image`（HIGH/CRITICAL）确认 Go 0、OS 降到 ~43 且全部无修复版；`trivy fs` 无新增；
4. 备份（DB 在线 backup + inspect + own.3 脚本）→ 新建 own.4 启动脚本（与 own.3 逐字一致仅镜像 tag）→ save/load → 重建容器 → 核对（/api/version hash、Restarts=0、端口、nginx 双次 200、**/admin 前端产物 hash 已变**、DB 周期增长、主题入口与「本周期流量」不变、日志）；
5. `ops-docs/INDEPENDENCE.md` 增 §6.7；提交推送；打标签 `server/v0.0.24-own.4`。

## 验收标准
- admin 前端产物 hash 前后对比有变化；trivy own.4 Go 依赖 0；部署后各项核对通过；
- 不改业务代码逻辑；失败可用 own.3 脚本回滚。
