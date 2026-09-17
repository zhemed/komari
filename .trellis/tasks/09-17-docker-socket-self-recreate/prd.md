# 容器部署的一键升级（挂 docker.sock，面板拉镜像并重建自身容器）

## Goal

Docker 部署的用户也要能在**网页里点一下就升级**，而不是拿到一条命令去手工执行。
做法（用户 2026-09-17 选定的方案 B）：容器里挂载 `/var/run/docker.sock` 时，服务端通过
Docker Engine API 拉取目标镜像，并用一个 helper 容器把**本容器**按原配置 + 新镜像重建。

## Background（事实与边界）

- 容器里**替换二进制是能生效的**（退出后 `--restart` 会拉起新二进制），但**重建容器会退回镜像版本**；
  0.0.8~0.0.10 出于这个原因只给"复制 pull 命令"。本任务改用"重建容器"消除这个不一致。
- 现有能力（可直接复用）：GitHub release 选择与 `komari-SHA256SUMS` 校验、状态机与
  `upgrade-state.json`、`admin:upgradeServer/upgradeStatus`、`server_upgrade_enabled` 开关、审计日志；
  容器/无 systemd 判定与 `Plan.Manual` 分支（`internal/upgrade/upgrade.go`）。
- **安全边界（必须写明并保持）**：docker.sock 等价于宿主 root。因此
  ① 只在 socket 存在时启用该模式；② 仍受 `server_upgrade_enabled` 约束；③ 写审计日志；
  ④ 文档与界面提示该权限含义；⑤ 不把 socket 暴露给 helper 之外的任何东西。
- 没有 socket（未挂载）时行为不变：仍走"复制 pull 命令"。

## Requirements

- R1 识别 Docker 模式：`/.dockerenv` 存在 **且** socket 路径可访问（默认 `/var/run/docker.sock`，
  可用设置 `server_upgrade_docker_socket` 覆盖）。
- R2 拉取目标镜像：`POST /images/create?fromImage=<repo>&tag=<tag>`，进度写入升级状态（面板可见）。
- R3 预检：用目标镜像跑一次 `<entrypoint> --help`（`docker run --rm`），输出必须包含
  `Komari Monitor <tag>`；不通过则中止，**不动现有容器**。
- R4 重建自身容器（由 helper 完成，见 design）：
  - 复用自身容器的 `Config`（Env/Cmd/Entrypoint/Labels/ExposedPorts/WorkingDir/User…）与
    `HostConfig`（Binds/Mounts/NetworkMode/RestartPolicy/PortBindings/LogConfig/…），仅替换镜像；
  - 保留容器名（先把旧容器改名为 `<name>-old-<时间戳>`）；
  - 自定义网络与别名按 `NetworkSettings.Networks` 还原；
  - **任何一步失败都要把旧容器改名回原名并启动**（回滚）。
- R5 成功后旧容器保留为停止状态（便于手工回滚），面板展示其名字；不自动删除。
- R6 状态与提示：面板可见阶段（pulling/precheck/recreating/restarting），失败原因能回传；
  容器模式界面文案从"复制命令"改为"立即升级（重建容器）"，并提示 socket 权限含义。
- R7 未挂 socket 时保持现状（manual + pull 命令），不得回归。
- R8 文档与规范：MAINTAINING §14 增"容器一键升级"（安全边界、回滚命令）、
  `spec/backend/server-upgrade.md` 同步；README 一句。
- R9 不改 agent；不引入 docker CLI/SDK 依赖（标准库直接讲 Engine API over unix socket）。

## Acceptance Criteria

- [ ] 真实容器（挂 docker.sock）里点面板"立即升级"：拉新镜像 → 重建自身容器 → 面板版本号变成目标版本；
      容器配置（名称、卷、网络/端口、restart 策略、env）与新镜像 tag 符合预期
- [ ] 升级后数据完整（累计流量、metric rollups、节点），`docker inspect` 显示镜像已是新 tag
- [ ] 失败注入：镜像不存在 / 预检不通过 → 现有容器**未被替换**，服务照常运行
- [ ] 重建失败 → 自动回滚（旧容器改回原名并启动），面板显示失败原因
- [ ] 未挂 socket 的容器：仍是"复制 pull 命令"，无回归
- [ ] 非容器（systemd + 二进制）形态行为完全不变
- [ ] `check-repo.sh --full` 全绿；单测覆盖 Engine API 客户端与重建参数组装
- [ ] 发布新版本（资产 18 个 + 镜像），本机容器实测留证

## Out of Scope

- 不实现"容器内自替换二进制"（方案 A）；
- 不做自动定时升级；不自动删除旧容器；不做 swarm/k8s 编排；
- 不动 agent 自更新。

## Notes

- 本任务是 `09-17-panel-one-click-upgrade`（已归档）的延伸：那版把"容器不替换二进制"写成了边界，
  用户明确要求改成"通过 Docker API 重建容器"；**"容器不能自升级"是取舍不是架构限制**，
  这条更正也要写进文档。
