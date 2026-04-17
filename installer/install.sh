#!/usr/bin/env bash
set -euo pipefail

if [[ $EUID -ne 0 ]]; then
  echo "错误：请使用 sudo 运行此脚本" >&2
  echo "  sudo ./install.sh" >&2
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PLIST_LABEL="ai.fty.spider"
PLIST_DST="/Library/LaunchDaemons/${PLIST_LABEL}.plist"

echo "▶ 停止旧版本服务（如有）..."
launchctl bootout "system/${PLIST_LABEL}" 2>/dev/null || true

echo "▶ 安装二进制..."
install -m 755 "${SCRIPT_DIR}/spider"  /usr/local/bin/spider
install -m 755 "${SCRIPT_DIR}/spdctl"  /usr/local/bin/spdctl

echo "▶ 创建日志目录..."
mkdir -p /var/log/spider
chmod 755 /var/log/spider

echo "▶ 安装 launchd plist..."
install -m 644 "${SCRIPT_DIR}/spider.plist" "${PLIST_DST}"

echo "▶ 检查端口 8000..."
if lsof -iTCP:8000 -sTCP:LISTEN -t >/dev/null 2>&1; then
  echo "✖ 错误：端口 8000 已被占用" >&2
  echo "" >&2
  echo "占用进程：" >&2
  lsof -iTCP:8000 -sTCP:LISTEN >&2
  echo "" >&2
  echo "解决方案：" >&2
  echo "  1. 停止占用进程：kill $(lsof -iTCP:8000 -sTCP:LISTEN -t 2>/dev/null)" >&2
  echo "  2. 或修改 Spider 监听端口：编辑 /etc/spider/config.yaml，设置 addr: :9090" >&2
  echo "     然后同步修改 spider.plist 中的健康检查地址，重新运行 install.sh" >&2
  exit 1
fi

echo "▶ 启动服务..."
launchctl bootstrap system "${PLIST_DST}"

echo "▶ 验证服务..."
sleep 1
if curl -sf http://localhost:8000/health >/dev/null 2>&1; then
  echo "✔ Spider 已启动：http://localhost:8000"
else
  echo "⚠ 服务可能尚未就绪，请稍后执行：curl http://localhost:8000/health"
fi

echo ""
echo "✔ 安装完成"
echo "  spdctl host list    # 查看主机列表"
echo "  spdctl mcp register # 注册到 Claude Code"
