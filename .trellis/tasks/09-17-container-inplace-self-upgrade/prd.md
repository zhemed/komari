# 容器零配置网页升级（容器内替换二进制 + 原地重执行）

## Goal

用户的需求只有一条：**在网页里点一下就把容器升级完成**。
不要求挂 `docker.sock`、不要求换部署形态、不要求重启策略配置。

做法：容器 + 没有可用 socket 时，走**容器内二进制替换**——
下载对应平台的 release 资产 → 校验 `komari-SHA256SUMS` → 自检（`--help` 版本行）→
备份旧二进制 → 原子替换；然后用 `syscall.Exec` **原地重执行**（同一容器、同一 PID，
不依赖 restart 策略）；若 re-exec 不可用则回落为退出进程交给 Docker 重启。
界面按模式给出文案与"重建容器会回退"的提示。

## Background（事实）

- 0.0.8~0.0.12 里，容器形态被短路成 `manual`（只给命令）或 `docker-recreate`（要求挂 socket）
  —— `internal/upgrade/upgrade.go` 的 `Prepare` 里容器分支直接 return，从不尝试二进制替换。
- 容器里替换 `/app/komari` **是有效的**：二进制在容器可写层，`docker restart` 后仍是新版本；
  只有**重建容器**（`docker rm` + `docker run` / `compose up`）才会退回镜像里的版本。
- 二进制替换链路已存在且经过验证（下载/校验/自检/备份/原子替换），容器里缺的只是"重启方式"。
- `syscall.Exec` 在容器里把当前进程镜像换成新二进制：PID 不变、容器不重启、无需 restart 策略。

## Requirements

- R1 容器 + 无可用 socket → 新模式 `container-replace`：下载 `komari-linux-<arch>`，
  校验 `komari-SHA256SUMS`，自检版本行，备份后原子替换。
- R2 替换成功后**优先 `syscall.Exec` 原地重执行**；失败则回落 `exit(42)`（交给 Docker 重启策略），
  两条路都要写状态并记审计。
- R3 二进制目录不可写（只读 rootfs 等）→ 仍回落 manual（给出可复制命令），不得报错卡死。
- R4 界面：该模式下按钮为"立即升级（容器内替换）"，并提示
  "重建容器会退回镜像版本；想与镜像完全一致请挂 docker.sock"。
- R5 不挂 socket 也能升级；挂了 socket 仍优先用重建容器（与镜像一致）。
- R6 文档：README 与 MAINTAINING §14.4.2/§14.6 写清两条容器升级路径与各自代价。
- R7 不回归：systemd 二进制形态、manual、download-only、docker-recreate 行为不变。

## Acceptance Criteria

- [ ] 真实容器（**不挂 socket**、`--restart always`、数据卷）里，用面板同款接口从当前版本升到旧版本再升回：
      版本号变化、**容器 ID 不变**（re-exec 生效）、服务不中断超过数秒
- [ ] 数据完整（累计流量、rollups、节点）
- [ ] 替换后容器内 `/app/komari` 与目标 release 资产**逐字节一致**；生成 `/app/komari.backup.<旧版本>`
- [ ] 只读目录注入 → 回落 manual 且给出可复制命令
- [ ] 挂 socket 的容器仍走 rebuild（不回归）；systemd 生产实例行为不变
- [ ] `check-repo.sh --full` 全绿；单测覆盖新模式的 Prepare/Execute 与 re-exec 回落
