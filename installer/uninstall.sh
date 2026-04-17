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
  detail "sudo ./uninstall.sh"
  exit 1
fi

PLIST_LABEL="ai.fty.spider"
PLIST_PATH="/Library/LaunchDaemons/${PLIST_LABEL}.plist"

h1 "Spider 卸载"

step "停止服务"
launchctl bootout "system/${PLIST_LABEL}" 2>/dev/null || true
success "服务已停止"

step "删除 launchd plist"
rm -f "${PLIST_PATH}"
success "${PLIST_PATH} 已删除"

step "删除二进制"
rm -f /usr/local/bin/spider /usr/local/bin/spd
success "spider / spd 已删除"

h1 "卸载完成"
warn "数据目录 /var/lib/spider 已保留，如需删除："
detail "rm -rf /var/lib/spider"
