#!/bin/sh
SPIDER_URL="{{.BaseURL}}"
SKILLS_DIR="$HOME/.claude/skills/spider"

TOKEN=""
while [ $# -gt 0 ]; do
  case $1 in
    --token) TOKEN="$2"; shift 2 ;;
    *) shift ;;
  esac
done

set -e

RED='\033[31m'; GREEN='\033[32m'; YELLOW='\033[33m'
BLUE='\033[34m'; DIM='\033[2m'; RESET='\033[0m'

h1()      { printf "\n${BLUE}══ %s ══${RESET}\n" "$*"; }
step()    { printf "  ${BLUE}▶ %s...${RESET}\n" "$*"; }
success() { printf "  ${GREEN}✔ %s${RESET}\n" "$*"; }
warn()    { printf "  ${YELLOW}⚠ %s${RESET}\n" "$*"; }
error()   { printf "  ${RED}✖ %s${RESET}\n" "$*" >&2; }

h1 "Spider 安装"

step "下载 Skills"
mkdir -p "$SKILLS_DIR"
curl -fsSL "$SPIDER_URL/api/v1/install/skills.tar.gz" | tar -xz -C "$SKILLS_DIR"
success "Skills 已安装到 $SKILLS_DIR"

step "注册 MCP 服务器"
if ! command -v claude >/dev/null 2>&1; then
  error "未找到 claude CLI，请先安装 Claude Code"; exit 1
fi
if [ -z "$TOKEN" ]; then
  warn "未提供 --token，MCP 服务器将以匿名方式注册（可能无法正常使用）"
  claude mcp add --scope global --transport http spider "$SPIDER_URL/mcp"
else
  claude mcp add --scope global --transport http --header "Authorization: Bearer $TOKEN" spider "$SPIDER_URL/mcp"
fi
success "已注册：spider → $SPIDER_URL/mcp"

h1 "安装完成 — 重启 Claude Code 即可使用 Spider"
