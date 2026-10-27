#!/usr/bin/env bash
# 版本化启动脚本：ghcr.io/zhemed/komari:0.0.25（own.6 发布组合）
# 约定：只把监听地址收紧到 loopback（供 nginx 反代），不写死任何站点信息。
set -euo pipefail
IMAGE="${KOMARI_IMAGE:-ghcr.io/zhemed/komari:0.0.25}"
LISTEN="${KOMARI_LISTEN:-127.0.0.1:25774}"
DATA_DIR="${KOMARI_DATA_DIR:-/root/komari/data}"
NAME="${KOMARI_CONTAINER:-komari}"

docker rm -f "$NAME" >/dev/null 2>&1 || true
docker run -d --name "$NAME" --restart always --network host \
  -v "${DATA_DIR}:/app/data" \
  -e GIN_MODE=release \
  -e "KOMARI_LISTEN=${LISTEN}" \
  --workdir /app \
  "$IMAGE"
docker ps --filter "name=${NAME}" --format '{{.Names}} ｜ {{.Status}} ｜ {{.Image}}'
