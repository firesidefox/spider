SHELL      := /bin/bash
BIN_DIR    := bin
SPIDER     := $(BIN_DIR)/spider
SPDCTL     := $(BIN_DIR)/spdctl
VERSION    := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT     := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_TIME := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS    := -s -w \
              -X main.version=$(VERSION) \
              -X main.commit=$(COMMIT) \
              -X main.buildTime=$(BUILD_TIME)
WEB_DIR   := web
WEB_DIST  := cmd/spider/web/dist

# ── 颜色 ────────────────────────────────────────────────
BLUE   := \033[34m
GREEN  := \033[32m
YELLOW := \033[33m
RED    := \033[31m
BOLD   := \033[1m
DIM    := \033[2m
RESET  := \033[0m

define log_h1
@printf "\n$(BOLD)$(BLUE)══ %s ══$(RESET)\n" "$(1)"
endef
define log_info
@printf "  $(BLUE)▶ %s$(RESET)\n" "$(1)"
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

.PHONY: all build web spider spider-only spdctl install clean tidy help

all: build

help:
	$(call log_h1,Spider 智能运维平台)
	@printf "  $(DIM)make build         $(RESET)编译前端 + spider + spdctl\n"
	@printf "  $(DIM)make web           $(RESET)仅编译前端 (需要 Node.js)\n"
	@printf "  $(DIM)make spider        $(RESET)编译 spider（含前端，需先 make web）\n"
	@printf "  $(DIM)make spider-only   $(RESET)编译 spider（跳过前端，开发用）\n"
	@printf "  $(DIM)make spdctl        $(RESET)仅编译 CLI 管理工具 (bin/spdctl)\n"
	@printf "  $(DIM)make install       $(RESET)安装到 \$$GOPATH/bin\n"
	@printf "  $(DIM)make clean         $(RESET)删除 bin/ 和前端产物\n"
	@printf "  $(DIM)make tidy          $(RESET)go mod tidy\n"

build: web spider spdctl

web:
	$(call log_h1,构建前端)
	$(call log_info,npm install)
	@cd $(WEB_DIR) && npm install
	$(call log_info,npm run build)
	@cd $(WEB_DIR) && npm run build
	$(call log_ok,前端构建完成 → $(WEB_DIST))

spider:
	@mkdir -p $(BIN_DIR)
	$(call log_h1,编译 spider)
	$(call log_info,版本: $(VERSION)  commit: $(COMMIT))
	@go build -ldflags "$(LDFLAGS)" -o $(SPIDER) ./cmd/spider
	$(call log_ok,spider → $(SPIDER))

spider-only:
	@mkdir -p $(BIN_DIR)
	$(call log_h1,编译 spider（跳过前端）)
	$(call log_info,版本: $(VERSION)  commit: $(COMMIT))
	@go build -ldflags "$(LDFLAGS)" -o $(SPIDER) ./cmd/spider
	$(call log_ok,spider → $(SPIDER))

spdctl:
	@mkdir -p $(BIN_DIR)
	$(call log_h1,编译 spdctl)
	@go build -ldflags "$(LDFLAGS)" -o $(SPDCTL) ./cmd/spdctl
	$(call log_ok,spdctl → $(SPDCTL))

install:
	$(call log_h1,安装)
	@go install -ldflags "$(LDFLAGS)" ./cmd/spider ./cmd/spdctl
	$(call log_ok,已安装到 $$GOPATH/bin)

clean:
	$(call log_warn,删除 $(BIN_DIR)/ 和前端产物...)
	@rm -rf $(BIN_DIR) $(WEB_DIST)
	$(call log_ok,清理完成)

tidy:
	$(call log_info,go mod tidy)
	@go mod tidy
	$(call log_ok,依赖整理完成)
