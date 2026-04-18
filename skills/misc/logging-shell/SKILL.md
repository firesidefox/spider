---
name: shell-logging
description: Use when adding colored output to shell scripts, sourcing a logging library, or structuring log functions with tput or ANSI codes
---

# Shell Logging

## Overview

Shell 脚本彩色日志两种实现：`tput`（终端自适应）或 ANSI 转义码（简单直接）。
推荐封装为独立 `.sh` 库，其他脚本 `source` 引入。

可读性增强原则：
- 用颜色区分日志级别（信息/成功/警告/错误）
- 用分隔线和标题划分脚本阶段
- 用进度提示标记长耗时步骤的开始和结束
- 错误输出到 stderr，正常输出到 stdout

## 方案对比

| 方案 | 依赖 | 非终端安全 | 适用场景 |
|------|------|-----------|---------|
| `tput` | terminfo | 是（`set +e`） | 正式项目，CI 兼容 |
| ANSI 转义码 | 无 | 否（颜色码裸输出） | 简单脚本 |

## 标准库结构（tput 方案）

```bash
#!/bin/bash
set +e  # tput 在非终端环境会报错，忽略

bold=$(tput bold)
reset=$(tput sgr0)
red=$(tput setaf 1)
green=$(tput setaf 76)   # 256色索引，亮绿
yellow=$(tput setaf 202) # 橙色
blue=$(tput setaf 25)
white=$(tput setaf 7)
dim=$(tput dim 2>/dev/null || true)

# 阶段标题
h1()      { printf "\n${bold}${blue}══ %s ══${reset}\n" "$@"; }
h2()      { printf "\n${bold}${white}── %s ──${reset}\n" "$@"; }

# 日志级别
info()    { printf "${white}  ➜ %s${reset}\n" "$@"; }
success() { printf "${green}  ✔ %s${reset}\n" "$@"; }
warn()    { printf "${yellow}  ⚠ %s${reset}\n" "$@"; }
error()   { printf "${red}  ✖ %s${reset}\n" "$@" >&2; }
debug()   { printf "${dim}  · %s${reset}\n" "$@"; }

# 步骤进度（长耗时操作前后调用）
step()    { printf "${blue}  ▶ %s...${reset}\n" "$@"; }
done_()   { printf "${green}  ✔ done${reset}\n"; }

# 分隔线
divider() { printf "${dim}%s${reset}\n" "────────────────────────────────"; }
```

## 可读性模式：阶段 + 步骤

```bash
source .logging.sh

h1 "部署流程"

h2 "构建"
step "编译前端"
npm run build
success "前端构建完成"

divider

h2 "上传"
step "上传到 $HOST"
scp bin/spider user@$HOST:/tmp/
success "上传完成 → /tmp/spider"

divider

h1 "部署完成"
```

输出效果：
```
══ 部署流程 ══

── 构建 ──
  ▶ 编译前端...
  ✔ 前端构建完成
────────────────────────────────

── 上传 ──
  ▶ 上传到 192.168.1.1...
  ✔ 上传完成 → /tmp/spider
────────────────────────────────

══ 部署完成 ══
```

## ANSI 方案（无依赖）

```bash
RED='\033[31m'; GREEN='\033[32m'; YELLOW='\033[33m'
BLUE='\033[34m'; DIM='\033[2m'; RESET='\033[0m'

h1()      { printf "\n${BLUE}══ %s ══${RESET}\n" "$1"; }
info()    { printf "  ${BLUE}➜ %s${RESET}\n" "$1"; }
success() { printf "  ${GREEN}✔ %s${RESET}\n" "$1"; }
warn()    { printf "  ${YELLOW}⚠ %s${RESET}\n" "$1"; }
error()   { printf "  ${RED}✖ %s${RESET}\n" "$1" >&2; }
step()    { printf "  ${BLUE}▶ %s...${RESET}\n" "$1"; }
```

用 `printf` 不用 `echo`：macOS `echo` 默认不解析 `\033` 转义。

## 命名约定

| 文件名 | 场景 |
|--------|------|
| `.logging.sh` | 项目根目录，隐藏文件 |
| `logging.sh` | `scripts/` 子目录 |
| `colors.sh` | 仅颜色变量，无函数 |

## 常见错误

- `tput` 在 CI/非 TTY 环境输出错误 → 加 `set +e` 或检测 `[ -t 1 ]`
- `source` 用相对路径 → 从不同目录调用时失效，改用 `source "$(dirname "$0")/.logging.sh"`
- 函数名与系统命令冲突（如 `error`）→ 加前缀如 `log_error`
- 错误信息输出到 stdout → 应输出到 stderr（`>&2`），方便管道过滤
