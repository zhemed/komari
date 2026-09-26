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
# 第 8 项是流程闸门（scripts/check-trellis-gate.sh）：提交必须绑定 Trellis 任务。
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
# 当前版本声明必须与 version.env 一致——**两处都查**（2026-09-26 修 README 漂到 0.0.18 时发现：
# 原来只查 MAINTAINING，README 写死版本号从未被校验，升了几版都没人发现）
ver_declare_ok=1
if grep -qF "**当前版本：\`${VER}\`**" docs/MAINTAINING.md; then
  ok "MAINTAINING 里声明的当前版本 = ${VER}"
else
  bad "MAINTAINING 里声明的当前版本不是 ${VER}（应含 **当前版本：\`${VER}\`**）"; ver_declare_ok=0
fi
README_VER="$(grep -oE '（当前 0\.0\.[0-9]+）' README.md 2>/dev/null | head -1 | grep -oE '[0-9]+\.[0-9]+\.[0-9]+')"
if [ -z "${README_VER}" ]; then
  bad "README 里找不到当前版本声明（应含「（当前 ${VER}）」）"; ver_declare_ok=0
elif [ "${README_VER}" = "${VER}" ]; then
  ok "README 的当前版本声明 = ${VER}"
else
  bad "README 的当前版本声明是 ${README_VER}，与 version.env（${VER}）不一致"; ver_declare_ok=0
fi
if [ "${ver_declare_ok}" = 1 ]; then
  ok "文档版本声明一致（不依赖 scripts/version.env 之外的写死值）"
fi
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
# is_ignored：被 .gitignore 覆盖的路径**故意不在磁盘上**（构建产物/本地数据），
# 一律不算缺失——2026-09-19 清理本地产物后，spec 里写的 frontend/node_modules/、
# frontend/dist/ 就属于这一类：它们"不在磁盘上"是正确状态，不是文档写错。
is_ignored() {
  git check-ignore -q -- "$1" 2>/dev/null && return 0
  git check-ignore -q -- "${1%/}/" 2>/dev/null && return 0
  return 1
}
ref_ok=0; missing=0; out_of_range=0; ref_ignored=0
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
      elif is_ignored "${ref#./}" || is_ignored "${dir}/${ref#./}"; then
        ref_ignored=$((ref_ignored + 1))
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
# 无论通过与否都报出计数——失败时也要能一眼看出"哪一类、几条"，自测脚本同样依赖这行做断言。
if [ "$missing" = 0 ] && [ "$out_of_range" = 0 ]; then
  ok "引用的仓库内路径全部存在（引用 ${ref_ok} 条；另有 ${ref_ignored} 条被 .gitignore 覆盖的可重建/本地路径按设计跳过；缺失 0 条，锚点超界 0 条）"
else
  printf '  \033[33m·\033[0m 引用检查：引用 %s 条；另有 %s 条被 .gitignore 覆盖的可重建/本地路径按设计跳过；缺失 %s 条，锚点超界 %s 条\n' \
    "${ref_ok}" "${ref_ignored}" "${missing}" "${out_of_range}"
fi

