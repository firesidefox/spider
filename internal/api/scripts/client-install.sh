#!/bin/sh
SPIDER_URL="{{.BaseURL}}"
SKILLS_DIR="$HOME/.claude/skills"

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
claude mcp remove spider --scope user 2>/dev/null || true
if [ -z "$TOKEN" ]; then
  warn "未提供 --token，MCP 服务器将以匿名方式注册（可能无法正常使用）"
  claude mcp add --scope user --transport http spider "$SPIDER_URL/mcp"
else
  claude mcp add --scope user --transport http spider "$SPIDER_URL/mcp" --header "Authorization: Bearer $TOKEN"
fi
success "已注册：spider → $SPIDER_URL/mcp"

step "注册 Claude Code Plugin 和 Marketplace"
CLAUDE_BIN="$(command -v claude 2>/dev/null || true)"
if [ -z "$CLAUDE_BIN" ]; then
  warn "未找到 claude 命令，跳过 plugin/marketplace 注册"
  warn "安装完成后手动运行："
  warn "  claude plugin marketplace add spider-skills ${SKILLS_DIR}/spider"
  warn "  claude plugin marketplace add misc-skills ${SKILLS_DIR}/misc"
else
  # 注册并安装每个 plugin
  install_plugin() {
    name="$1"; dir="$2"; pkg="$3"
    if "$CLAUDE_BIN" plugin marketplace list 2>/dev/null | grep -q "$name"; then
      success "Marketplace $name 已存在，跳过"
    else
      MKT_ERR="$(mktemp)"
      if "$CLAUDE_BIN" plugin marketplace add "$dir" 2>"$MKT_ERR"; then
        success "Marketplace $name 已注册"
      else
        warn "Marketplace $name 注册失败：$(cat "$MKT_ERR")"
        rm -f "$MKT_ERR"
        return
      fi
      rm -f "$MKT_ERR"
    fi
    if "$CLAUDE_BIN" plugin list 2>/dev/null | grep -q "^${pkg%%@*}"; then
      success "Plugin $pkg 已安装，跳过"
    else
      PLUGIN_ERR="$(mktemp)"
      if "$CLAUDE_BIN" plugin install "$pkg" 2>"$PLUGIN_ERR"; then
        success "Plugin $pkg 已安装"
      else
        warn "Plugin $pkg 安装失败：$(cat "$PLUGIN_ERR")"
      fi
      rm -f "$PLUGIN_ERR"
    fi
  }

  install_plugin "spider-skills"  "${SKILLS_DIR}/spider" "spider@spider-skills"
  install_plugin "misc-skills"    "${SKILLS_DIR}/misc"   "misc@misc-skills"
fi

h1 "安装完成 — 重启 Claude Code 即可使用 Spider"
