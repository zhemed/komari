#!/usr/bin/env bash
#
# 构建 komari-agent（我们自己的 0.0.x 版本线）。
#
# 与服务器构建最大的不同：agent 是纯 Go（CGO_ENABLED=0），交叉编译只需要 Go，
# **不需要 zig**，一次可以出全部 14 个平台（上游 build_all.sh 同口径）。
#
# 用法：
#   ./scripts/build-agent.sh                        # 全部 14 个平台 → dist/agent/
#   ./scripts/build-agent.sh --only linux/amd64     # 只构建一个平台（开发时快跑）
#   KOMARI_VERSION=0.0.5 ./scripts/build-agent.sh   # 下一版本（见 scripts/version.env）
#   KOMARI_AGENT_OUTPUT=/tmp/agent ./scripts/build-agent.sh
#
# 产物命名与上游一致（komari-agent-<os>-<arch>[.exe]），因为
# install-agent.sh、前端生成的安装命令、以及 agent 自更新的资产匹配都按这个名字找资产。
#
# 为什么二进制里必须带两项 -X 注入：
#   update.CurrentVersion → 节点详情里显示的版本号
#   update.Repo           → 自更新查询的仓库（必须是我们，不能是上游）
# 另外补丁 0001 已把 Repo 的源码默认值也改成我们的仓库，注入只是双保险。
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${REPO_ROOT}"

log() { printf '[build-agent] %s\n' "$*" >&2; }
die() { printf '[build-agent] ERROR: %s\n' "$*" >&2; exit 1; }

# shellcheck source=scripts/version.env
. "${SCRIPT_DIR}/version.env"
# shellcheck source=scripts/agent-pin.env
. "${SCRIPT_DIR}/agent-pin.env"

PATCH_DIR="${SCRIPT_DIR}/patches-agent"
WORK_DIR="${KOMARI_AGENT_WORK_DIR:-${REPO_ROOT}/.build/agent-src}"
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
      sed -n '2,20p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    *) die "未知参数：$1（只支持 --only <os>/<arch>）" ;;
  esac
done

command -v go >/dev/null 2>&1 || die "未找到 go 工具链"
command -v git >/dev/null 2>&1 || die "未找到 git"
[ -n "${KOMARI_AGENT_COMMIT:-}" ] || die "scripts/agent-pin.env 缺少 KOMARI_AGENT_COMMIT"

# 目录树规范化哈希：排序 + 归零 mtime/owner + gzip -n，同一份内容跨机器同哈希。
# （与 scripts/sync-frontend.sh 的 tree_hash 同口径。）
tree_hash() {
  ( cd "$1" && tar --sort=name --mtime='@0' --owner=0 --group=0 --numeric-owner \
      --exclude=.git -cf - . | gzip -n -9 | sha256sum | awk '{print $1}' )
}

GO_ACTUAL="$(go env GOVERSION)"
if [ -n "${KOMARI_AGENT_GO_VERSION:-}" ] && [ "${GO_ACTUAL}" != "${KOMARI_AGENT_GO_VERSION}" ]; then
  log "注意：本机 Go 为 ${GO_ACTUAL}，发布记录为 ${KOMARI_AGENT_GO_VERSION}；版本不同不影响构建，但产物哈希会不同"
fi

# ---------- 1. 取到 pin 的 commit ----------
if [ ! -d "${WORK_DIR}/.git" ]; then
  log "克隆 ${KOMARI_AGENT_REPO} → ${WORK_DIR}"
  rm -rf "${WORK_DIR}"
  mkdir -p "$(dirname "${WORK_DIR}")"
  git clone --filter=blob:none --no-checkout "${KOMARI_AGENT_REPO}" "${WORK_DIR}"
fi
cd "${WORK_DIR}"
log "检出 ${KOMARI_AGENT_COMMIT}"
if ! git fetch --depth 1 origin "${KOMARI_AGENT_COMMIT}" >/dev/null 2>&1; then
  log "按 SHA 拉取失败，回退到 origin/main 后再试"
  git fetch --depth 50 origin main >/dev/null 2>&1 \
    || die "无法从 ${KOMARI_AGENT_REPO} 拉取 ${KOMARI_AGENT_COMMIT}"
fi
git checkout --force "${KOMARI_AGENT_COMMIT}" >/dev/null 2>&1 || die "无法检出 ${KOMARI_AGENT_COMMIT}"
git clean -xfdq
[ "$(git rev-parse HEAD)" = "${KOMARI_AGENT_COMMIT}" ] || die "HEAD 不是 pin 的 commit"

