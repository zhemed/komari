#!/usr/bin/env bash
#
# 构建 komari 1.4.3 单二进制。
#
# 与上游 .github/workflows/release.yml:124 的口径一致，只是不依赖 CI、
# 也不依赖网络/Node——默认主题产物已 vendor 在仓库内（web/public/defaultTheme/）。
#
# 用法：
#   ./scripts/build-komari.sh
#   KOMARI_VERSION=1.4.4 ./scripts/build-komari.sh        # 发自有补丁版时递增 patch 位
#   KOMARI_OUTPUT=/tmp/komari ./scripts/build-komari.sh   # 自定义输出路径
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${REPO_ROOT}"

log() { printf '[build-komari] %s\n' "$*" >&2; }
die() { printf '[build-komari] ERROR: %s\n' "$*" >&2; exit 1; }

# 版本号规则：维持 1.4.3；将来发自有补丁版必须递增三段中的 patch 位（如 1.4.4），
# 否则前端 parseSemver（只取 x.y.z 三段）永远不会把它判为“可更新”。
# 版本号：本 fork 自有的 0.0.1 基线（自 2026-09 起从上游 komari 1.4.3 派生，见 docs/MAINTAINING.md）。
# 递增规则：前端 parseSemver 只取 x.y.z 三段并要求严格递增，故发新版本必须递增 patch 位。
VERSION="${KOMARI_VERSION:-0.0.1}"
if [ -z "${KOMARI_VERSION_HASH:-}" ]; then
  KOMARI_VERSION_HASH="$(git rev-parse HEAD 2>/dev/null || echo unknown)"
fi
OUTPUT="${KOMARI_OUTPUT:-${REPO_ROOT}/bin/komari}"

THEME_INDEX="web/public/defaultTheme/dist/index.html"
if [ ! -f "${THEME_INDEX}" ]; then
  die "缺少默认主题产物：${THEME_INDEX}
     这正是 web/public/public.go:130 会 panic 的原因（//go:embed defaultTheme，见 public.go:18）。
     修复：运行 ./scripts/sync-frontend.sh 重新生成（需要网络与 Node），
     或确认已 vendor 的 web/public/defaultTheme/ 未被删除。"
fi

command -v go >/dev/null 2>&1 || die "未找到 go 工具链"

log "go build（CGO_ENABLED=${CGO_ENABLED:-1}，Komari ${VERSION}，hash ${KOMARI_VERSION_HASH}）"
mkdir -p "$(dirname "${OUTPUT}")"
CGO_ENABLED="${CGO_ENABLED:-1}" go build -trimpath \
  -ldflags="-s -w -X github.com/komari-monitor/komari/utils.CurrentVersion=${VERSION} -X github.com/komari-monitor/komari/utils.VersionHash=${KOMARI_VERSION_HASH}" \
  -o "${OUTPUT}" .

log "产物: ${OUTPUT}（$(stat -c%s "${OUTPUT}") 字节）"
