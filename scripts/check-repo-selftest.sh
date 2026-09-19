#!/usr/bin/env bash
#
# check-repo.sh 第 2 项（文档引用完整性）的自测：**专门抓这类"检查器自己坏了却报绿"的回归**。
#
# 为什么会存在这个脚本：2026-09-19 清理本地产物后，第 2 项把"被 .gitignore 覆盖、
# 因此本来就不在磁盘上"的路径（frontend/node_modules/、frontend/dist/）报成文档错误，
# 4 条假失败。修完之后必须能证明两件事同时成立：
#   ① 真缺失仍然报错（不能因为放宽而变瞎）；
#   ② 被忽略的可重建路径被跳过；且**关键负例**——被忽略路径的 file:line 锚点超界
#      仍然报错（这条保证"跳过"没有顺带把锚点检查也一起跳过）。
#
# 判据全部来自期望值比对（用 bash 正则而非 grep -P，不依赖 PCRE）。
# 用法：./scripts/check-repo-selftest.sh
set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PASS=0; FAIL=0
ok()  { printf '  \033[32m✓\033[0m %s\n' "$*"; PASS=$((PASS + 1)); }
bad() { printf '  \033[31m✗\033[0m %s\n' "$*"; FAIL=$((FAIL + 1)); }

TMP="$(mktemp -d)"
trap 'rm -rf "${TMP}"' EXIT

mkdir -p "${TMP}/scripts"
cp "${REPO_ROOT}/scripts/check-repo.sh" "${TMP}/scripts/"
cp "${REPO_ROOT}/scripts/version.env" "${TMP}/scripts/"
cp "${REPO_ROOT}/scripts/check-trellis-gate.sh" "${TMP}/scripts/" 2>/dev/null || true
git -C "${TMP}" init -q
git -C "${TMP}" config user.email selftest@example.invalid
git -C "${TMP}" config user.name selftest
printf 'frontend/dist/\nfrontend/node_modules/\n' > "${TMP}/.gitignore"
git -C "${TMP}" add -A >/dev/null 2>&1
git -C "${TMP}" -c commit.gpgsign=false commit -qm init --no-verify >/dev/null 2>&1

# 夹具：三行分别是"真缺失"、"被忽略路径 + 合法锚点"、"被忽略路径 + 超界锚点"
mkdir -p "${TMP}/docs"
cat > "${TMP}/docs/MAINTAINING.md" <<'FIXTURE'
# fixture
- 真缺失：`scripts/definitely-missing-file.sh`
- 被忽略且本地没有：`frontend/node_modules/`（生成物，不入库）
- 被忽略但磁盘上有：`frontend/dist/`
- 锚点合法：`scripts/version.env:3`
- 锚点超界：`frontend/dist/index.html:99999`
FIXTURE

# 关键：让被忽略目录里的文件**真的存在**（模拟"本地构建过"的脏状态）。
# 这样第 2 项的锚点检查会读到它，我们才能验证"跳过被忽略路径"没有顺带跳过锚点校验：
# 目录被 .gitignore 覆盖 → 该条引用按设计跳过（不计缺失）；但文件在磁盘上存在 →
# 锚点 99999 仍然必须报超界。
mkdir -p "${TMP}/frontend/dist"
printf '<!doctype html>\n<title>fixture</title>\n' > "${TMP}/frontend/dist/index.html"

cd "${TMP}"
set +e
KOMARI_SKIP_SELFTEST=1 ./scripts/check-repo.sh > "${TMP}/out.txt" 2>&1
RC=$?
set -e 2>/dev/null || true
cd "${REPO_ROOT}"
OUT="$(cat "${TMP}/out.txt")"
printf '  （被检脚本退出码 = %s，符合预期：夹具里确实有真缺失）\n' "${RC}"

expect() {  # expect <期望数> <提取正则> <标签>
  local want="$1" re="$2" label="$3" got=0
  if [[ "${OUT}" =~ ${re} ]]; then got="${BASH_REMATCH[1]}"; fi
  if [ "${got}" = "${want}" ]; then ok "${label} = ${want}"; else bad "${label} 期望 ${want}，实际 ${got}"; fi
}

echo "[check-repo-selftest] 用 ${TMP} 里的夹具跑真实 check-repo.sh 第 2 项"
# 夹具期望值（逐条对应上面 MAINTAINING.md 的五行）：
#   真缺失 → 缺失 1；被忽略且本地没有 → 跳过 1；被忽略但磁盘上有 → 计入引用；
#   version.env:3 合法 → 计入引用；frontend/dist/index.html:99999 → 超界 1。
# 即：缺失 1 条、超界 1 条、跳过 1 条、计入引用 3 条。
expect 1 '缺失 ([0-9]+) 条' '缺失引用数'
expect 1 '锚点超界 ([0-9]+) 条' '锚点超界数'
expect 1 '另有 ([0-9]+) 条被 \.gitignore 覆盖' '被忽略跳过数'
expect 3 '引用 ([0-9]+) 条' '计入引用的条数'

if [[ "${OUT}" == *"引用了不存在的路径：scripts/definitely-missing-file.sh"* ]]; then
  ok "真缺失路径被报出"
else
  bad "真缺失路径没有被报出（检查器变瞎了）"
fi
if [[ "${OUT}" != *"引用了不存在的路径：frontend/dist/"* ]]; then
  ok "被 .gitignore 覆盖的路径没有被误报为缺失"
else
  bad "被 .gitignore 覆盖的路径仍被误报为缺失"
fi
if [[ "${OUT}" == *"超出文件行数"* ]]; then
  ok "被忽略路径的锚点超界仍然被检出（跳过没有连锚点一起跳过）"
else
  bad "被忽略路径的锚点超界漏检"
fi

echo
if [ "${FAIL}" = 0 ]; then
  printf '[check-repo-selftest] 结论：\033[32m全部通过\033[0m（%d 项）\n' "${PASS}"
  exit 0
fi
printf '[check-repo-selftest] 结论：\033[31m%d 项未通过\033[0m\n' "${FAIL}"
exit 1
