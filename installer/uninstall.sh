#!/usr/bin/env bash
set -euo pipefail

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

PLIST_LABEL="ai.fty.spider"
BIN_DIR="$HOME/.local/bin"
OS="$(uname -s)"

h1 "Spider 卸载"

step "停止服务"
if [ "$OS" = "Darwin" ]; then
  launchctl bootout "gui/$(id -u)/${PLIST_LABEL}" 2>/dev/null || true
  rm -f "$HOME/Library/LaunchAgents/${PLIST_LABEL}.plist"
elif [ "$OS" = "Linux" ]; then
  systemctl --user disable --now spider 2>/dev/null || true
  rm -f "$HOME/.config/systemd/user/spider.service"
  systemctl --user daemon-reload 2>/dev/null || true
fi
success "服务已停止"

step "删除二进制"
rm -f "$BIN_DIR/spider" "$BIN_DIR/spdctl"
success "spider / spdctl 已删除"

h1 "卸载完成"
warn "数据目录 ~/.spider 已保留，如需删除："
detail "rm -rf ~/.spider"
