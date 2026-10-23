#!/usr/bin/env bash
# 一键构建：服务端本地镜像 + 主题 zip（含 sha256）
#
# 用法： ./build.sh [--skip-install] [--skip-server] [--skip-theme]   # 在仓库根执行（本仓 = 内核 + 主题 + 部署脚本）
# 产物：
#   server/bin/komari                       服务端二进制
#   dist/server/komari-<版本>-<own 后缀>.tar.gz?  → 仅构建镜像，本地 tag
#   dist/theme/ink-build-<sha>.zip + .sha256
#   dist/BUILD-INFO.txt
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT"

SKIP_INSTALL=0; SKIP_SERVER=0; SKIP_THEME=0
for a in "$@"; do
  case "$a" in
    --skip-install) SKIP_INSTALL=1 ;;
    --skip-server) SKIP_SERVER=1 ;;
    --skip-theme) SKIP_THEME=1 ;;
    -h|--help) sed -n '2,14p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "未知参数：$a" >&2; exit 2 ;;
  esac
done

# 兼容 version.env 的两种写法：KOMARI_VERSION="0.0.24" 或 "${KOMARI_VERSION:-0.0.24}"
SERVER_VERSION="$(bash -c 'unset KOMARI_VERSION; . scripts/version.env >/dev/null 2>&1; printf %s "${KOMARI_VERSION}"' 2>/dev/null || true)"
if [ -z "$SERVER_VERSION" ]; then
  SERVER_VERSION="$(python3 -c "
import re
s = open('scripts/version.env').read()
m = re.search(r'KOMARI_VERSION=.*?([0-9]+\.[0-9]+\.[0-9]+)', s)
print(m.group(1) if m else '')" )"
fi
[ -n "$SERVER_VERSION" ] || { echo "无法从 server/scripts/version.env 解析版本" >&2; exit 1; }
OWN_SUFFIX="${OWN_SUFFIX:-own.1}"
SEMA_TAG="${SERVER_VERSION}+${OWN_SUFFIX}"       # 语义版本（+ 元数据）
IMAGE_TAG="komari-own:${SERVER_VERSION}-${OWN_SUFFIX}"  # docker tag 不允许 '+'
THEME_VERSION="$(python3 -c "import json;print(json.load(open('themes/ink/komari-theme.json'))['version'])")"
GIT_SHORT="$(git rev-parse --short HEAD 2>/dev/null || echo nogit)"

mkdir -p ops-dist/theme
rm -f ops-dist/theme/*.zip ops-dist/theme/*.sha256

if [ "$SKIP_SERVER" = "0" ]; then
  echo "== 服务端：构建二进制（动态链接）=="
  ./scripts/build-komari.sh
  echo "   产物: $(ls -la bin/komari | awk '{print $5" 字节"}')"
  echo "== 服务端：构建本地镜像 ${IMAGE_TAG} =="
  CTX="$ROOT/.build-image"
  rm -rf "$CTX"; mkdir -p "$CTX"
  cp deploy/Dockerfile.local "$CTX/Dockerfile"
  cp bin/komari "$CTX/komari"
  docker build -q -t "${IMAGE_TAG}" "$CTX" >/dev/null
  rm -rf "$CTX"
  echo "   镜像: ${IMAGE_TAG}"
fi

if [ "$SKIP_THEME" = "0" ]; then
  echo "== 主题：安装依赖并构建（版本 ${THEME_VERSION}）=="
  cd themes/ink
  if [ "$SKIP_INSTALL" = "0" ]; then bun install --frozen-lockfile >/dev/null; fi
  rm -f ink-build-*.zip
  bun run build >/dev/null
  ZIP="$(ls ink-build-*.zip | head -1)"
  [ -n "$ZIP" ] || { echo "主题构建未产出 zip" >&2; exit 1; }
  ZIP_PATH="$ROOT/ops-ops-dist/theme/$ZIP"
  cp "$ZIP" "$ZIP_PATH"
  ( cd "$ROOT/ops-dist/theme" && sha256sum "$ZIP" > "$ZIP.sha256" )
  # 产物内 manifest 版本必须与仓库一致
  INNER_VER="$(python3 - "$ROOT/ops-ops-dist/theme/$ZIP" <<'PY'
import sys, zipfile
z = zipfile.ZipFile(sys.argv[1])
import json
print(json.loads(z.read('komari-theme.json'))['version'])
PY
)"
  [ "$INNER_VER" = "$THEME_VERSION" ] || { echo "包内 manifest 版本 ${INNER_VER} != ${THEME_VERSION}" >&2; exit 1; }
  echo "   产物: ops-ops-dist/theme/$ZIP"
  echo "   sha256: $(awk '{print $1}' "$ROOT/ops-ops-dist/theme/$ZIP.sha256")"
  cd "$ROOT"
fi

{
  echo "build time : $(date -u +%Y-%m-%dT%H:%M:%SZ)"
  echo "git        : $GIT_SHORT"
  echo "server     : $SERVER_VERSION (semver $SEMA_TAG, image ${IMAGE_TAG})"
  echo "theme      : $THEME_VERSION (我们的自有版本线，与上游 0.6.x 无关)"
  if [ -n "${ZIP_PATH:-}" ] && [ -f "$ZIP_PATH" ]; then
    echo "theme zip  : $(basename "$ZIP_PATH")  $(stat -c %s "$ZIP_PATH") bytes  sha256=$(sha256sum "$ZIP_PATH" | awk '{print $1}')"
  fi
} > ops-dist/BUILD-INFO.txt

echo
echo "== 构建完成 =="
cat ops-dist/BUILD-INFO.txt
