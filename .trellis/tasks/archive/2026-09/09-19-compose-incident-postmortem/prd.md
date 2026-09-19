# compose 事故复盘：把最严重的失误写死成规范（防复发）

## Goal

用户 2026-09-19 定调：「compose 是我们维护以来最严重的 bug 和失误，明白了吗」。

不只是点头认账——把这件事**固化成可执行规范与机械闸门**，让后来的人（包括未来的我）
没法在不知情的情况下重建同一套机制。

## 取证（写之前先核对，不靠记忆）

时间线（均有提交哈希）：
- `644169c` 09-18 14:01 定稿 compose 部署（B 策略：挂 socket + helper 重建容器）入仓
- `6ed7fbd` 09-18 14:02 生产切到 `/opt/docker/komari`
- `dd0486a` 09-18 14:33 新增"读容器 label 找 compose 文件 + 升级后改 image tag"；同一提交里修 host 网络自识别
- `4479580` 09-18 14:46 发布 0.0.18
- `f263484` 09-18 15:04 文件名改 compose.yaml
- `4c2c204` 09-19 05:04 文件名改回
- `fb66759` 09-19 05:13 整弧回滚

关键机制（观测）：容器 label `com.docker.compose.project.config_files` 在创建时写入、不随文件改名更新；
`up -d` 不重建 → 旧值被继续用；检测读不到就**静默跳过**；host 网络下两条自识别线索全失效 → 静默退回容器内替换。

## Acceptance Criteria

- [x] 事故案例写成规范文件：`.trellis/spec/guides/incident-compose-autosync.md`
      （严重性举证、时间线、机制、五层根因、五条硬规则、可复用教训）
- [x] 规范接入三处入口：guides/index.md、backend/index.md 的 Pre-Development Checklist、
      MAINTAINING §14.2 支持矩阵顶部警示
- [x] 补上一直缺失的事实：`spec/backend/server-upgrade.md` §5「host 网络下容器模式会静默失效」
      （回滚后这条遗留重新存在，此前**没有任何规范记录它**）
- [x] 机械闸门：`scripts/check-repo.sh` 第 13 项——硬失败（`com.docker.compose`、`selfid.go`、
      `compose.go`）+ 提示（存量 `DetectSelfContainerID`）+ **判别性验证**（对历史提交 `dd0486a` 必须报红）
- [x] `./scripts/check-repo.sh --full` 全绿；CI 双任务绿

## 边界

- 不改已发布的历史记录（0.0.18 release 说明只在本地文件加归档注记，不动线上）
- 不碰生产、不碰线上 release/镜像
- 不为"凑闸门"删存量功能：`DetectSelfContainerID` 仍是 0.0.17 的默认检测路径，第 13 项只提示不判失败
