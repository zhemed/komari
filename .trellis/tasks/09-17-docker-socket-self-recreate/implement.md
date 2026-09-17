# 执行计划：容器一键升级（docker.sock + 重建自身容器）

> 顺序原则：先把"外部能验证的底层"做完（Engine API 客户端、重建子命令），
> 再接进升级流程与界面，最后才发版。每一步都给出验证命令。

## Step 1 · `internal/dockerapi`（Engine API over unix socket）

- [x] 客户端：`http.Client` + unix dialer；`Ping`/`ServerVersion`（取 API 版本，取 min(本地常量)）
- [x] `PullImage`（流式进度回调）、`InspectContainer`（完整结构体）、
      `RenameContainer`/`CreateContainer`/`StartContainer`/`StopContainer`/`RemoveContainer`、
      `WaitContainer`/`ContainerLogs`
- [x] 单测：用 `httptest` + unix socket 模拟 daemon，覆盖成功、409 名冲突、403 权限不足、
      拉取流式进度解析、超时
- 验证：`go test ./internal/dockerapi/... -v`

## Step 2 · 子命令 `komari docker-self-recreate`（helper 逻辑）

- [x] `cmd/docker_self_recreate.go`：读旧容器 → 预检（`docker run --rm <新镜像> … --help`）→
      改名 → 建新 → 起新 → 写状态；每步失败回滚（改名回原名 + 启动）
- [x] 幂等：镜像相同直接退出；同名容器已存在先 inspect 判断
- [x] 单测：把"重建参数组装"抽成纯函数（`buildRecreateSpec(oldInspect, newImage, newName) (createPayload, error)`），
      用固定 JSON 快照断言：卷/网络/端口/restart/env/entrypoint 全部保留、镜像被替换
- 验证：`go test ./internal/dockerapi/... ./cmd/... -v`；`go run . docker-self-recreate --help`

## Step 3 · 接进升级流程

- [x] `internal/upgrade`：`Plan.Mode`（`binary` / `docker-recreate` / `manual` / `download-only`）；
      容器 + socket 可用 → `docker-recreate`
- [x] Docker 分支 `Execute`：拉镜像（进度）→ 起 helper（挂 socket + 数据目录，AutoRemove）→ 写状态 `restarting`
- [x] 设置键 `server_upgrade_docker_socket`（默认 `/var/run/docker.sock`，可被通用设置接口改）
- [x] 审计日志：from/to、镜像 digest、旧容器名
- 验证：`go test ./internal/upgrade/... -v`；本机用真实 socket 跑 helper 子命令（对着测试容器）

## Step 4 · 界面

- [x] Docker 且 socket 可用：按钮文案改为"立即升级（重建容器）"，并显示权限提示
- [x] 状态阶段：pulling / prechecking / recreating / restarting（沿用现有轮询）
- [x] 失败展示：helper 日志尾部 + 旧容器名与手工回滚命令
- [x] 顺带把上次记的观感问题修掉：未登录/状态未知时不渲染按钮（`upgradeStatus && …`）
- [x] 5 语言补键；前端产物重建并更新 `FRONTEND_TREE_SHA256`
- 验证：`cd frontend && npx tsc --noEmit`；`./scripts/build-frontend.sh`

## Step 5 · 文档与规范

- [x] `docs/MAINTAINING.md` §14.4.2 改写：容器现在能一键升级（挂 socket 时），
      给出 `docker run` 示例、安全告警、回滚命令；**更正**"容器不能自升级是架构限制"的说法
      （那是取舍；现在提供重建容器方案）
- [x] `.trellis/spec/backend/server-upgrade.md` 同步（新增 docker 分支与不变量）
- [x] `README.md`：容器一键升级一句 + socket 权限提示
- 验证：`./scripts/check-repo.sh`（文档路径与锚点）

## Step 6 · 发版与端到端实测

- [x] 版本线 → 0.0.11；先提交后构建；18 资产 + 两个镜像
- [x] **E2E（真实容器 + 真实 docker.sock）**：
  - 起一个挂 socket 的容器（新版本镜像），点/调"立即升级"，目标选 0.0.10（降级同样走重建路径）
  - `docker inspect` 逐项对比重建前后：名称、卷、端口/网络、restart 策略、env
  - 数据完整（累计流量、rollups、节点）；镜像 tag 已变为目标
  - 再升回 0.0.11
