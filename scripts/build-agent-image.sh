#!/usr/bin/env bash
#
# 构建并（可选）推送 komari-agent 容器镜像到 ghcr.io/zhemed/komari-agent。
#
# 用法：
#   ./scripts/build-agent-image.sh                 # 本地单平台（linux/amd64）构建，不推送
#   ./scripts/build-agent-image.sh --push          # 三平台构建并推送 :<版本> 与 :latest
#   KOMARI_VERSION=0.0.5 ./scripts/build-agent-image.sh --push
#
# 前提：先跑 ./scripts/build-agent.sh 产出 dist/agent/（脚本会检查所需平台的产物是否齐全）。
#
# 认证：用 gh 的 token（需 write:packages 权限）
#   gh auth token | docker login ghcr.io -u <你的用户名> --password-stdin
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${REPO_ROOT}"

log() { printf '[build-agent-image] %s\n' "$*" >&2; }
die() { printf '[build-agent-image] ERROR: %s\n' "$*" >&2; exit 1; }

# shellcheck source=scripts/version.env
. "${SCRIPT_DIR}/version.env"

IMAGE="${KOMARI_AGENT_IMAGE:-ghcr.io/zhemed/komari-agent}"
AGENT_DIR="${KOMARI_AGENT_OUTPUT:-${REPO_ROOT}/dist/agent}"
PUSH=0
PLATFORMS="linux/amd64,linux/arm64,linux/arm/v7"

while [ $# -gt 0 ]; do
  case "$1" in
    --push) PUSH=1; shift ;;
    --platforms) PLATFORMS="$2"; shift 2 ;;
    -h|--help) sed -n '2,18p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) die "未知参数：$1（支持 --push / --platforms <列表>）" ;;
  esac
done

command -v docker >/dev/null 2>&1 || die "未找到 docker"
docker buildx version >/dev/null 2>&1 || die "未找到 docker buildx"

# 上下文：Dockerfile.agent 只 COPY 同名产物 + 标记文件，所以把三者放一起
# （没有任何 RUN 步骤 → 多架构构建不需要 QEMU/binfmt）
CTX="${REPO_ROOT}/.build/agent-image"
rm -rf "${CTX}"
mkdir -p "${CTX}"
cp -f "${REPO_ROOT}/Dockerfile.agent" "${CTX}/Dockerfile"
# 容器标记文件（Dockerfile.agent 刻意不写 RUN，改为 COPY 这个空文件）
: > "${CTX}/komari-agent-container"

# 需要的产物：按目标平台映射到 GOOS/GOARCH（linux/arm/v7 → linux-arm）
need_arch() {
  case "$1" in
    linux/amd64)   echo "komari-agent-linux-amd64" ;;
    linux/arm64)   echo "komari-agent-linux-arm64" ;;
    linux/arm/v7)  echo "komari-agent-linux-arm" ;;
    *) die "镜像暂不支持的平台：$1（产物矩阵里有，但镜像只发 amd64/arm64/armv7）" ;;
  esac
}

# 本地构建只校验/使用 amd64；推送时才要求全部平台产物齐全
if [ "${PUSH}" = "1" ]; then
  IFS=',' read -r -a plat_list <<< "${PLATFORMS}"
else
  plat_list=("linux/amd64")
fi
for p in "${plat_list[@]}"; do
  asset="$(need_arch "${p}")"
  [ -f "${AGENT_DIR}/${asset}" ] || die "缺少产物 ${AGENT_DIR}/${asset}
     先跑：./scripts/build-agent.sh（或 KOMARI_AGENT_OUTPUT=${AGENT_DIR} ./scripts/build-agent.sh）"
  cp -f "${AGENT_DIR}/${asset}" "${CTX}/${asset}"
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
  log "核对：docker manifest inspect ${IMAGE}:latest | grep -c architecture"
else
  log "本地构建（仅 linux/amd64，不推送）"
  docker buildx build \
    --platform linux/amd64 \
    --tag "${IMAGE}:${KOMARI_VERSION}" \
    --provenance=false \
    --load "${CTX}"
  log "已构建：${IMAGE}:${KOMARI_VERSION}（docker run ${IMAGE}:${KOMARI_VERSION} --help 验证）"
fi
