# Userspace Installer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 install.sh / uninstall.sh 改为用户空间安装，无需 sudo，支持 macOS (LaunchAgents) 和 Linux (systemd user)。

**Architecture:** 三个文件改动：`spider.plist` 改为含 `__HOME__` 占位符的模板；`install.sh` 动态替换占位符、按 OS 分支执行 launchctl/systemctl；`uninstall.sh` 同步路径和命令。

**Tech Stack:** bash, launchctl (macOS), systemctl/loginctl (Linux)

---

### Task 1: 更新 spider.plist 为模板

**Files:**
- Modify: `installer/spider.plist`

- [ ] **Step 1: 替换硬编码路径为占位符**

将 `installer/spider.plist` 改为：

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>ai.fty.spider</string>
  <key>ProgramArguments</key>
  <array>
    <string>__HOME__/.local/bin/spider</string>
    <string>serve</string>
    <string>--data-dir</string>
    <string>__HOME__/.spider/data</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>StandardOutPath</key>
  <string>__HOME__/.spider/logs/spider.log</string>
  <key>StandardErrorPath</key>
  <string>__HOME__/.spider/logs/spider.log</string>
</dict>
</plist>
```

- [ ] **Step 2: 验证 plist 格式**

```bash
plutil -lint installer/spider.plist
```

Expected: `installer/spider.plist: OK`

- [ ] **Step 3: Commit**

```bash
git add installer/spider.plist
git commit -m "feat(installer): convert plist to template with __HOME__ placeholder"
```

---

### Task 2: 重写 install.sh

**Files:**
- Modify: `installer/install.sh`

- [ ] **Step 1: 替换路径变量和 root 检查（第 20-33 行）**

将文件头部（`SCRIPT_DIR` 定义之后到 root 检查结束）替换为：

```bash
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PLIST_LABEL="ai.fty.spider"
BIN_DIR="$HOME/.local/bin"
DATA_DIR="$HOME/.spider/data"
LOG_DIR="$HOME/.spider/logs"
OS="$(uname -s)"

h1 "Spider 安装"

if [ "$(id -u)" -eq 0 ]; then
  error "请勿以 root 用户运行此脚本，直接运行 ./install.sh 即可。"
  exit 1
fi
```

- [ ] **Step 2: 替换"停止旧版本服务"步骤（第 37-44 行）**

```bash
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
```

- [ ] **Step 3: 替换"安装二进制"步骤（第 46-49 行）**

```bash
step "安装二进制"
mkdir -p "$BIN_DIR"
install -m 755 "${SCRIPT_DIR}/spider" "$BIN_DIR/spider"
install -m 755 "${SCRIPT_DIR}/spdctl" "$BIN_DIR/spdctl"
success "spider / spdctl → $BIN_DIR/"
```

- [ ] **Step 4: 替换"创建日志目录"和"创建数据目录"步骤（第 51-59 行）**

```bash
step "创建日志目录"
mkdir -p "$LOG_DIR"
success "$LOG_DIR 已就绪"

step "创建数据目录"
mkdir -p "$DATA_DIR"
success "$DATA_DIR 已就绪"
```

- [ ] **Step 5: 替换"安装内置 Skills"步骤（第 61-67 行）**

```bash
step "安装内置 Skills"
if [ -d "${SCRIPT_DIR}/skills" ]; then
  cp -r "${SCRIPT_DIR}/skills/." "$DATA_DIR/skills/"
  success "Skills → $DATA_DIR/skills/"
else
  warn "未找到 skills 目录，跳过"
fi
```

- [ ] **Step 6: 替换"安装 launchd plist"步骤（第 69-71 行）**

```bash
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
```

- [ ] **Step 7: 替换"启动服务"步骤（第 87-94 行）**

```bash
step "启动服务"
if [ "$OS" = "Darwin" ]; then
  if ! launchctl bootstrap "gui/$(id -u)" "$PLIST_DST" 2>/tmp/spider-bootstrap.err; then
    error "launchctl bootstrap 失败"
    cat /tmp/spider-bootstrap.err >&2
    detail "查看日志：tail -f $LOG_DIR/spider.log"
    detail "手动启动：$BIN_DIR/spider"
    exit 1
  fi
elif [ "$OS" = "Linux" ]; then
  if ! systemctl --user enable --now spider 2>/tmp/spider-bootstrap.err; then
    error "systemctl enable 失败"
    cat /tmp/spider-bootstrap.err >&2
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
```

- [ ] **Step 8: 替换"验证服务"和末尾提示（第 96-117 行）**

```bash
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
```

- [ ] **Step 9: 验证脚本语法**

```bash
bash -n installer/install.sh
```

Expected: 无输出（无语法错误）

- [ ] **Step 10: Commit**

```bash
git add installer/install.sh
git commit -m "feat(installer): rewrite install.sh for userspace install, no sudo required"
```

---

### Task 3: 重写 uninstall.sh

**Files:**
- Modify: `installer/uninstall.sh`

- [ ] **Step 1: 替换整个文件**

```bash
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
```

- [ ] **Step 2: 验证脚本语法**

```bash
bash -n installer/uninstall.sh
```

Expected: 无输出

- [ ] **Step 3: Commit**

```bash
git add installer/uninstall.sh
git commit -m "feat(installer): rewrite uninstall.sh for userspace, no sudo required"
```