# ---------- 3. 脚本语法 ----------
head_ "3. 脚本语法 + 工作流 YAML"
syn_fail=0
for s in scripts/*.sh install-komari.sh install-agent.sh .githooks/pre-commit .githooks/commit-msg; do
  if head -1 "$s" | grep -q "sh$" && ! head -1 "$s" | grep -q "bash"; then
    sh -n "$s" 2>/dev/null || { bad "$s 语法错误"; syn_fail=$((syn_fail + 1)); }
  else
    bash -n "$s" 2>/dev/null || { bad "$s 语法错误"; syn_fail=$((syn_fail + 1)); }
  fi
done
for wf in .github/workflows/*.yml .github/workflows/*.yaml; do
  [ -f "$wf" ] || continue
  if command -v python3 >/dev/null 2>&1 && python3 -c "import yaml" 2>/dev/null; then
    python3 -c "import sys, yaml; yaml.safe_load(open(sys.argv[1]))" "$wf" 2>/tmp/check-repo-yaml.log \
      || { bad "$wf YAML 解析失败（$(tail -1 /tmp/check-repo-yaml.log)）"; syn_fail=$((syn_fail + 1)); }
  fi
done
[ "$syn_fail" = 0 ] && ok "scripts/*.sh + 安装脚本 + .githooks 语法正常，工作流 YAML 可解析"

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
# 排除 vendored 前端、以及本脚本自己（它的正则里必然包含这些模式）。
hits="$(git grep -nIE "(ghp_[A-Za-z0-9]{20,}|github_pat_|sk-[A-Za-z0-9]{20,}|AKIA[0-9A-Z]{16}|-----BEGIN [A-Z ]*PRIVATE KEY-----)" -- . 2>/dev/null | grep -vE '^(frontend/|scripts/check-repo\.sh:)' || true)"
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

# ---------- 8. Trellis 流程闸门 ----------
head_ "8. Trellis 流程闸门（hooks 已装 + 提交可追溯）"
# 规则与三层闸门见 AGENTS.md「强制规则」、docs/MAINTAINING.md「流程闸门」。
# 这里跑第二层审计：本地 hooks 是否安装 + 起点之后每个改动提交是否都带 [task:<slug>]。
"${SCRIPT_DIR}/check-trellis-gate.sh" || bad "Trellis 闸门未通过（见上面 ✗ 行）"

# ---------- 慢检查 ----------
if [ "${FULL}" = "1" ]; then
  head_ "9. Go 质量门禁"
  if go build ./... && go vet ./... && go test ./... >/tmp/check-repo-test.log 2>&1; then
    ok "go build / vet / test 全绿"
  else
    bad "Go 门禁失败（详见 /tmp/check-repo-test.log）"; tail -3 /tmp/check-repo-test.log | sed 's/^/      /'
  fi

  head_ "10. 离线构建（模块缓存已预热时）"
  if GOPROXY=off GOFLAGS=-mod=mod ./scripts/build-komari.sh >/dev/null 2>&1; then
    ok "GOPROXY=off 构建成功"
  else
    bad "离线构建失败（本仓库没有 vendor/，需要模块缓存已预热）"
  fi

  head_ "11. agent 构建门禁"
  # 必须换输出目录：build-agent.sh 默认写 dist/agent 且开头就 rm -rf 该目录，
  # 直接跑会把刚构建好的发版资产清成单平台产物（2026-09-17 实际踩到——按
  # docs/MAINTAINING.md §3.4 的顺序“先建资产、再跑 --full”，自检把 14 个 agent 产物冲掉）。
  if KOMARI_AGENT_OUTPUT="${REPO_ROOT}/.build/check-agent" ./scripts/build-agent.sh --only linux/amd64 >/tmp/check-repo-agent.log 2>&1; then
    ok "agent 单平台构建 + 三道门禁通过"
  else
    bad "agent 构建门禁失败（详见 /tmp/check-repo-agent.log）"; tail -3 /tmp/check-repo-agent.log | sed 's/^/      /'
  fi
else
  printf '\n[check-repo] 提示：加 --full 可追加 Go 门禁、离线构建与 agent 门禁\n'
fi

# ---------- 12. 自检的自测（防止"检查器自己坏掉却报绿"） ----------
# 2026-09-19：第 2 项曾把被 .gitignore 覆盖的可重建目录误报为文档错误（4 条假失败）；
# 修它的时候必须同时证明"没变瞎"，所以固化成自动化断言（scripts/check-repo-selftest.sh）。
# 递归护栏：自测内部会在临时仓库里跑本脚本，用 KOMARI_SKIP_SELFTEST 断开二次调用。
if [ "${KOMARI_SKIP_SELFTEST:-0}" = 1 ]; then
  printf '\n[check-repo] 12. 自检自测：已按 KOMARI_SKIP_SELFTEST=1 跳过\n'
elif [ -x "${REPO_ROOT}/scripts/check-repo-selftest.sh" ]; then
  head_ "12. 自检自测（第 2 项路径/锚点检查的判别性断言）"
  if ./scripts/check-repo-selftest.sh >/tmp/check-repo-selftest.log 2>&1; then
    ok "自测全绿（$(grep -c '✓' /tmp/check-repo-selftest.log) 条断言）"
  else
    bad "自检自测失败（详见 /tmp/check-repo-selftest.log）"; grep '✗' /tmp/check-repo-selftest.log | sed 's/^/      /'
  fi
else
  printf '\n[check-repo] 12. 自检自测：scripts/check-repo-selftest.sh 不存在，跳过\n'
fi

# ---------- 13. upgrade 路径不依赖"引擎运行时元数据" ----------
# 2026-09-19 加的护栏，起因是本仓库最严重的一次失误：compose 全自动升级。
# 详见 .trellis/spec/guides/incident-compose-autosync.md 与 spec/backend/server-upgrade.md。
# 只扫**代码**（go 文件），不扫注释/文档——规范文档里本来就会提到这些词。
#
# 判据分两级，故意的：
#   * 硬失败：容器 label 依赖（`com.docker.compose.*`）与那两个已删除的文件（selfid.go / compose.go）。
#     它们正是事故的设计本身，重新出现必须有人先读事故案例。
#   * 提示（不算失败）：`DetectSelfContainerID` 是 0.0.17 的**合法存量**（默认检测路径），
#     但它在 host 网络下会静默失效——见 spec/backend/server-upgrade.md §5 的已知遗留。
#     把它判成失败等于要求删掉存量功能，那是越权；所以只提示，并把遗留文档指出来。
head_ "13. upgrade 路径不依赖引擎运行时元数据（compose 事故护栏）"
META_HARD='com\.docker\.compose|selfid\.go|compose\.go'
META_SOFT='DetectSelfContainerID'
META_POSTMORTEM='.trellis/spec/guides/incident-compose-autosync.md'
scan_meta() {  # scan_meta <模式> [git 树引用]
  local pat="$1" ref="${2:-}"
  if [ -n "${ref}" ]; then
    git grep -nE "${pat}" "${ref}" -- 'internal/upgrade/*.go' 'cmd/dockerSelfRecreate.go' 2>/dev/null
  else
    git grep -nE "${pat}" -- 'internal/upgrade/*.go' 'cmd/dockerSelfRecreate.go' 2>/dev/null
  fi
}
HARD_HITS="$(scan_meta "${META_HARD}")"
if [ -z "${HARD_HITS}" ]; then
  ok "当前代码没有容器 label / 引擎元数据依赖（判据：com.docker.compose 标签路径、compose 同步符号）"
else
  bad "upgrade 路径重新出现引擎运行时元数据依赖——先读 ${META_POSTMORTEM} 与 spec/backend/server-upgrade.md 的硬规则"
  printf '%s\n' "${HARD_HITS}" | sed 's/^/      /'
fi
SOFT_HITS="$(scan_meta "${META_SOFT}")"
if [ -z "${SOFT_HITS}" ]; then
  ok "自身容器识别不再走 hostname/cgroup 线索（无遗留提示）"
else
  printf '  \033[33m·\033[0m 提示：仍在使用 DetectSelfContainerID（合法存量）——它在 host 网络下会静默失效，见 spec/backend/server-upgrade.md §5 已知遗留\n'
fi
# 判别性验证：硬规则必须能抓住历史缺陷，否则它只是装饰（对 dd0486a 必须报红）
if [ -n "$(scan_meta "${META_HARD}" dd0486a)" ]; then
  ok "判别性验证：硬规则对已回滚的历史提交 dd0486a 报红（说明规则真的会报警）"
else
  bad "判别性验证失败：硬规则对历史提交 dd0486a 无告警，等于永远为绿（护栏失效）"
fi

# ---------- 14. 部署口径（2026-09-19 用户纠偏后重写） ----------
# 用户定稿：**主方案 = Docker 镜像**（docker run … ghcr.io/zhemed/komari:latest + 面板一键升级），
# 备选 = 二进制 + systemd（install-komari.sh）；**compose 已明确要求剔除**。
# 这条守卫防的是"部署口径漂移/再分叉"：多一套部署自动化 = 多一条必须在生产上验证的路径，
# 而上一套（compose）就是这么出事的。
head_ "14. 部署口径（Docker 主 + systemd 备；compose 已剔除）"
# 判据：仓库根（含 scripts/）里带 `deploy-entry:` 标记的只有三个文件——服务器的 install-komari.sh
# 与 agent 的 install-agent.sh/.ps1。新增任何第二个部署自动化都不该带这个标记，于是被挡下。
DEPLOY_ALLOWED='install-komari.sh|install-agent.sh|install-agent.ps1'
deploy_entries() {   # 扫仓库根（排除 .git/node_modules/.build/dist，否则递归进无关树）
  grep -rl '^# deploy-entry:' --include='*.sh' --include='*.ps1' \
    --exclude-dir=.git --exclude-dir=node_modules --exclude-dir=.build --exclude-dir=dist \
    . 2>/dev/null | sed 's|^\./||' | sort
}
DEPLOY_OK=1
DEPLOY_FOUND="$(deploy_entries)"
DEPLOY_EXTRA="$(printf '%s\n' "${DEPLOY_FOUND}" | grep -vE "^(${DEPLOY_ALLOWED})$" || true)"
if [ -n "${DEPLOY_EXTRA}" ]; then
  bad "发现第二个部署/升级入口（只允许 ${DEPLOY_ALLOWED}）：$(printf '%s' "${DEPLOY_EXTRA}" | tr '\n' ' ')"
  bad "  用户 2026-09-19 定调：部署/升级只用一条命令，见 docs/MAINTAINING.md §3.4.1"
  DEPLOY_OK=0
fi
grep -qE "^REPO_TAG=\"\\\$\{KOMARI_TAG:-${VER}\}\"" install-komari.sh \
  || { bad "install-komari.sh 的默认 tag 不是 ${VER}（与 scripts/version.env 不一致）"; DEPLOY_OK=0; }
grep -q '^BINARY_PATH="\$INSTALL_DIR/komari"$' install-komari.sh \
  || { bad "install-komari.sh 不再装到 \$INSTALL_DIR/komari（部署口径变了，需人确认）"; DEPLOY_OK=0; }
grep -q '^WorkingDirectory=\${DATA_DIR}$' install-komari.sh \
  || { bad "install-komari.sh 的 systemd 单元不再以 \${DATA_DIR} 为工作目录（数据路径口径变了）"; DEPLOY_OK=0; }
if [ "${DEPLOY_OK}" = 1 ]; then
  ok "部署口径就位：Docker 主方案 + systemd 备选（install-komari.sh，默认 tag = ${VER}）+ agent 安装脚本；部署自动化未分叉"
fi
if [ "${FULL}" = 1 ]; then
  grep -q '^upgrade_komari()' install-komari.sh \
    || { bad "install-komari.sh 里没有 upgrade_komari()——唯一升级路径不能丢"; DEPLOY_OK=0; }
  grep -q 'whiptail\|dialog' install-komari.sh \
    || { bad "install-komari.sh 的 TUI 改成别的实现了（MAINTAINING §3.4.1 的驱动要点会失效，需同步更新）"; DEPLOY_OK=0; }
  # 主方案口径：README 必须以 Docker 镜像为第一条部署命令（用户 2026-09-19 定稿）
  grep -q 'docker run -d --name komari --restart always' README.md \
    || { bad "README 不再以 Docker 镜像作为主部署命令（部署口径被改动了）"; DEPLOY_OK=0; }
  grep -q 'ghcr.io/zhemed/komari:latest' README.md \
    || { bad "README 缺少 ghcr.io/zhemed/komari:latest 镜像地址"; DEPLOY_OK=0; }
  grep -q '3\.4\.1 部署口径' docs/MAINTAINING.md \
    || { bad "MAINTAINING 缺少 §3.4.1「部署口径」"; DEPLOY_OK=0; }
  # 剔除 compose（用户 2026-09-19 明确要求）：用户面文档与运行代码里都不许再出现。
  # 必须排除本脚本自身——它得写着这条模式才能查别人（首版没排除，把自己抓了）。
  COMPOSE_PAT='docker compose|docker-compose|compose\.ya?ml'
  if git grep -qE "${COMPOSE_PAT}" -- README.md install-komari.sh 'scripts/*.sh' internal/ cmd/ \
       ':(exclude)scripts/check-repo.sh' 2>/dev/null; then
    bad "README/安装脚本/运行代码里重新出现 compose——用户已明确要求剔除"
    git grep -nE "${COMPOSE_PAT}" -- README.md install-komari.sh 'scripts/*.sh' internal/ cmd/ \
      ':(exclude)scripts/check-repo.sh' | head -5 | sed 's/^/      /'
    DEPLOY_OK=0
  fi
  # 判别性验证：**真的**造一个 install-compose.sh（带 deploy-entry 标记）到仓库根，
  # 守卫必须报出"第二个部署入口"；验证完立刻删掉（trap 保证异常也会清）。
  GUARD_TMP="${REPO_ROOT}/install-compose.sh"
  printf '#!/usr/bin/env bash\n# deploy-entry: 模仿被禁的第二套部署自动化（判别性验证用）\n' > "${GUARD_TMP}"
  trap 'rm -f "${GUARD_TMP}"' EXIT
  if [ -n "$(printf '%s\n' "$(deploy_entries)" | grep -vE "^(${DEPLOY_ALLOWED})$" || true)" ]; then
    ok "判别性验证：造一个 install-compose.sh 立刻被本项命中（守卫不是摆设）"
  else
    bad "判别性验证失败：造出 install-compose.sh 后守卫没报警（本项永远为绿）"
  fi
  rm -f "${GUARD_TMP}"; trap - EXIT
fi

# ---------- 15. 2FA 不得被重新引入 ----------
# 2026-09-20 用户要求**彻底移除 2FA**（账号级 + 敏感操作验证两层），并选择加这条护栏防回归。
# 回归的真实路径：从上游快照拷代码时把 pquerna/otp 一起带回来、或看到数据库还剩 two_factor 列
# 就顺手把模型字段加回去。所以：扫 Go 源码 / go.mod / 前端源码，命中即报红。
head_ "15. 2FA 不得被重新引入（用户 2026-09-20 要求彻底移除）"
TWOFA_PAT='pquerna/otp|two_factor|TwoFactor|RequireSensitive2FA|api/admin/2fa'
twofa_scan() {  # twofa_scan [git 树引用]
  local ref="${1:-}"
  if [ -n "${ref}" ]; then
    git grep -lE "${TWOFA_PAT}" "${ref}" -- '*.go' 'go.mod' 'frontend/src' 2>/dev/null
  else
    git grep -lE "${TWOFA_PAT}" -- '*.go' 'go.mod' 'frontend/src' 2>/dev/null
  fi
}
TWOFA_HITS="$(twofa_scan)"
if [ -z "${TWOFA_HITS}" ]; then
  ok "代码里没有 2FA 痕迹（判据：pquerna/otp、two_factor、TwoFactor、RequireSensitive2FA、/api/admin/2fa）"
else
  bad "2FA 又被加回来了——用户 2026-09-20 明确要求彻底移除，不要补回来"
  printf '%s\n' "${TWOFA_HITS}" | sed 's/^/      /'
fi
# 判别性验证：规则必须能抓住"还有 2FA"的历史提交，否则它只是装饰
if [ -n "$(twofa_scan 0.0.18)" ]; then
  ok "判别性验证：规则对 0.0.18（含 2FA 的历史提交）报红（说明规则真的会报警）"
else
  bad "判别性验证失败：规则对含 2FA 的历史提交无告警，等于永远为绿（护栏失效）"
fi

printf '\n[check-repo] 结论：'
if [ "${FAIL}" = 0 ]; then
  printf '\033[32m全部通过\033[0m\n'; exit 0
else
  printf '\033[31m%d 项未通过\033[0m\n' "${FAIL}"; exit 1
fi
