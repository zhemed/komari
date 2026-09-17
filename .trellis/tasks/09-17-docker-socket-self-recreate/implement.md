# 执行计划：容器一键升级（docker.sock + 重建自身容器）

> 顺序原则：先把"外部能验证的底层"做完（Engine API 客户端、重建子命令），
> 再接进升级流程与界面，最后才发版。每一步都给出验证命令。

## Step 1 · `internal/dockerapi`（Engine API over unix socket）

- [ ] 客户端：`http.Client` + unix dialer；`Ping`/`ServerVersion`（取 API 版本，取 min(本地常量)）
- [ ] `PullImage`（流式进度回调）、`InspectContainer`（完整结构体）、
      `RenameContainer`/`CreateContainer`/`StartContainer`/`StopContainer`/`RemoveContainer`、
      `WaitContainer`/`ContainerLogs`
- [ ] 单测：用 `httptest` + unix socket 模拟 daemon，覆盖成功、409 名冲突、403 权限不足、
      拉取流式进度解析、超时
- 验证：`go test ./internal/dockerapi/... -v`

## Step 2 · 子命令 `komari docker-self-recreate`（helper 逻辑）

- [ ] `cmd/docker_self_recreate.go`：读旧容器 → 预检（`docker run --rm <新镜像> … --help`）→
      改名 → 建新 → 起新 → 写状态；每步失败回滚（改名回原名 + 启动）
- [ ] 幂等：镜像相同直接退出；同名容器已存在先 inspect 判断
- [ ] 单测：把"重建参数组装"抽成纯函数（`buildRecreateSpec(oldInspect, newImage, newName) (createPayload, error)`），
      用固定 JSON 快照断言：卷/网络/端口/restart/env/entrypoint 全部保留、镜像被替换
- 验证：`go test ./internal/dockerapi/... ./cmd/... -v`；`go run . docker-self-recreate --help`

## Step 3 · 接进升级流程

- [ ] `internal/upgrade`：`Plan.Mode`（`binary` / `docker-recreate` / `manual` / `download-only`）；
      容器 + socket 可用 → `docker-recreate`
- [ ] Docker 分支 `Execute`：拉镜像（进度）→ 起 helper（挂 socket + 数据目录，AutoRemove）→ 写状态 `restarting`
- [ ] 设置键 `server_upgrade_docker_socket`（默认 `/var/run/docker.sock`，可被通用设置接口改）
- [ ] 审计日志：from/to、镜像 digest、旧容器名
- 验证：`go test ./internal/upgrade/... -v`；本机用真实 socket 跑 helper 子命令（对着测试容器）

## Step 4 · 界面

- [ ] Docker 且 socket 可用：按钮文案改为"立即升级（重建容器）"，并显示权限提示
- [ ] 状态阶段：pulling / prechecking / recreating / restarting（沿用现有轮询）
- [ ] 失败展示：helper 日志尾部 + 旧容器名与手工回滚命令
- [ ] 顺带把上次记的观感问题修掉：未登录/状态未知时不渲染按钮（`upgradeStatus && …`）
- [ ] 5 语言补键；前端产物重建并更新 `FRONTEND_TREE_SHA256`
- 验证：`cd frontend && npx tsc --noEmit`；`./scripts/build-frontend.sh`

## Step 5 · 文档与规范

- [ ] `docs/MAINTAINING.md` §14.4.2 改写：容器现在能一键升级（挂 socket 时），
      给出 `docker run` 示例、安全告警、回滚命令；**更正**"容器不能自升级是架构限制"的说法
      （那是取舍；现在提供重建容器方案）
- [ ] `.trellis/spec/backend/server-upgrade.md` 同步（新增 docker 分支与不变量）
- [ ] `README.md`：容器一键升级一句 + socket 权限提示
- 验证：`./scripts/check-repo.sh`（文档路径与锚点）

## Step 6 · 发版与端到端实测

- [ ] 版本线 → 0.0.11；先提交后构建；18 资产 + 两个镜像
- [ ] **E2E（真实容器 + 真实 docker.sock）**：
  - 起一个挂 socket 的容器（新版本镜像），点/调"立即升级"，目标选 0.0.10（降级同样走重建路径）
  - `docker inspect` 逐项对比重建前后：名称、卷、端口/网络、restart 策略、env
  - 数据完整（累计流量、rollups、节点）；镜像 tag 已变为目标
  - 再升回 0.0.11
- [ ] **失败注入**：不存在的 tag（拉取失败）、预检不通过 → 现有容器未变、服务照常
- [ ] **回归**：不挂 socket 的容器仍是"复制 pull 命令"；本机 systemd 形态行为不变
- 验证：以上每步留日志/截图；`check-repo.sh --full` 全绿

## 风险文件与回滚点

| 文件 | 风险 | 回滚 |
|---|---|---|
| `internal/dockerapi/*` | 讲错 API 版本/字段 → 重建失败 | 单测用快照锁定 payload 字段 |
| `cmd/docker_self_recreate.go` | 重建丢配置 → 用户服务起不来 | 每步回滚 + E2E 逐项 inspect 对比 |
| 前端 | 文案/条件改动 | 回滚提交 + 重建产物 |
| 本机部署 | E2E 会真的重建容器（不影响 systemd 生产实例：生产不是容器） | 旧容器保留为停止状态，可 `docker start` 回滚 |
