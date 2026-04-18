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

step "创建数据目录"
mkdir -p /var/lib/spider
chmod 700 /var/lib/spider
success "/var/lib/spider 已就绪"

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
if ! launchctl bootstrap system "${PLIST_DST}" 2>/tmp/spider-bootstrap.err; then
  error "launchctl bootstrap 失败"
  cat /tmp/spider-bootstrap.err >&2
  detail "查看日志：tail -f /var/log/spider/spider.log"
  detail "手动启动：/usr/local/bin/spider"
  exit 1
fi

step "验证服务"
spinner="⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏"
for i in $(seq 5); do
  frame="${spinner:$(( (i-1) % ${#spinner} )):1}"
  printf "\r  ${blue}%s 等待服务就绪 (%d/5)...${reset}" "$frame" "$i"
  sleep 1
  if curl -sf http://localhost:8000/health >/dev/null 2>&1; then
    printf "\r  ${green}✔ Spider 已启动：http://localhost:8000${reset}\n"
    break
  fi
  if [[ $i -eq 5 ]]; then
    printf "\r  ${yellow}⚠ 服务未响应，查看日志：tail -f /var/log/spider/spider.log${reset}\n"
  fi
done

h1 "安装完成"
detail "spdctl host list    # 查看主机列表"
detail "spdctl mcp register # 注册到 Claude Code"

printf "\n  ${yellow}首次登录提示：${reset}\n"
printf "  初始管理员密码已打印到服务日志，运行以下命令查看：\n"
printf "  ${bold}sudo grep 'default admin created' /var/log/spider/spider.log${reset}\n"
