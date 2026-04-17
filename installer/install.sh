#!/usr/bin/env bash
set -euo pipefail

# ── 日志 ────────────────────────────────────────────────
set +e
bold=$(tput bold 2>/dev/null); reset=$(tput sgr0 2>/dev/null)
red=$(tput setaf 1 2>/dev/null); green=$(tput setaf 76 2>/dev/null)
yellow=$(tput setaf 202 2>/dev/null); blue=$(tput setaf 25 2>/dev/null)
dim=$(tput dim 2>/dev/null || true)
set -e

h1()      { printf "\n${bold}${blue}══ %s ══${reset}\n" "$*"; }
step()    { printf "  ${blue}▶ %s...${reset}\n" "$*"; }
success() { printf "  ${green}✔ %s${reset}\n" "$*"; }
warn()    { printf "  ${yellow}⚠ %s${reset}\n" "$*"; }
error()   { printf "  ${red}✖ %s${reset}\n" "$*" >&2; }
detail()  { printf "    ${dim}%s${reset}\n" "$*"; }
# ────────────────────────────────────────────────────────

if [[ $EUID -ne 0 ]]; then
  error "请使用 sudo 运行此脚本"
  detail "sudo ./install.sh"
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PLIST_LABEL="ai.fty.spider"
PLIST_DST="/Library/LaunchDaemons/${PLIST_LABEL}.plist"

h1 "Spider 安装"

step "停止旧版本服务"
launchctl bootout "system/${PLIST_LABEL}" 2>/dev/null || true
success "旧服务已停止（或不存在）"

step "安装二进制"
install -m 755 "${SCRIPT_DIR}/spider" /usr/local/bin/spider
install -m 755 "${SCRIPT_DIR}/spdctl" /usr/local/bin/spdctl
success "spider / spdctl → /usr/local/bin/"

step "创建日志目录"
mkdir -p /var/log/spider
chmod 755 /var/log/spider
success "/var/log/spider 已就绪"

step "安装 launchd plist"
install -m 644 "${SCRIPT_DIR}/spider.plist" "${PLIST_DST}"
success "${PLIST_DST}"

step "检查端口 8000"
if lsof -iTCP:8000 -sTCP:LISTEN -t >/dev/null 2>&1; then
  error "端口 8000 已被占用"
  printf "\n" >&2
  lsof -iTCP:8000 -sTCP:LISTEN >&2
  printf "\n" >&2
  printf "  ${yellow}解决方案：${reset}\n" >&2
  detail "1. 停止占用进程：kill $(lsof -iTCP:8000 -sTCP:LISTEN -t 2>/dev/null)"
  detail "2. 或修改监听端口：编辑 /etc/spider/config.yaml，设置 addr: :9090"
  detail "   然后同步修改 spider.plist，重新运行 install.sh"
  exit 1
fi
success "端口 8000 可用"

step "启动服务"
launchctl bootstrap system "${PLIST_DST}"

step "验证服务"
sleep 1
if curl -sf http://localhost:8000/health >/dev/null 2>&1; then
  success "Spider 已启动：http://localhost:8000"
else
  warn "服务可能尚未就绪，稍后执行：curl http://localhost:8000/health"
fi

h1 "安装完成"
detail "spdctl host list    # 查看主机列表"
detail "spdctl mcp register # 注册到 Claude Code"
