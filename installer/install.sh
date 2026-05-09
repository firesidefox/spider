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

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PLIST_LABEL="ai.fty.spider"
BIN_DIR="$HOME/.local/bin"
DATA_DIR="$HOME/.spider/data"
LOG_DIR="$HOME/.spider/logs"
OS="$(uname -s)"
PLIST_DST=""

h1 "Spider 安装"

if [ "$(id -u)" -eq 0 ]; then
  error "请勿以 root 用户运行此脚本，直接运行 ./install.sh 即可。"
  exit 1
fi

step "停止旧版本服务"
if [ "$OS" = "Darwin" ]; then
  launchctl bootout "gui/$(id -u)/${PLIST_LABEL}" 2>/dev/null || true
  for i in $(seq 10); do
    launchctl print "gui/$(id -u)/${PLIST_LABEL}" >/dev/null 2>&1 || break
    sleep 1
  done
elif [ "$OS" = "Linux" ]; then
  systemctl --user stop spider 2>/dev/null || true
fi
success "旧服务已停止（或不存在）"

step "安装二进制"
mkdir -p "$BIN_DIR"
install -m 755 "${SCRIPT_DIR}/spider" "$BIN_DIR/spider"
install -m 755 "${SCRIPT_DIR}/spdctl" "$BIN_DIR/spdctl"
success "spider / spdctl → $BIN_DIR/"

step "创建日志目录"
mkdir -p "$LOG_DIR"
success "$LOG_DIR 已就绪"

step "创建数据目录"
mkdir -p "$DATA_DIR"
success "$DATA_DIR 已就绪"

step "安装内置 Skills"
if [ -d "${SCRIPT_DIR}/skills" ]; then
  cp -r "${SCRIPT_DIR}/skills/." "$DATA_DIR/skills/"
  success "Skills → $DATA_DIR/skills/"
else
  warn "未找到 skills 目录，跳过"
fi

step "安装服务配置"
if [ "$OS" = "Darwin" ]; then
  PLIST_DST="$HOME/Library/LaunchAgents/${PLIST_LABEL}.plist"
  mkdir -p "$HOME/Library/LaunchAgents"
  sed "s|__HOME__|$HOME|g" "${SCRIPT_DIR}/spider.plist" > "$PLIST_DST"
  chmod 644 "$PLIST_DST"
  success "$PLIST_DST"
elif [ "$OS" = "Linux" ]; then
  SERVICE_DST="$HOME/.config/systemd/user/spider.service"
  mkdir -p "$HOME/.config/systemd/user"
  cat > "$SERVICE_DST" <<EOF
[Unit]
Description=Spider AI
After=network.target

[Service]
ExecStart=$BIN_DIR/spider serve --data-dir $DATA_DIR
Restart=always
StandardOutput=append:$LOG_DIR/spider.log
StandardError=append:$LOG_DIR/spider.log

[Install]
WantedBy=default.target
EOF
  systemctl --user daemon-reload
  success "$SERVICE_DST"
fi

step "检查端口 8000"
if [ "$OS" = "Darwin" ]; then
  _port_check() { lsof -iTCP:8000 -sTCP:LISTEN -t >/dev/null 2>&1; }
  _port_list()  { lsof -iTCP:8000 -sTCP:LISTEN; }
  _port_pid()   { lsof -iTCP:8000 -sTCP:LISTEN -t 2>/dev/null; }
  _service_hint() { detail "   然后同步修改 ~/Library/LaunchAgents/ai.fty.spider.plist，重新运行 install.sh"; }
else
  _port_check() { ss -tlnp 2>/dev/null | grep -q ':8000 '; }
  _port_list()  { ss -tlnp 2>/dev/null | grep ':8000 '; }
  _port_pid()   { ss -tlnp 2>/dev/null | grep ':8000 ' | grep -oP 'pid=\K[0-9]+' | head -1; }
  _service_hint() { detail "   然后同步修改 ~/.config/systemd/user/spider.service，重新运行 install.sh"; }
fi
if _port_check; then
  error "端口 8000 已被占用"
  printf "\n" >&2
  _port_list >&2
  printf "\n" >&2
  printf "  ${yellow}解决方案：${reset}\n" >&2
  detail "1. 停止占用进程：kill $(_port_pid)"
  detail "2. 或修改监听端口：编辑 ~/.spider/data/config.yaml，设置 addr: :9090"
  _service_hint
  exit 1
fi
success "端口 8000 可用"

step "启动服务"
_bootstrap_err=$(mktemp)
trap 'rm -f "$_bootstrap_err"' EXIT
if [ "$OS" = "Darwin" ]; then
  if ! launchctl bootstrap "gui/$(id -u)" "$PLIST_DST" 2>"$_bootstrap_err"; then
    error "launchctl bootstrap 失败"
    cat "$_bootstrap_err" >&2
    detail "查看日志：tail -f $LOG_DIR/spider.log"
    detail "手动启动：$BIN_DIR/spider"
    exit 1
  fi
elif [ "$OS" = "Linux" ]; then
  if ! systemctl --user enable --now spider 2>"$_bootstrap_err"; then
    error "systemctl enable 失败"
    cat "$_bootstrap_err" >&2
    detail "查看日志：tail -f $LOG_DIR/spider.log"
    detail "手动启动：$BIN_DIR/spider"
    exit 1
  fi
  step "启用开机自启（linger）"
  if sudo loginctl enable-linger "$(whoami)" 2>/dev/null; then
    success "linger 已启用，开机无需登录即可运行"
  else
    warn "linger 启用失败（需要 sudo），服务仅在登录后运行"
    detail "手动执行：sudo loginctl enable-linger $(whoami)"
  fi
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
    printf "\r  ${yellow}⚠ 服务未响应，查看日志：tail -f $LOG_DIR/spider.log${reset}\n"
  fi
done

h1 "安装完成"
detail "spdctl host list    # 查看主机列表"
detail "spdctl mcp register # 注册到 Claude Code"

if [[ ":$PATH:" != *":$BIN_DIR:"* ]]; then
  printf "\n  ${yellow}PATH 提示：${reset}\n"
  printf "  $BIN_DIR 不在 PATH 中，请添加到 ~/.zshrc 或 ~/.bashrc：\n"
  printf "  ${bold}export PATH=\"\$HOME/.local/bin:\$PATH\"${reset}\n"
fi

printf "\n  ${yellow}首次登录提示：${reset}\n"
printf "  初始管理员密码已打印到服务日志，运行以下命令查看：\n"
printf "  ${bold}grep 'default admin created' $LOG_DIR/spider.log${reset}\n"