# ---------- 2. 应用自有补丁 ----------
shopt -s nullglob
patches=("${PATCH_DIR}"/[0-9]*.patch)
shopt -u nullglob
[ ${#patches[@]} -gt 0 ] || die "未找到任何补丁：${PATCH_DIR}/[0-9]*.patch"
for p in "${patches[@]}"; do
  log "应用补丁 $(basename "$p")"
  git apply --check "$p" || die "补丁无法应用（上游文件结构已变？）：$p"
  git apply "$p"
done

# ---------- 3. 仓库里的安装脚本成品必须与“pin 源码 + 补丁”的结果一致 ----------
# 我们对外发的是 install-agent.sh / install-agent.ps1 两个成品文件（前端安装命令直接指向它们），
# 所以这里回放补丁做比对，防止上游文件或补丁变了而仓库成品没重新生成。
PINNED_VERSION="$(sed -n 's/^KOMARI_VERSION="\${KOMARI_VERSION:-\(.*\)}"$/\1/p' "${SCRIPT_DIR}/version.env")"
if [ "${KOMARI_AGENT_SKIP_INSTALLER_CHECK:-0}" = "1" ]; then
  log "注意：KOMARI_AGENT_SKIP_INSTALLER_CHECK=1，跳过安装脚本一致性校验"
else
  for pair in "install-agent.sh:install.sh" "install-agent.ps1:install.ps1"; do
    vendored="${pair%%:*}"
    upstream="${pair##*:}"
    if ! diff -q "${upstream}" "${REPO_ROOT}/${vendored}" >/dev/null 2>&1; then
      die "${vendored} 与“pin 源码 + 补丁回放”的结果不一致。
     说明上游安装脚本或补丁变了，但仓库里的成品没重新生成。修复：
       cp ${WORK_DIR}/${upstream} ${REPO_ROOT}/${vendored}
     确认 diff 符合预期后再提交。"
    fi
    log "${vendored} 与补丁回放结果一致"
  done

  if [ "${KOMARI_VERSION}" = "${PINNED_VERSION}" ]; then
    grep -q "^default_agent_version=\"${KOMARI_VERSION}\"$" "${REPO_ROOT}/install-agent.sh" \
      || die "install-agent.sh 里 default_agent_version 不是 ${KOMARI_VERSION}（发版时忘了同步？）"
    grep -q "^\$DefaultAgentVersion = \"${KOMARI_VERSION}\"$" "${REPO_ROOT}/install-agent.ps1" \
      || die "install-agent.ps1 里 \$DefaultAgentVersion 不是 ${KOMARI_VERSION}（发版时忘了同步？）"
    log "安装脚本 pin 的版本与构建版本一致：${KOMARI_VERSION}"
  else
    log "注意：KOMARI_VERSION=${KOMARI_VERSION} 与仓库默认版本 ${PINNED_VERSION} 不同，跳过安装脚本版本一致性校验"
  fi
fi

# ---------- 4. 源码树哈希校验 ----------
SRC_HASH="$(tree_hash "${WORK_DIR}")"
log "打补丁后源码树哈希: ${SRC_HASH}"
if [ -n "${AGENT_SOURCE_TREE_SHA256:-}" ]; then
  if [ "${AGENT_SOURCE_TREE_SHA256}" != "${SRC_HASH}" ]; then
    die "源码树哈希不一致：期望 ${AGENT_SOURCE_TREE_SHA256}，实际 ${SRC_HASH}。
     说明 pin 的 commit 变了，或补丁内容/顺序变了。确认接受后把
     scripts/agent-pin.env 的 AGENT_SOURCE_TREE_SHA256 更新为上面的实际值，并在提交信息里说明原因。"
  fi
  log "源码树哈希校验通过（与 scripts/agent-pin.env 一致）"
else
  log "scripts/agent-pin.env 的 AGENT_SOURCE_TREE_SHA256 为空，请回填：AGENT_SOURCE_TREE_SHA256=\"${SRC_HASH}\""
fi

# ---------- 5. 构建矩阵（与上游 build_all.sh 一致：14 个平台） ----------
OS_LIST=(windows linux darwin freebsd)
ARCH_LIST=(amd64 arm64 386 arm loong64)
LDFLAGS="-X github.com/komari-monitor/komari-agent/update.CurrentVersion=${KOMARI_VERSION} -X github.com/komari-monitor/komari-agent/update.Repo=${KOMARI_AGENT_UPDATE_REPO}"

rm -rf "${OUTPUT_DIR}"
mkdir -p "${OUTPUT_DIR}"
: > "${OUTPUT_DIR}/SHA256SUMS"

log "go build（Komari agent ${KOMARI_VERSION}，自更新目标 ${KOMARI_AGENT_UPDATE_REPO}，Go ${GO_ACTUAL}）"
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
      go build -trimpath -ldflags="${LDFLAGS}" -o "${OUTPUT_DIR}/${BINARY_NAME}" .
    ( cd "${OUTPUT_DIR}" && sha256sum "${BINARY_NAME}" >> SHA256SUMS )
  done
done

COUNT="$(find "${OUTPUT_DIR}" -type f -name 'komari-agent-*' | wc -l)"
log "完成：${COUNT} 个产物在 ${OUTPUT_DIR}（$(du -sh "${OUTPUT_DIR}" | cut -f1)），校验和见 SHA256SUMS"
log "下一步：./scripts/build-agent-image.sh（构建并推送镜像）或直接上传到 release"
