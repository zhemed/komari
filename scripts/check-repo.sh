#!/usr/bin/env bash
#
# 仓库自检：一次跑完"我们这个仓库是否自洽"的所有机械检查。
#
# 用法：
#   ./scripts/check-repo.sh          # 快速检查（秒级）
#   ./scripts/check-repo.sh --full   # 追加 Go 质量门禁、离线构建、agent 门禁（数分钟）
#
# 设计原则：只做**能机械判定**的检查，不猜测意图；发现不一致就打印并计入失败，
# 全部通过时以 0 退出。发版前应当跑一次 --full（见 docs/MAINTAINING.md §3.4）。
#
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${REPO_ROOT}"

FULL=0
[ "${1:-}" = "--full" ] && FULL=1

FAIL=0
ok()   { printf '  \033[32m✓\033[0m %s\n' "$*"; }
bad()  { printf '  \033[31m✗\033[0m %s\n' "$*"; FAIL=$((FAIL + 1)); }
head_() { printf '\n[check-repo] %s\n' "$*"; }

# ---------- 1. 版本字面量一致性 ----------
head_ "1. 版本字面量一致性"
# shellcheck source=scripts/version.env
. "${SCRIPT_DIR}/version.env"
VER="${KOMARI_VERSION}"
check_literal() {
  local file="$1" pattern="$2" label="$3"
  if grep -q -- "$pattern" "$file"; then ok "${label} = ${VER}"; else bad "${label} 不是 ${VER}（$file）"; fi
}
check_literal install-komari.sh "REPO_TAG=\"\${KOMARI_TAG:-${VER}}\"" "install-komari.sh REPO_TAG"
check_literal install-agent.sh "^default_agent_version=\"${VER}\"$" "install-agent.sh default_agent_version"
check_literal install-agent.ps1 "^\\\$DefaultAgentVersion = \"${VER}\"$" "install-agent.ps1 \$DefaultAgentVersion"
if grep -q "当前 \*\*${VER}\*\*" docs/MAINTAINING.md; then ok "MAINTAINING 里声明的当前版本 = ${VER}"; else bad "MAINTAINING 里声明的当前版本不是 ${VER}"; fi
if git describe --tags --exact-match >/dev/null 2>&1; then
  [ "$(git describe --tags --exact-match)" = "${VER}" ] \
    && ok "当前提交正好是 tag ${VER}" \
    || bad "当前提交的 tag 与 version.env（${VER}）不一致：$(git describe --tags --exact-match)"
else
  ok "当前提交不是 tag（开发中，跳过 tag 比对）"
fi

