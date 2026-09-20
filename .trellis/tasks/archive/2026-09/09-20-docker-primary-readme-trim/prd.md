# Docker 镜像定为主方案 + README 删减 + 彻底剔除 compose

## 用户纠正（2026-09-20）

「docker run -d --name komari --restart always --network host -v ./data:/app/data
ghcr.io/zhemed/komari:latest 这个才是我们的主方案 Docker 镜像；而且 README 写了一大堆，需要大面积删减；
还有 compose 我都说了有严重 bug 为什么还不剔除」。

**我上一轮读反了**：用户说"我们还是用这样的部署命令吧"，指的是 README 里那条 **Docker 命令**
（升级仍走面板/脚本），我却把 systemd 脚本写成主方案——判断错误，已纠正。

## 判别性验证（先证明再写）

用用户给的原命令实测（临时数据目录 + 换端口避免撞生产 25774）：

- `docker run` 成功；`ghcr.io/zhemed/komari:latest` = `sha256:e731121a…`，Created `2026-09-18T14:48:11Z`
  （= 0.0.18 的构建时间）
- 容器 `Up`，面板首启进 `/install` 向导（HTTP 200，307 跳转来自 `/`）——新数据目录的正常行为
- 数据落宿主目录；容器一键升级（容器内替换）是既有能力，不需要挂 socket
- 验完即清理临时容器与数据目录（`docker ps -a` 只剩 litepan）

## 交付

1. **README 重写**：Docker 命令是第一条、标为**唯一主方案**；systemd 降为备选；**compose 零出现**；
   145 行 → 94 行（删掉冗余解释、重复的构建细节与上游沿革段落）。
2. **MAINTAINING §3.4.1 重写为「部署口径」**：Docker 主 + systemd 备 + compose 明确禁止
   （不再提"唯一路径是 install-komari.sh"），验收清单同时给容器与 systemd 两套命令。
3. **spec 更正**：`server-upgrade.md` §5 与 `backend/index.md` 改为"Docker 主方案 + systemd 备选；
   compose 已剔除"，并说明主方案用的是"容器内替换"、不受 host 网络遗留影响。
4. **§14.2 警示改口径**：不再以 compose 为主语；说明 host 网络 + 挂 socket 的重建容器模式已停用。
5. **自检第 14 项改判据**（`check-repo.sh`）：
   - 主方案口径：README 必须含那条 `docker run … --restart always` 与 `ghcr.io/zhemed/komari:latest`
   - **compose 禁入**：`git grep` 扫 README/install 脚本/`scripts/*.sh`/`internal/`/`cmd/`，
     命中 `docker compose|docker-compose|compose.ya?ml` 即报红（排除检查脚本自身）
   - 仍保留：部署自动化不得分叉（`deploy-entry:` 只允许三个）+ 判别性验证（造 `install-compose.sh`）
   - 判别性验证：往 README 里塞一行 `docker compose up -d` → 报红并打印行号；还原 → 通过

## Acceptance Criteria

- [x] README 主方案 = 用户给的 Docker 命令；compose 在 README 零命中
- [x] README 大幅删减（145 → 94 行）
- [x] MAINTAINING/spec 口径与 README 一致；compose 只作为"禁止"与"事故记录"存在
- [x] 第 14 项含 compose 禁入 + 两条判别性验证，实测有效
- [x] `check-repo.sh --full` 14 项全绿
- [x] 生产未被本轮改动触碰（临时容器已清理）
