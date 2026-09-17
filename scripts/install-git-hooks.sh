#!/usr/bin/env bash
#
# 安装 Trellis 提交闸门（第一层）。新克隆仓库后跑一次即可。
#
#   ./scripts/install-git-hooks.sh
#
# 它只改**本地**配置 core.hooksPath=.githooks（不随仓库分发），所以每个克隆都要跑一次；
# scripts/check-repo.sh 第 8 项会检查是否已安装。
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${REPO_ROOT}"

if [ ! -d .githooks ]; then
  printf '✗ 找不到 .githooks/（仓库不完整？）\n' >&2
  exit 1
fi

git config core.hooksPath .githooks
chmod +x .githooks/* 2>/dev/null || true

printf '✓ 已设置 core.hooksPath=.githooks（本地配置，不随仓库分发）\n'
printf '  现在：改动非 .trellis/ 文件前必须有进行中的任务，提交消息必须带 [task:<slug>]\n'
printf '  规则：AGENTS.md「强制规则」/ docs/MAINTAINING.md「流程闸门」\n'
