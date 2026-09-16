#!/usr/bin/env bash
#
# 从 pin 住的上游前端 commit 重新生成默认主题，并注入 web/public/defaultTheme/。
#
# 为什么需要这个脚本：本后端仓库无法单独构建——web/public/public.go:18 是
# `//go:embed defaultTheme`，缺少该目录时 static() 会 panic（public.go:130）。
# 上游用 .github/actions/build-frontend/action.yml 在 CI 里临时克隆 komari-web 生成它，
# 但普通 tag（如 1.4.3）不锁前端版本（action.yml:34 退化为默认分支），
# 因此我们把产物 vendor 进仓库，并用本脚本以固定 commit 重新生成。
#
# 用法：
#   ./scripts/sync-frontend.sh            # 重新生成 + 校验哈希
#   VITE_KOMARI_UPDATE_REPO=owner/repo ./scripts/sync-frontend.sh   # 覆盖更新检查目标
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
# shellcheck source=scripts/frontend-pin.env
source "${SCRIPT_DIR}/frontend-pin.env"

WORK_DIR="${KOMARI_WEB_WORK_DIR:-${REPO_ROOT}/.build/komari-web}"
DEST_DIR="${REPO_ROOT}/web/public/defaultTheme"
PATCH_DIR="${SCRIPT_DIR}/patches"

log() { printf '[sync-frontend] %s\n' "$*" >&2; }
die() { printf '[sync-frontend] ERROR: %s\n' "$*" >&2; exit 1; }

# 目录树规范化哈希：排序 + 归零 mtime/owner + gzip -n，保证同一份内容跨机器同哈希。
tree_hash() {
  ( cd "$1" && tar --sort=name --mtime='@0' --owner=0 --group=0 --numeric-owner \
      -cf - . | gzip -n -9 | sha256sum | awk '{print $1}' )
}

command -v git >/dev/null 2>&1 || die "未找到 git"
command -v npm >/dev/null 2>&1 || die "未找到 npm（本步骤需要 Node/npm；只想构建后端请直接用 scripts/build-komari.sh，vendor 产物已在仓库内）"
command -v tar  >/dev/null 2>&1 || die "未找到 tar"
[ -n "${KOMARI_WEB_COMMIT:-}" ] || die "scripts/frontend-pin.env 缺少 KOMARI_WEB_COMMIT"

# ---------- 1. 取到 pin 的 commit ----------
if [ ! -d "${WORK_DIR}/.git" ]; then
  log "克隆 ${KOMARI_WEB_REPO} → ${WORK_DIR}"
  rm -rf "${WORK_DIR}"
  mkdir -p "$(dirname "${WORK_DIR}")"
  git clone --filter=blob:none --no-checkout "${KOMARI_WEB_REPO}" "${WORK_DIR}"
fi
cd "${WORK_DIR}"
log "检出 ${KOMARI_WEB_COMMIT}（tag ${KOMARI_WEB_TAG}）"
if ! git fetch --depth 1 origin "${KOMARI_WEB_COMMIT}" >/dev/null 2>&1; then
  log "按 SHA 拉取失败，回退到 tag refs/tags/${KOMARI_WEB_TAG}"
  git fetch --depth 1 origin "refs/tags/${KOMARI_WEB_TAG}" >/dev/null 2>&1 \
    || die "既无法按 SHA 也无法按 tag ${KOMARI_WEB_TAG} 拉取 ${KOMARI_WEB_REPO}"
fi
git checkout --force "${KOMARI_WEB_COMMIT}" >/dev/null 2>&1 || die "无法检出 ${KOMARI_WEB_COMMIT}"
# 彻底清理上一次的构建残留，保证可复现（npm ci 会重建 node_modules）
git clean -xfdq
[ "$(git rev-parse HEAD)" = "${KOMARI_WEB_COMMIT}" ] || die "HEAD 不是 pin 的 commit"

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

# ---------- 3. 构建 ----------
# 可复现构建：以 pin commit 的提交时间作为构建时间。上游 vite.config.ts:53 把
# new Date().toISOString() 经 define.__BUILD_TIME__ 注入产物（Footer.tsx:26 显示），
# 会让该 chunk 内容与全部文件名哈希随时间连锁变化；补丁 0002 使其可被
# SOURCE_DATE_EPOCH（reproducible-builds 约定）覆盖。
SOURCE_DATE_EPOCH="${SOURCE_DATE_EPOCH:-$(git show -s --format=%ct "${KOMARI_WEB_COMMIT}")}"
export SOURCE_DATE_EPOCH
log "SOURCE_DATE_EPOCH=${SOURCE_DATE_EPOCH}"

log "npm ci（依 package-lock.json 锁定依赖）"
npm ci --no-audit --no-fund >/dev/null
log "npm run build（VITE_KOMARI_UPDATE_REPO=${VITE_KOMARI_UPDATE_REPO:-${KOMARI_UPDATE_REPO}}）"
VITE_KOMARI_UPDATE_REPO="${VITE_KOMARI_UPDATE_REPO:-${KOMARI_UPDATE_REPO}}" npm run build >/dev/null
[ -f dist/index.html ] || die "构建未产出 dist/index.html"

# ---------- 4. 暂存并原子替换注入目录 ----------
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

# ---------- 5. 产物侧反回归校验 ----------
if grep -rIq "komari-monitor/komari/releases" "${DEST_DIR}"; then
  die "产物中仍存在上游更新检查地址 komari-monitor/komari/releases（补丁未生效）"
fi

# ---------- 6. 哈希校验 ----------
HASH="$(tree_hash "${DEST_DIR}")"
log "目录树哈希: ${HASH}"
if [ -n "${FRONTEND_TREE_SHA256:-}" ]; then
  if [ "${FRONTEND_TREE_SHA256}" != "${HASH}" ]; then
    die "哈希不一致：期望 ${FRONTEND_TREE_SHA256}，实际 ${HASH}。
     说明依赖解析或产物发生变化。确认接受后，把 scripts/frontend-pin.env 的
     FRONTEND_TREE_SHA256 更新为上面的实际值，并在提交信息中说明原因。"
  fi
  log "哈希校验通过（与 scripts/frontend-pin.env 一致）"
else
  log "scripts/frontend-pin.env 的 FRONTEND_TREE_SHA256 为空，请回填：FRONTEND_TREE_SHA256=\"${HASH}\""
fi
log "完成。接下来：./scripts/build-komari.sh"