- [x] **失败注入**：不存在的 tag（拉取失败）、预检不通过 → 现有容器未变、服务照常
- [x] **回归**：不挂 socket 的容器仍是"复制 pull 命令"；本机 systemd 形态行为不变
- 验证：以上每步留日志/截图；`check-repo.sh --full` 全绿

## 风险文件与回滚点

| 文件 | 风险 | 回滚 |
|---|---|---|
| `internal/dockerapi/*` | 讲错 API 版本/字段 → 重建失败 | 单测用快照锁定 payload 字段 |
| `cmd/docker_self_recreate.go` | 重建丢配置 → 用户服务起不来 | 每步回滚 + E2E 逐项 inspect 对比 |
| 前端 | 文案/条件改动 | 回滚提交 + 重建产物 |
| 本机部署 | E2E 会真的重建容器（不影响 systemd 生产实例：生产不是容器） | 旧容器保留为停止状态，可 `docker start` 回滚 |

---

## 端到端实测记录（2026-09-17，真实容器 + 真实 docker socket）

| 场景 | 结果 |
|---|---|
| 模式探测 | ✅ 容器 + socket → `mode=docker-recreate`、`supported=true`；未挂 socket → `mode=manual`、`supported=false`（返回可复制命令，无回归） |
| 容器重建升级 | ✅ 0.0.12 容器内触发升级到 0.0.11：**约 4 秒**完成；新容器跑 0.0.11，helper 容器（`komari-upgrade-helper-<ts>`，Exited 0）与旧容器（`komari-self-old-<ts>`，Exited）都保留 |
| 配置不漂移 | ✅ 重建前后逐项对比一致：容器名、卷（数据卷 + socket）、端口映射、restart 策略、网络模式、env、WorkingDir、Cmd；Hostname 已正确重新分配为新容器短 ID（没有沿用旧 ID） |
| 数据完整 | ✅ 节点数 1、累计流量 4093.9/1456.3 MB、metric_rollups 11102 行全在 |
| 审计 | ✅ `logs` 表：`server upgrade requested: 0.0.12 -> 0.0.11` |
| 升回 | ✅ 再从 0.0.11 升回 0.0.12（约 4 秒）——说明"目标镜像无 helper 子命令"的场景已被 0.0.12 的修复覆盖 |
| 失败注入（不存在的 tag） | ✅ 返回 `release 9.9.9 not found`，容器 ID 未变，服务 200 |
| 发布 | ✅ 0.0.12：18 资产 + 两个镜像；本机生产（systemd）升到 0.0.12，二进制与 release 资产一致 |

## E2E 抓到的缺陷：0.0.11 的 helper 用错镜像（已修于 0.0.12）

现象：0.0.11 容器里点升级（目标 0.0.10）→ 没有任何变化，状态卡在 `restarting`。
证据（`docker events`）：`create practical_boyd → start → die → destroy`（1 秒内完成）——
helper 用**目标镜像**启动，而 `docker-self-recreate` 子命令是 0.0.11 才引入的，目标镜像里没有；
又因为当时 helper 是 `AutoRemove=true`，现场被一并抹掉，只剩"点了没反应"。

0.0.12 的修法：① helper 改用**当前镜像**；② helper 不自动删除、命名并打标签（下次升级前清理），
失败可 `docker logs` 查；③ 父进程监视 helper，提前退出即取日志尾部写状态回报面板。
回归测试：`helperPayload` 签名与 `AutoRemove=false` + 标签断言（单测）。
公开的 0.0.11 发布说明已追加更正段（原文保留）。

## 未覆盖（如实标注）

- **真实容器里的"自动回滚"注入**：单测覆盖了 create 失败与 start 失败两条回滚路径
  （`TestRecreateRollsBackWhenCreateFails` / `WhenStartFails`）；真实容器里要制造"新容器起不来"
  需要占用宿主端口或容器名，而那同时也会挡住回滚自身的重启，测试不具结论性，故未做。
- **swarm/k8s**、**自动删除旧容器**：按 PRD 明确不在范围内。
