SHELL      := /bin/bash
BIN_DIR    := bin
SPIDER     := $(BIN_DIR)/spider
SPDCTL     := $(BIN_DIR)/spd
VERSION    := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT     := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_TIME := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS    := -s -w \
              -X main.version=$(VERSION) \
              -X main.commit=$(COMMIT) \
              -X main.buildTime=$(BUILD_TIME)
WEB_DIR   := web
WEB_DIST  := cmd/spider/dist
DIST_DIR  := dist
INSTALLER := installer

# ── 颜色 ────────────────────────────────────────────────
BLUE   := \033[34m
GREEN  := \033[32m
YELLOW := \033[33m
RED    := \033[31m
BOLD   := \033[1m
DIM    := \033[2m
RESET  := \033[0m

.PHONY: all build web spider spider-only build-linux build-darwin dist publish install clean tidy help spd

all: build

help:
	@printf "\n$(BOLD)$(BLUE)══ Spider 智能运维平台 ══$(RESET)\n"
	@printf "  $(DIM)make build         $(RESET)编译前端 + spider + spd\n"
	@printf "  $(DIM)make web           $(RESET)仅编译前端 (需要 Node.js)\n"
	@printf "  $(DIM)make spider-only   $(RESET)编译 spider（跳过前端，开发用）\n"
	@printf "  $(DIM)make build-linux   $(RESET)交叉编译 linux/amd64 二进制\n"
	@printf "  $(DIM)make build-darwin  $(RESET)交叉编译 darwin arm64 + amd64 二进制\n"
	@printf "  $(DIM)make dist          $(RESET)打包 macOS zip 安装包到 dist/\n"
	@printf "  $(DIM)make publish       $(RESET)build-linux + 复制到 ~/.spider/bin/\n"
	@printf "  $(DIM)make install       $(RESET)安装到 \$$GOPATH/bin\n"
	@printf "  $(DIM)make clean         $(RESET)删除 bin/ dist/ 和前端产物\n"
	@printf "  $(DIM)make tidy          $(RESET)go mod tidy\n"

build: web spider spd

web:
	@printf "\n$(BOLD)$(BLUE)══ 构建前端 ══$(RESET)\n"
	@printf "  $(BLUE)▶ npm install$(RESET)\n"
	@cd $(WEB_DIR) && npm install
	@printf "  $(BLUE)▶ npm run build$(RESET)\n"
	@cd $(WEB_DIR) && npm run build
	@printf "  $(GREEN)✔ 前端构建完成 → $(WEB_DIST)$(RESET)\n"

spider:
	@mkdir -p $(BIN_DIR)
	@printf "\n$(BOLD)$(BLUE)══ 编译 spider ══$(RESET)\n"
	@printf "  $(BLUE)▶ 版本: $(VERSION)  commit: $(COMMIT)$(RESET)\n"
	@go build -ldflags "$(LDFLAGS)" -o $(SPIDER) ./cmd/spider
	@printf "  $(GREEN)✔ spider → $(SPIDER)$(RESET)\n"

spd:
	@mkdir -p $(BIN_DIR)
	@printf "\n$(BOLD)$(BLUE)══ 编译 spd ══$(RESET)\n"
	@go build -ldflags "$(LDFLAGS)" -o $(SPDCTL) ./cmd/spd
	@printf "  $(GREEN)✔ spd → $(SPDCTL)$(RESET)\n"

build-linux:
	@mkdir -p $(BIN_DIR)
	@printf "\n$(BOLD)$(BLUE)══ 交叉编译 spider → linux/amd64 ══$(RESET)\n"
	@printf "  $(BLUE)▶ 版本: $(VERSION)  commit: $(COMMIT)$(RESET)\n"
	@GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/spider-linux-amd64 ./cmd/spider
	@printf "  $(GREEN)✔ spider-linux-amd64 → $(BIN_DIR)/spider-linux-amd64$(RESET)\n"

build-darwin:
	@mkdir -p $(BIN_DIR)
	@printf "\n$(BOLD)$(BLUE)══ 交叉编译 darwin arm64 + amd64 ══$(RESET)\n"
	@printf "  $(BLUE)▶ 版本: $(VERSION)  commit: $(COMMIT)$(RESET)\n"
	@GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/spider-darwin-arm64 ./cmd/spider
	@GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/spd-darwin-arm64 ./cmd/spd
	@GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/spider-darwin-amd64 ./cmd/spider
	@GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/spd-darwin-amd64 ./cmd/spd
	@printf "  $(GREEN)✔ darwin binaries → $(BIN_DIR)/$(RESET)\n"

dist: web build-darwin
	@printf "\n$(BOLD)$(BLUE)══ 打包 macOS 安装包 ══$(RESET)\n"
	@rm -rf $(DIST_DIR)
	@mkdir -p $(DIST_DIR)
	@mkdir -p $(DIST_DIR)/spider-$(VERSION)-arm64
	@cp $(BIN_DIR)/spider-darwin-arm64   $(DIST_DIR)/spider-$(VERSION)-arm64/spider
	@cp $(BIN_DIR)/spd-darwin-arm64      $(DIST_DIR)/spider-$(VERSION)-arm64/spd
	@cp $(INSTALLER)/install.sh          $(DIST_DIR)/spider-$(VERSION)-arm64/install.sh
	@cp $(INSTALLER)/uninstall.sh        $(DIST_DIR)/spider-$(VERSION)-arm64/uninstall.sh
	@cp $(INSTALLER)/spider.plist        $(DIST_DIR)/spider-$(VERSION)-arm64/spider.plist
	@chmod +x $(DIST_DIR)/spider-$(VERSION)-arm64/install.sh $(DIST_DIR)/spider-$(VERSION)-arm64/uninstall.sh
	@cd $(DIST_DIR) && zip -qr spider-$(VERSION)-arm64.zip spider-$(VERSION)-arm64/
	@rm -rf $(DIST_DIR)/spider-$(VERSION)-arm64
	@mkdir -p $(DIST_DIR)/spider-$(VERSION)-x86_64
	@cp $(BIN_DIR)/spider-darwin-amd64   $(DIST_DIR)/spider-$(VERSION)-x86_64/spider
	@cp $(BIN_DIR)/spd-darwin-amd64      $(DIST_DIR)/spider-$(VERSION)-x86_64/spd
	@cp $(INSTALLER)/install.sh          $(DIST_DIR)/spider-$(VERSION)-x86_64/install.sh
	@cp $(INSTALLER)/uninstall.sh        $(DIST_DIR)/spider-$(VERSION)-x86_64/uninstall.sh
	@cp $(INSTALLER)/spider.plist        $(DIST_DIR)/spider-$(VERSION)-x86_64/spider.plist
	@chmod +x $(DIST_DIR)/spider-$(VERSION)-x86_64/install.sh $(DIST_DIR)/spider-$(VERSION)-x86_64/uninstall.sh
	@cd $(DIST_DIR) && zip -qr spider-$(VERSION)-x86_64.zip spider-$(VERSION)-x86_64/
	@rm -rf $(DIST_DIR)/spider-$(VERSION)-x86_64
	@printf "  $(GREEN)✔ dist/spider-$(VERSION)-arm64.zip$(RESET)\n"
	@printf "  $(GREEN)✔ dist/spider-$(VERSION)-x86_64.zip$(RESET)\n"

publish: build-linux
	@printf "\n$(BOLD)$(BLUE)══ 发布二进制到 DataDir ══$(RESET)\n"
	@mkdir -p ~/.spider/bin
	@cp $(BIN_DIR)/spider-linux-amd64 ~/.spider/bin/spider-linux-amd64
	@printf "  $(GREEN)✔ 已复制到 ~/.spider/bin/spider-linux-amd64$(RESET)\n"

install:
	@printf "\n$(BOLD)$(BLUE)══ 安装 ══$(RESET)\n"
	@go install -ldflags "$(LDFLAGS)" ./cmd/spider ./cmd/spd
	@printf "  $(GREEN)✔ 已安装到 $$GOPATH/bin$(RESET)\n"

clean:
	@printf "  $(YELLOW)⚠ 删除 $(BIN_DIR)/ $(DIST_DIR)/ 和前端产物...$(RESET)\n"
	@rm -rf $(BIN_DIR) $(WEB_DIST) $(DIST_DIR)
	@printf "  $(GREEN)✔ 清理完成$(RESET)\n"

tidy:
	@printf "  $(BLUE)▶ go mod tidy$(RESET)\n"
	@go mod tidy
	@printf "  $(GREEN)✔ 依赖整理完成$(RESET)\n"
