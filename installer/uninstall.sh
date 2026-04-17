#!/usr/bin/env bash
set -euo pipefail

if [[ $EUID -ne 0 ]]; then
  echo "错误：请使用 sudo 运行此脚本" >&2
  echo "  sudo ./uninstall.sh" >&2
  exit 1
fi

PLIST_LABEL="ai.fty.spider"
PLIST_PATH="/Library/LaunchDaemons/${PLIST_LABEL}.plist"

echo "▶ 停止服务..."
launchctl bootout "system/${PLIST_LABEL}" 2>/dev/null || true

echo "▶ 删除 launchd plist..."
rm -f "${PLIST_PATH}"

echo "▶ 删除二进制..."
rm -f /usr/local/bin/spider /usr/local/bin/spdctl

echo ""
echo "✔ 卸载完成"
echo "  数据目录 ~/.spider/ 已保留，如需删除请手动执行："
echo "  rm -rf ~/.spider"
