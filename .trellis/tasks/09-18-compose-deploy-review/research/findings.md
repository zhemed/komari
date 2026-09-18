# Docker Compose 部署方案审查（2026-09-18）

对用户草稿逐条**用真实镜像验证**（不是纸面评审）。测试用 `ghcr.io/zhemed/komari:0.0.17`、
host 网络 + 备用端口 25790 起的临时 compose 项目，跑完已 `compose down` + 删除临时目录。

## 结论：草稿方向正确，5 处要改

### ✅ 已实测确认正确的部分

| 草稿项 | 验证方式与结果 |
|---|---|
| `network_mode: host` + 不写 `ports` | 起容器成功；容器内 `127.0.0.1:25790` 直接可达（与宿主同 netns） |
| `KOMARI_LISTEN` | 代码里真实存在：`cmd/server.go:25` `GetEnv("KOMARI_LISTEN", "0.0.0.0:25774")` |
| `TZ: Asia/Shanghai` | 镜像装了 tzdata（`/usr/share/zoneinfo/Asia/Shanghai` 存在）；实测容器日志时间 21:40（CST）而探针 UTC 时间 13:40 |
| `volumes: ./data:/app/data` | 应用确实用相对路径 `./data`（`database/dbcore/dbcore.go:204`），Dockerfile `WORKDIR /app` → `/app/data` 正确 |
| `healthcheck` 机制 | 容器状态实测到达 **healthy**（FailingStreak=0，ExitCode=0） |
| busybox wget 存在 | `/usr/bin/wget`（BusyBox v1.37.0）——探针不会因缺工具而永远 unhealthy |

### ⚠️ 必须改的 5 处

1. **版本钉错了：`0.0.7` → `0.0.17`。**
   实测 `docker run --entrypoint /app/komari ghcr.io/zhemed/komari:0.0.7 docker-self-recreate`
   → `unknown command "docker-self-recreate" for "Komari"`（0.0.11 才引入该子命令），
   而 0.0.17 同命令返回 `--container 与 --image 必填`（子命令存在）。
   即：钉 0.0.7 = 面板一键升级完全不可用（且 0.0.8 才引入 `komari-SHA256SUMS`，0.0.13 才引入零配置容器内替换）。

2. **healthcheck 换成 curl，别用 `wget -qO-`。**
   `-qO-` 会把响应体打进健康日志：实测单次探针 **3020 字节 HTML**（每次探针都存一份）。
   镜像里 curl 是显式安装的（`command -v curl` → `/usr/bin/curl`），
   实测 `curl -fsSL -o /dev/null http://127.0.0.1:25790/` 退出码 0（`/` 是 307 → 跟随到 `/install` → 200）。

3. **加日志上限。** 实测默认 `json-file` 且 `Config=map[]`（无 max-size）——面板日志无限增长。

4. **`GIN_MODE: release` 是冗余的**：Dockerfile 已有 `ENV GIN_MODE=release`。留着无害，删掉更干净。

5. **升级路径必须二选一（关键取舍，务必写进注释）：**
   - **(A) 以 compose 为准**：面板里点"容器内替换"升级后，**任何 `docker compose up -d`（重建容器）
     都会把版本退回镜像 tag**。要升级就改 compose 里的 tag → `docker compose pull && docker compose up -d`。
   - **(B) 挂 `/var/run/docker.sock`**：面板改为"拉镜像 + 重建容器"，版本与镜像永远一致，
     代价是该容器获得**宿主 root 等价权限**。
   两者不能混用默认期待，否则会出现"点完升级、下次 compose up 又变回去"的困惑。

### 📋 定稿（可直接用，含注释）

```yaml
services:
  komari:
    image: ghcr.io/zhemed/komari:0.0.17   # ① 钉版本；升级=改这里 → pull + up -d
    container_name: komari
    restart: unless-stopped                # ②
    network_mode: host                     # ③ host 网络：不要写 ports
    environment:
      TZ: Asia/Shanghai                    # 镜像已装 tzdata，实测生效
      KOMARI_LISTEN: 0.0.0.0:25774         # 代码默认值即 0.0.0.0:25774（cmd/server.go:25）
      # GIN_MODE 已在镜像里设成 release，无需重复
    volumes:
      - ./data:/app/data                   # ④ 应用用 ./data（WORKDIR=/app）
    logging:                               # ⑤ 默认 json-file 无上限，实测确认
      driver: json-file
      options:
        max-size: "10m"
        max-file: "3"
    healthcheck:                           # ⑥ 用 curl：镜像自带，且不把 HTML 打进健康日志
      test: ["CMD", "curl", "-fsSL", "-o", "/dev/null", "http://127.0.0.1:25774/"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 30s
```

## 目录约定（`/opt/docker/<project>/`，伞目录 750）

- 布局建议：`/opt/docker/komari/{docker-compose.yml,data/}`；`data/` 由 docker 以 root 创建
  （容器实测 `uid=0`），与伞目录 750 root:root 不冲突。
- `./data` 里除 `komari.db`、`metrics.db` 外还有 `data/backup/upgrade-*.zip`（升级自动备份）；
  建议把整个 `data/` 纳入定期备份，它是唯一有状态的东西。
- 伞目录 750 只影响**宿主上非 root 用户**；compose 由 root 跑不受影响。

## 未验证 / 边界（写实）

- 没在他们的 `/opt/docker/` 真机上部署（越出工作区，未获授权）；
- "重建容器会退回镜像版本"这条来自 0.0.13/0.0.14 轮次的 E2E（记录在 MAINTAINING §14.2/§14.4.2），
  本次没有重复演示；
- agent 侧的 compose 未涉及（本次只审服务器）。
