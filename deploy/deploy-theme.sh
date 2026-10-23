#!/usr/bin/env bash
# 主题部署脚本：备份 → sha256 校验 → 替换 data/theme/<name> → 打印回滚命令
#
# 用法：
#   ./deploy/deploy-theme.sh --zip dist/theme/ink-build-xxxx.zip --host <user@host>
#   ./deploy/deploy-theme.sh --zip ... --host ... --dry-run     # 只打印计划，不触碰远端
# 可选：--theme-name（默认 ink）、--remote-theme-root（默认 /root/komari/data/theme）、
#       --remote-backup-dir（默认 /root/backups）
#
# 约定：不重启任何服务；主题为静态产物，替换后刷新页面即生效。
set -euo pipefail

ZIP=""; HOST="${KOMARI_HOST:-}"; THEME_NAME="ink"
REMOTE_THEME_ROOT="/root/komari/data/theme"
REMOTE_BACKUP_DIR="/root/backups"
DRY_RUN=0

while [ $# -gt 0 ]; do
  case "$1" in
    --zip) ZIP="${2:?}"; shift 2 ;;
    --host) HOST="${2:?}"; shift 2 ;;
    --theme-name) THEME_NAME="${2:?}"; shift 2 ;;
    --remote-theme-root) REMOTE_THEME_ROOT="${2:?}"; shift 2 ;;
    --remote-backup-dir) REMOTE_BACKUP_DIR="${2:?}"; shift 2 ;;
    --dry-run) DRY_RUN=1; shift ;;
    -h|--help) sed -n '2,11p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "未知参数：$1" >&2; exit 2 ;;
  esac
done

die() { echo "错误：$*" >&2; exit 1; }

[ -n "$ZIP" ] || die "缺少 --zip"
[ -f "$ZIP" ] || die "zip 不存在：$ZIP"
[ -n "$HOST" ] || die "缺少 --host（或环境变量 KOMARI_HOST）"

LOCAL_SHA="$(sha256sum "$ZIP" | awk '{print $1}')"
SIDE_CAR="${ZIP}.sha256"
if [ -f "$SIDE_CAR" ]; then
  EXPECT="$(awk '{print $1}' "$SIDE_CAR")"
  [ "$EXPECT" = "$LOCAL_SHA" ] || die "本地 sha256 与 sidecar 不一致（$LOCAL_SHA vs $EXPECT）"
  echo "[ok] 本地 sha256 与 sidecar 一致：$LOCAL_SHA"
else
  echo "[warn] 未找到 $SIDE_CAR，仅使用实算 sha256：$LOCAL_SHA"
fi

REMOTE_ZIP="$REMOTE_BACKUP_DIR/$(basename "$ZIP")"
REMOTE_THEME_DIR="$REMOTE_THEME_ROOT/$THEME_NAME"

cat <<PLAN
计划：
  1) 上传   $ZIP  →  $HOST:$REMOTE_ZIP
  2) 远端校验 sha256 = $LOCAL_SHA
  3) 备份   $HOST:$REMOTE_THEME_DIR → $REMOTE_BACKUP_DIR/theme-$THEME_NAME-<版本>_<UTC时间戳>.tgz（含条目数校验）
  4) 替换   $HOST:$REMOTE_THEME_DIR（仅清空 dist 后解包，不动其它文件）
  5) 打印回滚命令（不重启任何服务）
PLAN

if [ "$DRY_RUN" = "1" ]; then echo "[dry-run] 未执行任何远端操作。"; exit 0; fi

echo "[1/5] 上传 zip"
scp -q "$ZIP" "$HOST:$REMOTE_ZIP"

echo "[2/5] 远端 sha256 校验"
REMOTE_SHA="$(ssh "$HOST" bash -s -- "$REMOTE_ZIP" <<'EOS'
set -euo pipefail
sha256sum "$1" | awk '{print $1}'
EOS
)"
[ "$REMOTE_SHA" = "$LOCAL_SHA" ] || die "远端 sha256 不一致（$REMOTE_SHA）⇒ 中止，未改动线上"
echo "[ok] 远端 sha256 一致"

echo "[3/5] 备份现有主题（并校验条目数）"
BAK="$(ssh "$HOST" bash -s -- "$REMOTE_BACKUP_DIR" "$REMOTE_THEME_ROOT" "$THEME_NAME" <<'EOS'
set -euo pipefail
BACKUP_DIR="$1"; THEME_ROOT="$2"; THEME_NAME="$3"
THEME_DIR="$THEME_ROOT/$THEME_NAME"
[ -f "$THEME_DIR/komari-theme.json" ] || { echo "远端主题目录不完整：$THEME_DIR" >&2; exit 5; }
TS="$(date -u +%Y%m%d%H%M%S)"
VER="$(python3 -c "import json;print(json.load(open('$THEME_DIR/komari-theme.json'))['version'])" 2>/dev/null || echo unknown)"
BAK="$BACKUP_DIR/theme-${THEME_NAME}-${VER}_${TS}.tgz"
tar czf "$BAK" -C "$THEME_ROOT" "$THEME_NAME"
N_ARC="$(tar tzf "$BAK" | grep -vc '/$' || true)"
N_SRC="$(find "$THEME_DIR" -type f | wc -l)"
echo "  备份条目=$N_ARC ｜ 源文件=$N_SRC ｜ 版本=$VER" >&2
[ "$N_ARC" = "$N_SRC" ] || { echo "备份条目与源文件数不一致 ⇒ 中止（线上未改动）" >&2; exit 6; }
printf '%s' "$BAK"
EOS
)"
echo "[ok] 备份：$BAK"

echo "[4/5] 替换主题产物"
ssh "$HOST" bash -s -- "$REMOTE_ZIP" "$REMOTE_THEME_DIR" <<'EOS'
set -euo pipefail
ZIP="$1"; THEME_DIR="$2"
[ -f "$ZIP" ] || { echo "远端缺 zip：$ZIP" >&2; exit 7; }
rm -rf "$THEME_DIR/dist"
python3 - "$ZIP" "$THEME_DIR" <<'PY'
import sys, zipfile
zip_path, target = sys.argv[1], sys.argv[2]
with zipfile.ZipFile(zip_path) as z:
    z.extractall(target)
    print('  解包条目:', len(z.namelist()))
PY
chmod -R a+rX "$THEME_DIR"
echo -n '  替换后文件数: '; find "$THEME_DIR" -type f | wc -l
echo -n '  manifest 版本: '; python3 -c "import json;print(json.load(open('$THEME_DIR/komari-theme.json'))['version'])"
EOS

echo "[5/5] 回滚命令（请保存）："
echo "  ssh $HOST \"rm -rf $REMOTE_THEME_DIR && tar xzf $BAK -C $REMOTE_THEME_ROOT\""
echo "完成：主题已替换（静态生效，未重启服务）。"
