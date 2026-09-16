#!/usr/bin/env bash
#
# 构建 komari 单二进制（本仓库自维护的 0.0.x 版本线）。
#
# 与上游 release 流程的 ldflags 口径一致，但不依赖 CI、也不依赖网络/Node——
# 默认主题产物已 vendor 在仓库内（web/public/defaultTheme/）。
#
# 用法：
#   ./scripts/build-komari.sh                                      # 本机构建（动态链接 glibc）
#   KOMARI_STATIC=1 ./scripts/build-komari.sh                      # 发布用：linux/amd64 静态（需 zig）
#   KOMARI_STATIC=1 KOMARI_GOARCH=arm64 ./scripts/build-komari.sh  # 发布用：linux/arm64 静态
#   KOMARI_VERSION=0.0.2 ./scripts/build-komari.sh                 # 下一版本：递增 patch 位
#   KOMARI_OUTPUT=/tmp/komari ./scripts/build-komari.sh            # 自定义输出路径
#
# 为什么发布必须静态：Dockerfile 基于 alpine:3.21（musl），glibc 动态二进制在其中无法运行；
# glibc 静态虽然能链接成功，但 getaddrinfo/NSS 依赖宿主共享库，不作为发布形态。
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${REPO_ROOT}"

log() { printf '[build-komari] %s\n' "$*" >&2; }
die() { printf '[build-komari] ERROR: %s\n' "$*" >&2; exit 1; }

# 版本号：本仓库自有的 0.0.x 版本线（自 2026-09 起由上游 komari 1.4.3 派生，见 docs/MAINTAINING.md）。
# 默认值集中在 scripts/version.env（发版只改那一处），KOMARI_VERSION 仍可覆盖。
# shellcheck source=scripts/version.env
. "${SCRIPT_DIR}/version.env"
VERSION="${KOMARI_VERSION}"
if [ -z "${KOMARI_VERSION_HASH:-}" ]; then
  KOMARI_VERSION_HASH="$(git rev-parse HEAD 2>/dev/null || echo unknown)"
fi
OUTPUT="${KOMARI_OUTPUT:-${REPO_ROOT}/bin/komari}"
GOARCH_TARGET="${KOMARI_GOARCH:-amd64}"
STATIC="${KOMARI_STATIC:-0}"

THEME_INDEX="web/public/defaultTheme/dist/index.html"
if [ ! -f "${THEME_INDEX}" ]; then
  die "缺少默认主题产物：${THEME_INDEX}
     这正是 web/public/public.go:130 会 panic 的原因（//go:embed defaultTheme，见 public.go:18）。
     修复：运行 ./scripts/sync-frontend.sh 重新生成（需要网络与 Node），
     或确认已 vendor 的 web/public/defaultTheme/ 未被删除。"
fi

command -v go >/dev/null 2>&1 || die "未找到 go 工具链"

LDFLAGS="-s -w -X github.com/komari-monitor/komari/utils.CurrentVersion=${VERSION} -X github.com/komari-monitor/komari/utils.VersionHash=${KOMARI_VERSION_HASH}"

if [ "${STATIC}" = "1" ] || [ "${STATIC}" = "true" ]; then
  # 定位 zig：优先 KOMARI_ZIG，其次 PATH，最后本仓库 .build/tools/ 下的本地安装
  ZIG_BIN="${KOMARI_ZIG:-}"
  if [ -z "${ZIG_BIN}" ] && command -v zig >/dev/null 2>&1; then
    ZIG_BIN="$(command -v zig)"
  fi
  if [ -z "${ZIG_BIN}" ]; then
    ZIG_BIN="$(find "${REPO_ROOT}/.build/tools" -maxdepth 2 -type f -name zig 2>/dev/null | head -1)"
  fi
  [ -n "${ZIG_BIN}" ] && [ -x "${ZIG_BIN}" ] || die "KOMARI_STATIC=1 需要 zig，但未找到。
     安装：从 https://ziglang.org/download/ 下载 linux-x86_64 包并解压，或设 KOMARI_ZIG=/path/to/zig。
     不静默退化为动态链接——alpine 镜像会因此无法运行。"
  case "${GOARCH_TARGET}" in
    amd64) ZIG_TARGET="x86_64-linux-musl" ;;
    arm64) ZIG_TARGET="aarch64-linux-musl" ;;
    *) die "静态构建暂不支持 GOARCH=${GOARCH_TARGET}（仅 amd64 / arm64）" ;;
  esac
  log "静态构建：GOARCH=${GOARCH_TARGET}，CC=\"${ZIG_BIN} cc -target ${ZIG_TARGET}\""
  set -- env CGO_ENABLED=1 "GOARCH=${GOARCH_TARGET}" "CC=${ZIG_BIN} cc -target ${ZIG_TARGET}"
  LDFLAGS="${LDFLAGS} -linkmode external -extldflags=-static"
else
  log "本机构建：GOARCH=${GOARCH_TARGET}，CGO_ENABLED=${CGO_ENABLED:-1}（动态链接）"
  set -- env CGO_ENABLED="${CGO_ENABLED:-1}" "GOARCH=${GOARCH_TARGET}"
fi

log "go build（Komari ${VERSION}，hash ${KOMARI_VERSION_HASH}）"
mkdir -p "$(dirname "${OUTPUT}")"
"$@" go build -trimpath -ldflags="${LDFLAGS}" -o "${OUTPUT}" .

log "产物: ${OUTPUT}（$(stat -c%s "${OUTPUT}") 字节）"
if command -v file >/dev/null 2>&1; then
  file -b "${OUTPUT}" | cut -c1-90 >&2
fi
