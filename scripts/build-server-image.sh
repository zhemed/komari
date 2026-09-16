#!/usr/bin/env bash
#
# 构建并（可选）推送 komari **服务器**容器镜像到 ghcr.io/zhemed/komari。
#
# 用法：
#   ./scripts/build-server-image.sh                 # 本地单平台（linux/amd64）构建，不推送
#   ./scripts/build-server-image.sh --push          # amd64 + arm64 构建并推送 :<版本> 与 :latest
#   KOMARI_VERSION=0.0.5 ./scripts/build-server-image.sh --push
#
# 前提：先产出静态服务器二进制（Dockerfile 注释里写了原因，alpine 需要静态链接）：
#   KOMARI_STATIC=1 KOMARI_OUTPUT=dist/komari-linux-amd64 ./scripts/build-komari.sh
#   KOMARI_STATIC=1 KOMARI_GOARCH=arm64 KOMARI_OUTPUT=dist/komari-linux-arm64 ./scripts/build-komari.sh
#
# 与 scripts/build-agent-image.sh 是"同一个套路的两份实现"（上下文准备 + buildx 调用），
# 刻意不抽公共函数：两者的平台/产物名/异常说明都不同，抽出来反而更难读。
#
# 认证：用 gh 的 token（需 write:packages 权限）
#   gh auth token | docker login ghcr.io -u <你的用户名> --password-stdin
#
# 注：本机若没有 QEMU/binfmt 模拟器，arm64 会因为 Dockerfile 里的 `RUN apk add` 报
# `exec format error`；装一次模拟器即可：
#   docker run --privileged --rm tonistiigi/binfmt --install arm64
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${REPO_ROOT}"

log() { printf '[build-server-image] %s\n' "$*" >&2; }
die() { printf '[build-server-image] ERROR: %s\n' "$*" >&2; exit 1; }

# shellcheck source=scripts/version.env
. "${SCRIPT_DIR}/version.env"

IMAGE="${KOMARI_SERVER_IMAGE:-ghcr.io/zhemed/komari}"
SERVER_DIR="${KOMARI_SERVER_OUTPUT:-${REPO_ROOT}/dist}"
PUSH=0
PLATFORMS="linux/amd64,linux/arm64"

while [ $# -gt 0 ]; do
  case "$1" in
    --push) PUSH=1; shift ;;
    --platforms) PLATFORMS="$2"; shift 2 ;;
    -h|--help) sed -n '2,26p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) die "未知参数：$1（支持 --push / --platforms <列表>）" ;;
  esac
done

command -v docker >/dev/null 2>&1 || die "未找到 docker"
docker buildx version >/dev/null 2>&1 || die "未找到 docker buildx"

# 上下文：Dockerfile 只 COPY komari-${TARGETOS}-${TARGETARCH}，所以把产物和 Dockerfile 放一起
CTX="${REPO_ROOT}/.build/server-image"
rm -rf "${CTX}"
mkdir -p "${CTX}"
cp -f "${REPO_ROOT}/Dockerfile" "${CTX}/Dockerfile"

need_asset() {
  case "$1" in
    linux/amd64) echo "komari-linux-amd64" ;;
    linux/arm64) echo "komari-linux-arm64" ;;
    *) die "服务器镜像暂不支持的平台：$1（只发 amd64/arm64，因为只有这两个静态产物）" ;;
  esac
}

# 本地构建只校验/使用 amd64；推送时才要求全部平台产物齐全
if [ "${PUSH}" = "1" ]; then
  IFS=',' read -r -a plat_list <<< "${PLATFORMS}"
else
  plat_list=("linux/amd64")
fi
for p in "${plat_list[@]}"; do
  asset="$(need_asset "${p}")"
  [ -f "${SERVER_DIR}/${asset}" ] || die "缺少产物 ${SERVER_DIR}/${asset}
     先跑：KOMARI_STATIC=1 KOMARI_OUTPUT=${SERVER_DIR}/${asset} ./scripts/build-komari.sh
     （arm64 加 KOMARI_GOARCH=arm64；静态构建需要 zig，见 docs/MAINTAINING.md §3.3）"
  file "${SERVER_DIR}/${asset}" | grep -q "statically linked" \
    || die "${SERVER_DIR}/${asset} 不是静态链接。alpine 镜像里跑不起来，
     用 KOMARI_STATIC=1 重新构建（这也是 Dockerfile 注释里的前提）。"
  cp -f "${SERVER_DIR}/${asset}" "${CTX}/${asset}"
done

if [ "${PUSH}" = "1" ]; then
  log "构建并推送 ${IMAGE}:${KOMARI_VERSION} 与 :latest（平台 ${PLATFORMS}）"
  docker buildx build \
    --platform "${PLATFORMS}" \
    --tag "${IMAGE}:${KOMARI_VERSION}" \
    --tag "${IMAGE}:latest" \
    --provenance=false \
    --push "${CTX}"
  log "已推送：${IMAGE}:${KOMARI_VERSION} / :latest"
else
  log "本地构建（仅 linux/amd64，不推送）"
  docker buildx build \
    --platform linux/amd64 \
    --tag "${IMAGE}:${KOMARI_VERSION}" \
    --provenance=false \
    --load "${CTX}"
  log "已构建：${IMAGE}:${KOMARI_VERSION}"
  log "验证：docker run --rm -p 25774:25774 ${IMAGE}:${KOMARI_VERSION}"
fi
