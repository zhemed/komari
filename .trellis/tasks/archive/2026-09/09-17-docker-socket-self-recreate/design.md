# 设计：容器一键升级（docker.sock + 重建自身容器）

## 1. 为什么必须有 helper 容器

服务端**不能删除/重建自己正在其中运行的容器**：
- 容器名被自己占着，新建同名容器会 409；
- 端口（`-p` 映射时）也要等旧容器停了才释放，而旧容器停了就没人执行后续步骤了；
- 自己 `stop` 自己之后，没有任何进程去 `start` 新容器。

所以最后一步交给一个**独立的 helper 容器**：它不随本容器退出而消失，能安全地
"改名旧容器 → 建新容器 → 起新容器"。

helper 用**目标镜像**启动（它必然包含本版本新增的子命令），只挂 docker.sock 与本容器的数据目录，
跑完即 `AutoRemove`。

## 2. 新增组件

### 2.1 `internal/dockerapi`（标准库，unix socket，不引 SDK）

| 方法 | 用途 |
|---|---|
| `Ping` / `ServerVersion` | 探测 socket 可用与 API 版本 |
| `PullImage(ctx, ref, progress)` | `POST /images/create`，流式进度（JSON lines）回调给状态机 |
| `InspectContainer(id)` | 读自身容器完整配置（`Config` + `HostConfig` + `NetworkSettings` + `Mounts`） |
| `RunHelperContainer(spec)` | 建 + 起 helper（不 `--rm` 由 daemon 负责，用 `AutoRemove`） |
| `WaitContainer(id)` / `ContainerLogs(id)` | 等 helper 结束、取日志（失败原因回传面板） |
| `RenameContainer(id, name)` / `CreateContainer(payload)` / `StartContainer(id)` / `StopContainer(id)` | helper 内部用 |

实现要点：`http.Client` + `Transport.DialContext` 拨 unix socket；请求路径带 API 版本
（`/v1.43/...`，取自 `ServerVersion` 与本地常量的较小值）。

### 2.2 子命令 `komari docker-self-recreate`（helper 模式）

```
komari docker-self-recreate \
  --container <本容器 id 或名字> \
  --image ghcr.io/zhemed/komari:0.0.11 \
  --state-file <数据目录>/upgrade-state.json \
  --sanity-tag 0.0.11
```

执行顺序（**每步失败都回滚**）：

1. `InspectContainer` 读旧容器；校验 `Config.Image` 与目标不同（幂等：相同则直接写状态退出）。
2. 预检：`docker run --rm <新镜像> <Entrypoint…> --help`，输出须含 `Komari Monitor <tag>`。
3. `RenameContainer(old, old-<时间戳>)`。
4. 组装新容器 payload：`Config`（镜像换成目标、其余字段沿用）、`HostConfig`（沿用，剔除只读/创建时不接受的字段）、
   `NetworkingConfig`（按旧容器 `NetworkSettings.Networks` 还原别名与静态 IP）。
5. `CreateContainer`（用**原名**）→ 失败：把旧容器改名回去 + 启动，写状态 `failed`。
6. `StartContainer` → 失败：删除半成品容器 + 恢复旧容器。
7. 写状态 `completed`，退出；旧容器保留为停止状态（回滚点，不自动删除）。

### 2.3 `internal/upgrade` 的 Docker 分支

`Prepare` 增加判定顺序（在"容器 → manual"之前）：

```
容器 && socket 可访问 → Mode = docker-recreate
容器 && 无 socket     → Mode = manual（现状：给 pull 命令）
非容器               → 现状（二进制替换 / 仅下载）
```

Docker 分支的 `Execute`：
1. GitHub 侧选目标 tag（沿用现有逻辑；`komari-SHA256SUMS` 不再适用镜像形态——镜像完整性由
   registry digest 保证，状态里记录 digest 便于审计）；
2. `PullImage`（进度 → `PhaseDownloading`）；
3. 启动 helper（挂 socket + 数据目录），把 helper 容器 id 写给状态；
4. 本进程写状态 `restarting` 后**什么都不做**（不自杀）：helper 会 force-remove 旧容器，
   本进程随之终止——这比"自己 stop 自己"更可控（helper 能拿到退出码与日志）。
5. 新容器起来后，`upgradeStatus` 由新进程读取状态文件 → `completed`（已有 `Reconcile` 机制）。

### 2.4 安全边界（写进文档与界面）

- 只有 socket 存在时才启用；socket 存在本身就是用户显式选择（`-v /var/run/docker.sock:...`）。
- 仍受 `server_upgrade_enabled` 开关约束；每次升级写审计日志（含 from/to/digest/旧容器名）。
- 界面在 Docker 模式且可用时给出提示："此模式通过 Docker socket 重建容器，等于授予容器宿主权限"。
- 不把 socket 传给任何其他东西；helper 只做重建这一件事，跑完即销毁。
- 没有 socket → 回落到"复制 pull 命令"，不报错。

## 3. 关键取舍

| 取舍 | 选择 | 理由 |
|---|---|---|
| helper 用什么镜像 | **目标镜像** | 不引入 `docker:cli` 依赖；新版本必然带该子命令 |
| 旧容器处理 | 改名保留（停止） | 给用户留回滚点；不自动删（避免误删用户数据/配置） |
| 镜像校验 | 记录 registry digest（拉取结果） | 镜像形态下 SHA256 文件不适用；digest 由 registry 保证 |
| 失败回滚 | helper 内完成 | 只有 helper 在容器外，能在新旧容器都不健康时兜底 |

## 4. 风险

| 风险 | 缓解 |
|---|---|
| 重建后配置漂移（丢卷/网络/restart 策略） | 直接沿用 inspect 的 `Config`/`HostConfig`/网络，不手写默认值；E2E 里逐项 `docker inspect` 对比 |
| helper 半途失败导致"三个容器" | 每步回滚 + 幂等（同名容器已存在则先 inspect 判断）；状态文件记录 `old_container` 便于手工清理 |
| 用户没挂数据目录 → 状态写不进去 | 检测不到挂载就只在 helper 日志里留结果，面板靠 `/api/version` 判定成功 |
| socket 权限不足（非 root 运行） | 预检 `Ping` 失败即回落 manual 模式并给出说明 |
