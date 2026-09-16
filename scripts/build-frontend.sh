#!/usr/bin/env bash
#
# 用本仓库内的前端源码（frontend/）构建默认主题，并注入 web/public/defaultTheme/。
#
# 前端源码**就在本仓库**（frontend/ = 上游 komari-web@4a74e8a8 的快照 + 我们内联的改动，
# 见 docs/MAINTAINING.md §3.2），所以本脚本不克隆上游、不打补丁。
#
# 为什么需要这个脚本：后端无法单独构建——web/public/public.go:18 是
# `//go:embed defaultTheme`，缺该目录时 static() 会 panic（public.go:130）。
# 产物已提交进仓库，因此**只改后端的人根本不需要跑这个脚本**；只有改前端时才需要。
#
# 用法：
#   ./scripts/build-frontend.sh                                     # 重新构建 + 校验哈希
#   VITE_KOMARI_UPDATE_REPO=owner/repo ./scripts/build-frontend.sh  # 覆盖“发现新版本”检查目标
#
# 需要 Node/npm 与网络（npm ci 要从 registry 装依赖；node_modules 不入库）。
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
# shellcheck source=scripts/frontend-build.env
source "${SCRIPT_DIR}/frontend-build.env"

SRC_DIR="${REPO_ROOT}/frontend"
DEST_DIR="${REPO_ROOT}/web/public/defaultTheme"

log() { printf '[build-frontend] %s\n' "$*" >&2; }
die() { printf '[build-frontend] ERROR: %s\n' "$*" >&2; exit 1; }

# 目录树规范化哈希：排序 + 归零 mtime/owner + gzip -n，保证同一份内容跨机器同哈希。
tree_hash() {
  ( cd "$1" && tar --sort=name --mtime='@0' --owner=0 --group=0 --numeric-owner \
      -cf - . | gzip -n -9 | sha256sum | awk '{print $1}' )
}

command -v npm >/dev/null 2>&1 || die "未找到 npm（改前端才需要 Node/npm；只构建后端请直接用 scripts/build-komari.sh，产物已在仓库内）"
command -v tar >/dev/null 2>&1 || die "未找到 tar"
[ -f "${SRC_DIR}/package.json" ] || die "找不到前端源码：${SRC_DIR}/package.json 不存在。前端源码应随仓库一起存在。"
[ -f "${SRC_DIR}/package-lock.json" ] || die "找不到 ${SRC_DIR}/package-lock.json（npm ci 依赖它锁定依赖版本）"

# ---------- 1. 安装依赖并构建 ----------
cd "${SRC_DIR}"
# 可复现构建：用记录在 frontend-build.env 里的上游 commit 时间当构建时间。上游
# vite.config.ts:53 把 new Date().toISOString() 经 define.__BUILD_TIME__ 注入产物
# （Footer.tsx:26 显示），会让承载它的 chunk 改名并连锁影响文件名与 SW 预缓存 revision；
# 我们已把 buildTime 改成优先读 SOURCE_DATE_EPOCH（reproducible-builds 约定）。
export SOURCE_DATE_EPOCH="${SOURCE_DATE_EPOCH:-${KOMARI_FRONTEND_SOURCE_DATE_EPOCH}}"
log "SOURCE_DATE_EPOCH=${SOURCE_DATE_EPOCH}"
[ -n "${SOURCE_DATE_EPOCH}" ] || die "frontend-build.env 缺少 KOMARI_FRONTEND_SOURCE_DATE_EPOCH"

log "npm ci（依 package-lock.json 锁定依赖）"
npm ci --no-audit --no-fund >/dev/null
log "npm run build（VITE_KOMARI_UPDATE_REPO=${VITE_KOMARI_UPDATE_REPO:-${KOMARI_UPDATE_REPO}}）"
VITE_KOMARI_UPDATE_REPO="${VITE_KOMARI_UPDATE_REPO:-${KOMARI_UPDATE_REPO}}" npm run build >/dev/null
[ -f dist/index.html ] || die "构建未产出 dist/index.html"

# ---------- 2. 暂存并原子替换注入目录 ----------
STAGE="${REPO_ROOT}/.build/defaultTheme-stage"
rm -rf "${STAGE}"
mkdir -p "${STAGE}/dist"
cp -R dist/. "${STAGE}/dist/"
cp -f komari-theme.json "${STAGE}/"
# 与上游 action.yml:45-48 行为对齐（两个拼写都放，兼容历史差异）
[ -f preview.png ] && cp -f preview.png "${STAGE}/preview.png"
[ -f perview.png ] && cp -f perview.png "${STAGE}/perview.png"
if [ -f "${STAGE}/preview.png" ] && [ ! -f "${STAGE}/perview.png" ]; then cp -f "${STAGE}/preview.png" "${STAGE}/perview.png"; fi
if [ -f "${STAGE}/perview.png" ] && [ ! -f "${STAGE}/preview.png" ]; then cp -f "${STAGE}/perview.png" "${STAGE}/preview.png"; fi
[ -f "${STAGE}/dist/index.html" ] || die "暂存目录缺少 dist/index.html"

rm -rf "${DEST_DIR}"
mkdir -p "$(dirname "${DEST_DIR}")"
mv "${STAGE}" "${DEST_DIR}"
log "已注入 ${DEST_DIR}（$(du -sh "${DEST_DIR}" | cut -f1)）"

# ---------- 3. 产物侧反回归校验 ----------
if grep -rIq "komari-monitor/komari/releases" "${DEST_DIR}"; then
  die "产物中仍存在上游更新检查地址 komari-monitor/komari/releases"
fi
if grep -rIq "komari-monitor/komari-agent" "${DEST_DIR}"; then
  die "产物中仍指向上游 agent（安装命令应指向本仓库）"
fi

# ---------- 4. 哈希校验 ----------
HASH="$(tree_hash "${DEST_DIR}")"
log "目录树哈希: ${HASH}"
if [ -n "${FRONTEND_TREE_SHA256:-}" ]; then
  if [ "${FRONTEND_TREE_SHA256}" != "${HASH}" ]; then
    die "哈希不一致：期望 ${FRONTEND_TREE_SHA256}，实际 ${HASH}。
     说明依赖解析或产物发生变化。确认接受后，把 scripts/frontend-build.env 的
     FRONTEND_TREE_SHA256 更新为上面的实际值，并在提交信息中说明原因。"
  fi
  log "哈希校验通过（与 scripts/frontend-build.env 一致）"
else
  log "scripts/frontend-build.env 的 FRONTEND_TREE_SHA256 为空，请回填：FRONTEND_TREE_SHA256=\"${HASH}\""
fi
log "完成。接下来：./scripts/build-komari.sh（前端产物内嵌进服务器二进制）"
