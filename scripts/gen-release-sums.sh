#!/usr/bin/env bash
#
# 生成服务端二进制的校验和资产 dist/komari-SHA256SUMS。
#
# 为什么单独一个脚本：两个平台的静态构建是两次独立的 build-komari.sh 调用，
# 在构建脚本里"追加一行"会在失败/重跑时留下过期条目；这里按最终产物整体重算，
# 缺文件就报错，不会生成半份清单。
#
# 用法：
#   ./scripts/gen-release-sums.sh            # 使用 dist/
#   ./scripts/gen-release-sums.sh <目录>     # 指定目录
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
DIST_DIR="${1:-${REPO_ROOT}/dist}"

ASSETS=(komari-linux-amd64 komari-linux-arm64)
for f in "${ASSETS[@]}"; do
    if [ ! -f "${DIST_DIR}/${f}" ]; then
        echo "[gen-release-sums] 缺少 ${DIST_DIR}/${f}（先跑 KOMARI_OUTPUT=... ./scripts/build-komari.sh）" >&2
        exit 1
    fi
done

( cd "${DIST_DIR}" && sha256sum "${ASSETS[@]}" > komari-SHA256SUMS )
echo "[gen-release-sums] 已生成 ${DIST_DIR}/komari-SHA256SUMS"
cat "${DIST_DIR}/komari-SHA256SUMS"