# ---------- 2. 文档引用完整性 ----------
head_ "2. 文档里引用的仓库内路径存在，且 file:line 锚点没超界"
# 只查"我们自己的东西"（构建输入、我们维护的目录），不查 Go import 路径 / HTTP 路由 /
# owner/repo slug / 运行时数据路径——那些本来就不是文件路径，逐条判断只会产生噪声。
ref_ok=0; missing=0; out_of_range=0
for f in README.md docs/MAINTAINING.md .trellis/spec/backend/*.md; do
  dir="$(dirname "$f")"
  while IFS= read -r line; do
    # 同行已明确说明"删除/移除/不再/历史"的引用不查（写的是曾经如此）
    case "$line" in *删除*|*移除*|*不再*|*历史*|*曾是*|*曾经*) continue ;; esac
    for ref in $(printf '%s' "$line" | sed 's/~~[^~]*~~//g' \
        | grep -oE '`[^`]+`' | tr -d '`' | sed 's/:[0-9]*-\?[0-9]*$//; s/:[0-9]\+$//'); do
      case "$ref" in
        scripts/*|frontend/*|agent/*|docs/*.md|.trellis/*|web/public/defaultTheme/*|web/public/defaultTheme) ;;
        install-komari.sh|install-agent.sh|install-agent.ps1|Dockerfile|Dockerfile.agent|README.md|LICENSE|NOTICE) ;;
        *) continue ;;
      esac
      case "$ref" in *'*'*|*'<'*|*'http'*|*'...'*) continue ;; esac
      if [ -e "${ref#./}" ] || [ -e "${dir}/${ref#./}" ]; then
        ref_ok=$((ref_ok + 1))
      else
        bad "$f 引用了不存在的路径：$ref"; missing=$((missing + 1))
      fi
    done
    # file:line 锚点：文件存在时，行号不得超过该文件行数（文件被削短后最容易飘的就是这种）
    for anchor in $(printf '%s' "$line" | grep -oE '`[A-Za-z0-9_./-]+\.[A-Za-z0-9]+:[0-9]+`' | tr -d '`'); do
      afile="${anchor%:*}"; aline="${anchor##*:}"
      [ -f "$afile" ] || continue
      total="$(wc -l < "$afile")"
      [ "$aline" -le "$total" ] || { bad "$f 的锚点 ${anchor} 超出文件行数（实际 ${total} 行）"; out_of_range=$((out_of_range + 1)); }
    done
  done < "$f"
done
[ "$missing" = 0 ] && [ "$out_of_range" = 0 ] \
  && ok "引用的仓库内路径全部存在（${ref_ok} 条），file:line 锚点均未超界" \
  || true

# ---------- 3. 脚本语法 ----------
head_ "3. 脚本语法"
syn_fail=0
for s in scripts/*.sh install-komari.sh install-agent.sh; do
  if head -1 "$s" | grep -q "sh$" && ! head -1 "$s" | grep -q "bash"; then
    sh -n "$s" 2>/dev/null || { bad "$s 语法错误"; syn_fail=$((syn_fail + 1)); }
  else
    bash -n "$s" 2>/dev/null || { bad "$s 语法错误"; syn_fail=$((syn_fail + 1)); }
  fi
done
[ "$syn_fail" = 0 ] && ok "scripts/*.sh + 两个安装脚本语法正常"

# ---------- 4. 跟踪文件卫生 ----------
head_ "4. 不该入库的东西没入库"
# 注意：scripts/*.env 是刻意的配置文件（version.env / *-build.env），不算脏文件；
# 只拦真正不该入库的：依赖目录、构建产物、运行期数据、根目录 .env 与 *.db
junk="$(git ls-files | grep -E "node_modules/|^frontend/dist/|^data/|^bin/|(^|/)\.env$|(^|/)\.env\.local$|\.[ds]?db$" || true)"
[ -z "$junk" ] && ok "没有 node_modules / dist / data / bin / .env / *.db 被跟踪" || { bad "以下文件不该入库："; echo "$junk" | sed 's/^/      /'; }
untracked_src="$(git status --porcelain frontend agent 2>/dev/null | grep -cE '^\?\?' || true)"
[ "${untracked_src:-0}" = 0 ] && ok "frontend/ 与 agent/ 源码无未跟踪文件" || bad "frontend/ 或 agent/ 里有未跟踪文件（源码应当全部入库）"

# ---------- 5. 敏感串扫描 ----------
head_ "5. 敏感串扫描（密钥/私钥）"
hits="$(git grep -nIE "(ghp_[A-Za-z0-9]{20,}|github_pat_|sk-[A-Za-z0-9]{20,}|AKIA[0-9A-Z]{16}|-----BEGIN [A-Z ]*PRIVATE KEY-----)" -- . 2>/dev/null | grep -v '^frontend/' || true)"
[ -z "$hits" ] && ok "未发现形如 token/私钥的字符串" || { bad "疑似密钥："; echo "$hits" | head -5 | sed 's/^/      /'; }

# ---------- 6. 前端产物哈希 ----------
head_ "6. 前端产物与 frontend-build.env 记录一致"
# shellcheck source=scripts/frontend-build.env
. "${SCRIPT_DIR}/frontend-build.env"
if [ -d web/public/defaultTheme ]; then
  HASH="$( cd web/public/defaultTheme && tar --sort=name --mtime='@0' --owner=0 --group=0 --numeric-owner -cf - . | gzip -n -9 | sha256sum | awk '{print $1}' )"
  [ "${FRONTEND_TREE_SHA256}" = "${HASH}" ] \
    && ok "目录树哈希 = ${HASH}" \
    || bad "目录树哈希不一致：记录 ${FRONTEND_TREE_SHA256} / 实际 ${HASH}"
else
  bad "web/public/defaultTheme 不存在（后端无法构建，见 MAINTAINING §1）"
fi

# ---------- 7. 构建期不再克隆上游 ----------
head_ "7. 构建脚本不克隆上游"
clones="$(grep -nE "^\s*(git clone|git fetch)" scripts/*.sh | grep -v "^\s*#" || true)"
[ -z "$clones" ] && ok "scripts/ 里没有构建期 git clone/fetch" || { bad "仍有构建期克隆："; echo "$clones" | sed 's/^/      /'; }

# ---------- 慢检查 ----------
if [ "${FULL}" = "1" ]; then
  head_ "8. Go 质量门禁"
  if go build ./... && go vet ./... && go test ./... >/tmp/check-repo-test.log 2>&1; then
    ok "go build / vet / test 全绿"
  else
    bad "Go 门禁失败（详见 /tmp/check-repo-test.log）"; tail -3 /tmp/check-repo-test.log | sed 's/^/      /'
  fi

  head_ "9. 离线构建（模块缓存已预热时）"
  if GOPROXY=off GOFLAGS=-mod=mod ./scripts/build-komari.sh >/dev/null 2>&1; then
    ok "GOPROXY=off 构建成功"
  else
    bad "离线构建失败（本仓库没有 vendor/，需要模块缓存已预热）"
  fi

  head_ "10. agent 构建门禁"
  if ./scripts/build-agent.sh --only linux/amd64 >/tmp/check-repo-agent.log 2>&1; then
    ok "agent 单平台构建 + 三道门禁通过"
  else
    bad "agent 构建门禁失败（详见 /tmp/check-repo-agent.log）"; tail -3 /tmp/check-repo-agent.log | sed 's/^/      /'
  fi
else
  printf '\n[check-repo] 提示：加 --full 可追加 Go 门禁、离线构建与 agent 门禁\n'
fi

printf '\n[check-repo] 结论：'
if [ "${FAIL}" = 0 ]; then
  printf '\033[32m全部通过\033[0m\n'; exit 0
else
  printf '\033[31m%d 项未通过\033[0m\n' "${FAIL}"; exit 1
fi
