---
name: makefile-logging
description: Use when adding colored output to a Makefile — using ANSI escape code variables
---

# Makefile Logging

## Overview

Makefile 每行命令在独立子 shell 执行，无法跨行保留 shell 函数。彩色输出使用 ANSI 变量方案。

可读性增强原则：
- 用颜色区分阶段标题、进度、成功、警告、错误
- 用 `step` + `ok` 包裹每个耗时操作，标记开始和结束
- 用分隔线划分 target 内的逻辑阶段

## ANSI 变量方案

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

## 常见错误

- `\033` 在某些 shell 需要 `printf` 而非 `echo`（macOS `echo` 不解析转义）
- 错误信息忘记重定向到 stderr → 加 `>&2` 方便管道过滤
