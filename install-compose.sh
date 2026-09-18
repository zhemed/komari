#!/bin/bash
#
# Komari 服务器 · docker compose 一条命令部署
#
#   curl -fsSL https://raw.githubusercontent.com/zhemed/komari/refs/heads/main/install-compose.sh | sudo bash
#
# 它做的事：建项目目录（默认 /opt/docker/komari，750）→ 写 docker-compose.yml
# （钉版本 / host 网络 / 日志上限 / healthcheck / 可选挂 docker.sock）→ docker compose up -d
# → 等健康检查通过 → 打印访问地址与后续操作。
#
# 参数（也可用环境变量）：
#   --dir <路径>     项目目录（默认 /opt/docker/komari；env KOMARI_COMPOSE_DIR）
#   --name <名字>    容器名（默认 komari；env KOMARI_COMPOSE_NAME）
#   --tag <版本>     镜像 tag（默认见下方 DEFAULT_TAG；env KOMARI_TAG）
#   --listen <地址>  监听地址，host 网络用（默认 0.0.0.0:25774；env KOMARI_LISTEN）
#   --port <端口>    改用端口映射（去掉 host 网络，映射 <端口>:25774）
#   --tz <时区>      容器时区（默认取宿主机 /etc/timezone，取不到用 Asia/Shanghai）
#   --no-socket      不挂 /var/run/docker.sock（放弃面板内一键升级，最小权限部署）
#   --force          项目目录已有 docker-compose.yml 时覆盖（默认拒绝，保护现有部署）
#   --no-start       只写文件，不启动
#
# 幂等：默认不会覆盖已存在的 compose 文件；加 --force 才会重写并重建容器。
# 数据目录（./data）永远不会被本脚本删除或覆盖。
#
set -euo pipefail

DEFAULT_TAG="0.0.18" # 发版时与 scripts/version.env 同步（check-repo 第 1 项会校验）

PROJECT_DIR="${KOMARI_COMPOSE_DIR:-/opt/docker/komari}"
NAME="${KOMARI_COMPOSE_NAME:-komari}"
TAG="${KOMARI_TAG:-$DEFAULT_TAG}"
LISTEN="${KOMARI_LISTEN:-0.0.0.0:25774}"
PORT=""
TZ_VALUE="${KOMARI_TZ:-$(cat /etc/timezone 2>/dev/null || echo Asia/Shanghai)}"
MOUNT_SOCKET=1
FORCE=0
START=1

log()  { printf '%s\n' "$*"; }
ok()   { printf '\033[32m%s\033[0m\n' "$*"; }
warn() { printf '\033[33m[警告] %s\033[0m\n' "$*"; }
die()  { printf '\033[31m[错误] %s\033[0m\n' "$*" >&2; exit 1; }

while [ $# -gt 0 ]; do
  case "$1" in
    --dir) PROJECT_DIR="${2:?--dir 需要路径}"; shift 2 ;;
    --name) NAME="${2:?--name 需要容器名}"; shift 2 ;;
    --tag) TAG="${2:?--tag 需要版本}"; shift 2 ;;
    --listen) LISTEN="${2:?--listen 需要地址}"; shift 2 ;;
    --port) PORT="${2:?--port 需要端口}"; shift 2 ;;
    --tz) TZ_VALUE="${2:?--tz 需要时区}"; shift 2 ;;
    --no-socket) MOUNT_SOCKET=0; shift ;;
    --force) FORCE=1; shift ;;
    --no-start) START=0; shift ;;
    -h|--help) sed -n '2,30p' "$0"; exit 0 ;;
    *) die "未知参数：$1（用 --help 看用法）" ;;
  esac
done

[ -n "$TAG" ] || die "镜像 tag 不能为空"
[ -n "$TZ_VALUE" ] || TZ_VALUE="Asia/Shanghai"

command -v docker >/dev/null 2>&1 || die "未找到 docker：请先安装 Docker Engine（https://docs.docker.com/engine/install/）"
docker compose version >/dev/null 2>&1 || die "未找到 docker compose（v2）：请安装 docker-compose-plugin"

COMPOSE_FILE="$PROJECT_DIR/docker-compose.yml"
if [ -f "$COMPOSE_FILE" ] && [ "$FORCE" != 1 ]; then
  die "$COMPOSE_FILE 已存在（可能是现有部署）。要覆盖请加 --force；只想启停请直接在该目录跑 docker compose up -d / down"
fi

