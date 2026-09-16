#!/usr/bin/env bash
#
# 构建 komari-agent（我们自己的 0.0.x 版本线）。
#
# 源码就在本仓库的 agent/ 目录里（上游 komari-agent 的快照 + 我们内联的改动，见
# docs/MAINTAINING.md §11），**不再克隆上游、不再打补丁**。纯 Go（CGO_ENABLED=0），
# 交叉编译只需要 Go，不需要 zig/gcc，一次可出 14 个平台。
#
# 用法：
#   ./scripts/build-agent.sh                        # 全部 14 个平台 → dist/agent/
#   ./scripts/build-agent.sh --only linux/amd64     # 单平台（开发时快跑）
#   KOMARI_VERSION=0.0.6 ./scripts/build-agent.sh   # 下一版本（见 scripts/version.env）
#   KOMARI_AGENT_OUTPUT=/tmp/agent ./scripts/build-agent.sh
#
# 产物命名与上游一致（komari-agent-<os>-<arch>[.exe]）：安装脚本、前端生成的安装命令、
# agent 自更新的资产匹配三者都按这个名字找资产。
#
# 为什么用 -buildvcs=false：agent 的身份是构建时注入的版本号（我们的 0.0.x 线），
# 不该把本仓库的提交信息编进二进制——否则同一份 agent 源码在不同提交上构建出的产物不同。
#
# 构建期自检（改坏了会被拦下，不是装饰）：
#   1. agent/update/update.go 里的 `Filters: []string{"^komari-agent-"}`：
#      没有它，同 release 里的服务器二进制 komari-linux-amd64 会被当成 agent 的更新包
#      （详见 docs/MAINTAINING.md §11.1，有实测对照实验）。
#   2. agent/update/update.go 里的 `Repo string = "zhemed/komari"`：自更新只能指向我们。
#   3. install-agent.sh / install-agent.ps1 里 pin 的默认版本必须等于本次 KOMARI_VERSION
#      （发版时忘了同步会被拦下）。
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${REPO_ROOT}"

log() { printf '[build-agent] %s\n' "$*" >&2; }
die() { printf '[build-agent] ERROR: %s\n' "$*" >&2; exit 1; }

# shellcheck source=scripts/version.env
. "${SCRIPT_DIR}/version.env"
# shellcheck source=scripts/agent-build.env
. "${SCRIPT_DIR}/agent-build.env"

AGENT_SRC="${REPO_ROOT}/agent"
WORK_DIR="${KOMARI_AGENT_WORK_DIR:-${AGENT_SRC}}"
OUTPUT_DIR="${KOMARI_AGENT_OUTPUT:-${REPO_ROOT}/dist/agent}"
ONLY=""

while [ $# -gt 0 ]; do
  case "$1" in
    --only)
      [ -n "${2:-}" ] || die "--only 需要参数，形如 linux/amd64"
      ONLY="$2"
      shift 2
      ;;
    -h|--help)
      sed -n '2,24p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    *) die "未知参数：$1（只支持 --only <os>/<arch>）" ;;
  esac
done

command -v go >/dev/null 2>&1 || die "未找到 go 工具链"
[ -f "${AGENT_SRC}/go.mod" ] || die "找不到 agent 源码：${AGENT_SRC}/go.mod 不存在。
     agent 源码应当随仓库一起存在（本仓库不再用 pin + 补丁的方式去拉上游）。"

GO_ACTUAL="$(go env GOVERSION)"
if [ -n "${KOMARI_AGENT_GO_VERSION:-}" ] && [ "${GO_ACTUAL}" != "${KOMARI_AGENT_GO_VERSION}" ]; then
  log "注意：本机 Go 为 ${GO_ACTUAL}，发布记录为 ${KOMARI_AGENT_GO_VERSION}；版本不同不影响构建，但产物哈希会不同"
fi

# ---------- 1. 关键防线自检 ----------
grep -q 'Filters: \[\]string{"\^komari-agent-"}' "${AGENT_SRC}/update/update.go" \
  || die "资产过滤不见了（agent/update/update.go 里没有 Filters: []string{\"^komari-agent-\"}）。
     没有它，agent 会把同 release 里的服务器二进制 komari-linux-amd64 当成自己的更新包。
     详见 docs/MAINTAINING.md §11.1。"
