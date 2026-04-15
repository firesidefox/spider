---
name: makefile-logging
description: Use when adding colored output to a Makefile — choosing between ANSI variables, call macros, include .mk files, or sourcing .sh logging libraries
---

# Makefile Logging

## Overview

Makefile 每行命令在独立子 shell 执行，无法跨行保留 shell 函数。彩色输出有四种方案，按复用性和依赖选择。

可读性增强原则：
- 用颜色区分阶段标题、进度、成功、警告、错误
- 用 `step` + `ok` 包裹每个耗时操作，标记开始和结束
- 用分隔线划分 target 内的逻辑阶段

## 方案对比

| 方案 | 依赖 | 复用性 | 适用场景 |
|------|------|--------|---------|
| ANSI 变量 | 无 | 中 | 简单项目，少量 target |
| `call` 宏 | 无 | 高 | 多 target，纯 Makefile |
| `include .mk` | 无 | 高 | 团队共享，跨项目复用 |
| `source .sh` + `\` 续行 | bash | 高 | 已有 shell 日志库 |

## 方案 1：ANSI 变量（零依赖）

```makefile
RED    := \033[31m
GREEN  := \033[32m
YELLOW := \033[33m
BLUE   := \033[34m
DIM    := \033[2m
BOLD   := \033[1m
RESET  := \033[0m
DIV    := $(DIM)────────────────────────────────$(RESET)

web:
	@printf "$(BOLD)$(BLUE)══ 构建前端 ══$(RESET)\n"
	@printf "  $(BLUE)▶ npm install...$(RESET)\n"
	@cd $(WEB_DIR) && npm install
	@printf "  $(BLUE)▶ npm run build...$(RESET)\n"
	@cd $(WEB_DIR) && npm run build
	@printf "  $(GREEN)✔ 前端构建完成$(RESET)\n"
	@printf "$(DIV)\n"
```

## 方案 2：`call` 宏（推荐，纯 Makefile）

```makefile
define log_h1
@printf "\n$(BOLD)$(BLUE)══ %s ══$(RESET)\n" "$(1)"
endef
define log_step
@printf "  $(BLUE)▶ %s...$(RESET)\n" "$(1)"
endef
define log_ok
@printf "  $(GREEN)✔ %s$(RESET)\n" "$(1)"
endef
define log_warn
@printf "  $(YELLOW)⚠ %s$(RESET)\n" "$(1)"
endef
define log_err
@printf "  $(RED)✖ %s$(RESET)\n" "$(1)" >&2
endef
define log_div
@printf "$(DIM)────────────────────────────────$(RESET)\n"
endef

web:
	$(call log_h1,构建前端)
	$(call log_step,npm install)
	@cd $(WEB_DIR) && npm install
	$(call log_step,npm run build)
	@cd $(WEB_DIR) && npm run build
	$(call log_ok,前端构建完成)
	$(call log_div)
```

## 方案 3：`include` 子文件（跨项目复用）

命名约定：`logging.mk`（与 `.logging.sh` 对应）

```makefile
# logging.mk
RED    := \033[31m
GREEN  := \033[32m
YELLOW := \033[33m
BLUE   := \033[34m
DIM    := \033[2m
BOLD   := \033[1m
RESET  := \033[0m

define log_h1
@printf "\n$(BOLD)$(BLUE)══ %s ══$(RESET)\n" "$(1)"
endef
define log_step
@printf "  $(BLUE)▶ %s...$(RESET)\n" "$(1)"
endef
define log_ok
@printf "  $(GREEN)✔ %s$(RESET)\n" "$(1)"
endef
define log_err
@printf "  $(RED)✖ %s$(RESET)\n" "$(1)" >&2
endef
define log_div
@printf "$(DIM)────────────────────────────────$(RESET)\n"
endef
```

```makefile
# Makefile
include logging.mk

web:
	$(call log_h1,构建前端)
	$(call log_step,npm run build)
	@cd $(WEB_DIR) && npm run build
	$(call log_ok,前端构建完成)
```

`include` 是原生 Make 特性，GNU Make 3.81+ 支持。

## 方案 4：`source .sh` + `\` 续行

已有 `.logging.sh` 时，用 `\` 把多行合并为一个 shell 进程：

```makefile
SHELL := /bin/bash

web:
	@source .logging.sh && h1 "构建前端" && \
	  step "npm run build" && \
	  cd $(WEB_DIR) && npm run build && \
	  success "前端构建完成"
```

注意：`.ONESHELL` 需要 GNU Make 3.82+，3.81 不支持。

## 常见错误

- `source` 后换行 → 函数丢失（新子 shell）。必须用 `\` 续行或每行重新 source
- `$(call log_ok,文字)` 里的逗号被 Make 解析为参数分隔符 → 用变量传递含逗号的字符串
- `\033` 在某些 shell 需要 `printf` 而非 `echo`（macOS `echo` 不解析转义）
- 错误信息忘记重定向到 stderr → 加 `>&2` 方便管道过滤