# 容器名冲突预检：同名容器若不属于本项目目录，直接拒绝并给出解法，
# 而不是等 docker compose 起一半再报 Conflict（2026-09-18 实测踩到）。
if docker inspect "$NAME" >/dev/null 2>&1; then
  existing_dir="$(docker inspect --format '{{index .Config.Labels "com.docker.compose.project.working_dir"}}' "$NAME" 2>/dev/null || true)"
  if [ "$existing_dir" != "$PROJECT_DIR" ]; then
    die "已有同名容器 \"$NAME\"（项目目录：${existing_dir:-未知/非 compose}）。换名字加 --name，或换目录加 --dir；确认要替换请先自行处理该容器"
  fi
fi

log "== Komari compose 部署 =="
log "  项目目录: $PROJECT_DIR"
log "  容器名:   $NAME"
log "  镜像:     ghcr.io/zhemed/komari:$TAG"
if [ -n "$PORT" ]; then
  log "  网络:     bridge + 端口映射 $PORT:25774"
else
  log "  网络:     host（监听 $LISTEN）"
fi
log "  docker.sock: $([ "$MOUNT_SOCKET" = 1 ] && echo '挂载（面板内一键升级，= 宿主 root 等价权限）' || echo '不挂（放弃面板内一键升级）')"

mkdir -p "$PROJECT_DIR" "$PROJECT_DIR/data"
chmod 750 "$PROJECT_DIR" 2>/dev/null || true

{
  echo "# 由 install-compose.sh 生成（$(date -u +%Y-%m-%dT%H:%M:%SZ)）"
  echo "# 取值依据 / 升级交互 / 重启策略的取舍见 docs/MAINTAINING.md §15"
  echo "services:"
  echo "  komari:"
  echo "    image: ghcr.io/zhemed/komari:$TAG"
  echo "    container_name: $NAME"
  echo "    restart: unless-stopped        # 尊重手工 docker stop；崩溃/升级兜底会被拉起（§15.6）"
  if [ -n "$PORT" ]; then
    echo "    ports:"
    echo "      - \"$PORT:25774\""
  else
    echo "    network_mode: host             # host 网络：不要写 ports"
  fi
  echo "    environment:"
  echo "      TZ: $TZ_VALUE"
  if [ -z "$PORT" ]; then
    echo "      KOMARI_LISTEN: $LISTEN"
  fi
  echo "    volumes:"
  echo "      - ./data:/app/data           # 唯一有状态的东西（备份它就够了）"
  if [ "$MOUNT_SOCKET" = 1 ]; then
    echo "      - /var/run/docker.sock:/var/run/docker.sock   # 面板升级走重建容器（= 宿主 root 等价权限）"
  fi
  echo "    logging:                       # docker 默认 json-file 无上限"
  echo "      driver: json-file"
  echo "      options:"
  echo "        max-size: \"10m\""
  echo "        max-file: \"3\""
  echo "    healthcheck:"
  echo "      test: [\"CMD\", \"curl\", \"-fsSL\", \"-o\", \"/dev/null\", \"http://127.0.0.1:25774/\"]"
  echo "      interval: 30s"
  echo "      timeout: 10s"
  echo "      retries: 3"
  echo "      start_period: 30s"
} > "$COMPOSE_FILE"
chmod 640 "$COMPOSE_FILE" 2>/dev/null || true
ok "已写入 $COMPOSE_FILE"

if [ "$START" != 1 ]; then
  log "（--no-start）下一步：cd $PROJECT_DIR && docker compose up -d"
  exit 0
fi

cd "$PROJECT_DIR"
docker compose up -d

log "等待健康检查通过..."
for _ in $(seq 1 60); do
  status="$(docker inspect --format '{{.State.Health.Status}}' "$NAME" 2>/dev/null || echo starting)"
  [ "$status" = "healthy" ] && break
  sleep 2
done
if [ "$status" = "healthy" ]; then
  ok "容器健康（healthy）"
else
  warn "健康检查尚未通过（status=$status）——看日志：docker logs $NAME"
fi

if [ -n "$PORT" ]; then
  URL="http://localhost:$PORT"
else
  URL="http://localhost:25774"
fi

cat <<EOF

------------------------------------------------------------------------
部署完成
  面板地址:  $URL   （首次访问会进安装向导）
  数据目录:  $PROJECT_DIR/data
  常用命令:  cd $PROJECT_DIR && docker compose logs -f / restart / down

升级方式:
  · 挂了 docker.sock：面板里点"立即升级"（拉镜像 + 重建容器，版本与镜像始终一致；
    0.0.18 起会自动把本文件里的 image tag 同步成新版本）
  · 没挂 docker.sock：改本文件的 image tag 后 docker compose pull && docker compose up -d
  · 手工回滚：docker start ${NAME}-old-<时间戳>（升级时保留的旧容器）
------------------------------------------------------------------------
EOF