grep -q 'Repo string = "zhemed/komari"' "${AGENT_SRC}/update/update.go" \
  || die "agent/update/update.go 的自更新目标不是 zhemed/komari。"
log "关键防线自检通过（资产过滤 + 自更新目标）"

# 安装脚本 pin 的版本：只有构建仓库默认版本时才硬校验（临时构建别的版本只提示）
PINNED_VERSION="$(sed -n 's/^KOMARI_VERSION="\${KOMARI_VERSION:-\(.*\)}"$/\1/p' "${SCRIPT_DIR}/version.env")"
if [ "${KOMARI_VERSION}" = "${PINNED_VERSION}" ]; then
  grep -q "^default_agent_version=\"${KOMARI_VERSION}\"$" "${REPO_ROOT}/install-agent.sh" \
    || die "install-agent.sh 里 default_agent_version 不是 ${KOMARI_VERSION}（发版时忘了同步？）"
  grep -q "^\$DefaultAgentVersion = \"${KOMARI_VERSION}\"$" "${REPO_ROOT}/install-agent.ps1" \
    || die "install-agent.ps1 里 \$DefaultAgentVersion 不是 ${KOMARI_VERSION}（发版时忘了同步？）"
  log "安装脚本 pin 的版本与构建版本一致：${KOMARI_VERSION}"
else
  log "注意：KOMARI_VERSION=${KOMARI_VERSION} 与仓库默认版本 ${PINNED_VERSION} 不同，跳过安装脚本版本一致性校验"
fi

# ---------- 2. 构建矩阵（与上游 build_all.sh 一致：14 个平台） ----------
OS_LIST=(windows linux darwin freebsd)
ARCH_LIST=(amd64 arm64 386 arm loong64)
LDFLAGS="-X github.com/komari-monitor/komari-agent/update.CurrentVersion=${KOMARI_VERSION} -X github.com/komari-monitor/komari-agent/update.Repo=${KOMARI_AGENT_UPDATE_REPO}"

rm -rf "${OUTPUT_DIR}"
mkdir -p "${OUTPUT_DIR}"
: > "${OUTPUT_DIR}/SHA256SUMS"

log "go build（Komari agent ${KOMARI_VERSION}，自更新目标 ${KOMARI_AGENT_UPDATE_REPO}，Go ${GO_ACTUAL}，源码 ${WORK_DIR}）"
cd "${WORK_DIR}"
for GOOS in "${OS_LIST[@]}"; do
  for GOARCH in "${ARCH_LIST[@]}"; do
    # 与上游 build_all.sh 相同的排除项：windows/arm、darwin/{386,arm}、非 linux 的 loong64
    if { [ "${GOOS}" = "windows" ] && [ "${GOARCH}" = "arm" ]; } || \
       { [ "${GOOS}" = "darwin" ] && { [ "${GOARCH}" = "386" ] || [ "${GOARCH}" = "arm" ]; }; } || \
       { [ "${GOOS}" != "linux" ] && [ "${GOARCH}" = "loong64" ]; }; then
      continue
    fi
    [ -z "${ONLY}" ] || [ "${ONLY}" = "${GOOS}/${GOARCH}" ] || continue

    BINARY_NAME="komari-agent-${GOOS}-${GOARCH}"
    [ "${GOOS}" = "windows" ] && BINARY_NAME="${BINARY_NAME}.exe"

    log "构建 ${GOOS}/${GOARCH} → ${BINARY_NAME}"
    env CGO_ENABLED=0 "GOOS=${GOOS}" "GOARCH=${GOARCH}" \
      go build -trimpath -buildvcs=false -ldflags="${LDFLAGS}" -o "${OUTPUT_DIR}/${BINARY_NAME}" .
    ( cd "${OUTPUT_DIR}" && sha256sum "${BINARY_NAME}" >> SHA256SUMS )
  done
done

COUNT="$(find "${OUTPUT_DIR}" -type f -name 'komari-agent-*' | wc -l)"
log "完成：${COUNT} 个产物在 ${OUTPUT_DIR}（$(du -sh "${OUTPUT_DIR}" | cut -f1)），校验和见 SHA256SUMS"
log "下一步：./scripts/build-agent-image.sh（构建并推送镜像）或直接上传到 release"
