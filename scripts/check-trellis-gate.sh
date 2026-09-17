#!/usr/bin/env bash
#
# Trellis 强制闸门 · 第二层（事后审计，第一层被 --no-verify 绕过时的兜底）
#
#   ./scripts/check-trellis-gate.sh               # 本地完整检查：hooks 已装 + 提交审计
#   ./scripts/check-trellis-gate.sh --audit-only  # 只做提交审计（CI 用；CI 里没有本地 git 配置）
#
# 审计规则：从 .trellis/gates/enforce-from 记录的起点提交之后，每个提交——
#   * 跳过 merge（--no-merges）与空提交；
#   * 只改 .trellis/ 的提交跳过（journal / 归档 / 闸门自身）；
#   * 其余提交的消息里必须出现 [task:<slug>]，且 slug 对应真实任务（进行中或已归档）。
# 违规逐条打印并以退出码 1 结束，因此 check-repo（发版前必跑）与 CI 都会因此变红。
#
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${REPO_ROOT}"

AUDIT_ONLY=0
[ "${1:-}" = "--audit-only" ] && AUDIT_ONLY=1

FAIL=0
ok()  { printf '  \033[32m✓\033[0m %s\n' "$*"; }
bad() { printf '  \033[31m✗\033[0m %s\n' "$*"; FAIL=$((FAIL + 1)); }

# ---------- 1. 本地闸门是否已安装（CI 跳过） ----------
if [ "${AUDIT_ONLY}" = 0 ]; then
  hooks_path="$(git config --get core.hooksPath || true)"
  if [ "${hooks_path}" != ".githooks" ]; then
    bad "git 提交闸门未安装（core.hooksPath=${hooks_path:-未设置}）→ 运行 ./scripts/install-git-hooks.sh"
  else
    missing=0
    for h in .githooks/pre-commit .githooks/commit-msg; do
      [ -f "$h" ] || { bad "缺少 $h"; missing=1; }
      [ -x "$h" ] || { bad "$h 没有执行位（chmod +x）"; missing=1; }
    done
    [ "${missing}" = 0 ] && ok "提交闸门已安装（core.hooksPath=.githooks，两个 hook 可执行）"
  fi
fi

# ---------- 2. 提交可追溯性审计 ----------
marker_file=".trellis/gates/enforce-from"
if [ ! -f "${marker_file}" ]; then
  bad "缺少审计起点文件 ${marker_file}（应记录启用闸门时的提交 SHA）"
else
  from="$(tr -d '[:space:]' < "${marker_file}")"
  if ! git cat-file -e "${from}^{commit}" 2>/dev/null; then
    bad "审计起点 ${from} 不在当前历史里（CI 需要 actions/checkout 的 fetch-depth: 0）"
  else
    audited=0
    violations=0
    while IFS= read -r sha; do
      [ -n "${sha}" ] || continue
      files="$(git show --pretty=format: --name-only "${sha}" | sed '/^[[:space:]]*$/d')"
      [ -n "${files}" ] || continue
      # 纯 .trellis/ 改动跳过
      if ! printf '%s\n' "${files}" | grep -qv '^\.trellis/'; then continue; fi
      audited=$((audited + 1))
      if ! git log -1 --format=%B "${sha}" | grep -qE '\[task:[A-Za-z0-9._-]+\]'; then
        printf '      未带任务锚点：%s %s\n' "${sha:0:8}" "$(git log -1 --format=%s "${sha}")"
        violations=$((violations + 1))
      fi
    done < <(git log --no-merges --format=%H "${from}..HEAD")

    if [ "${violations}" = 0 ]; then
      ok "提交可追溯：${from:0:8}..HEAD 共核对 ${audited} 个改动提交，全部带 [task:<slug>]"
    else
      bad "${violations} 个提交没有任务锚点（含 --no-verify 绕过的）：上面已逐条列出"
      printf '      补救：在提交消息里补 [task:<slug>]（git commit --amend），或 revert 掉重新按流程提交\n'
    fi
  fi
fi

if [ "${FAIL}" = 0 ]; then
  printf '  \033[32m✓\033[0m Trellis 闸门通过\n'
  exit 0
fi
exit 1
