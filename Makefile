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

.PHONY: all build web spider spider-only build-linux build-darwin dist publish install clean tidy help

all: build

help:
	$(call log_h1,Spider 智能运维平台)
	@printf "  $(DIM)make build         $(RESET)编译前端 + spider + spdctl\n"
	@printf "  $(DIM)make web           $(RESET)仅编译前端 (需要 Node.js)\n"
	@printf "  $(DIM)make spider-only   $(RESET)编译 spider（跳过前端，开发用）\n"
	@printf "  $(DIM)make build-linux   $(RESET)交叉编译 linux/amd64 二进制\n"
	@printf "  $(DIM)make build-darwin  $(RESET)交叉编译 darwin arm64 + amd64 二进制\n"
	@printf "  $(DIM)make dist          $(RESET)打包 macOS zip 安装包到 dist/\n"
	@printf "  $(DIM)make publish       $(RESET)build-linux + 复制到 ~/.spider/bin/\n"
	@printf "  $(DIM)make install       $(RESET)安装到 \$$GOPATH/bin\n"
	@printf "  $(DIM)make clean         $(RESET)删除 bin/ dist/ 和前端产物\n"
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

build-linux:
	@mkdir -p $(BIN_DIR)
	$(call log_h1,交叉编译 spider → linux/amd64)
	$(call log_info,版本: $(VERSION)  commit: $(COMMIT))
	@GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/spider-linux-amd64 ./cmd/spider
	$(call log_ok,spider-linux-amd64 → $(BIN_DIR)/spider-linux-amd64)

build-darwin:
	@mkdir -p $(BIN_DIR)
	$(call log_h1,交叉编译 darwin arm64 + amd64)
	$(call log_info,版本: $(VERSION)  commit: $(COMMIT))
	@GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/spider-darwin-arm64 ./cmd/spider
	@GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/spdctl-darwin-arm64 ./cmd/spdctl
	@GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/spider-darwin-amd64 ./cmd/spider
	@GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/spdctl-darwin-amd64 ./cmd/spdctl
	$(call log_ok,darwin binaries → $(BIN_DIR)/)

dist: web build-darwin
	$(call log_h1,打包 macOS 安装包)
	@mkdir -p $(DIST_DIR)
	@rm -f  $(DIST_DIR)/Spider-$(VERSION)-arm64.zip $(DIST_DIR)/Spider-$(VERSION)-x86_64.zip
	@rm -rf $(DIST_DIR)/Spider-$(VERSION)-arm64 $(DIST_DIR)/Spider-$(VERSION)-x86_64
	@mkdir -p $(DIST_DIR)/Spider-$(VERSION)-arm64
	@cp $(BIN_DIR)/spider-darwin-arm64   $(DIST_DIR)/Spider-$(VERSION)-arm64/spider
	@cp $(BIN_DIR)/spdctl-darwin-arm64   $(DIST_DIR)/Spider-$(VERSION)-arm64/spdctl
	@cp $(INSTALLER)/install.sh          $(DIST_DIR)/Spider-$(VERSION)-arm64/install.sh
	@cp $(INSTALLER)/uninstall.sh        $(DIST_DIR)/Spider-$(VERSION)-arm64/uninstall.sh
	@cp $(INSTALLER)/spider.plist        $(DIST_DIR)/Spider-$(VERSION)-arm64/spider.plist
	@chmod +x $(DIST_DIR)/Spider-$(VERSION)-arm64/install.sh $(DIST_DIR)/Spider-$(VERSION)-arm64/uninstall.sh
	@cd $(DIST_DIR) && zip -qr Spider-$(VERSION)-arm64.zip Spider-$(VERSION)-arm64/
	@rm -rf $(DIST_DIR)/Spider-$(VERSION)-arm64
	@mkdir -p $(DIST_DIR)/Spider-$(VERSION)-x86_64
	@cp $(BIN_DIR)/spider-darwin-amd64   $(DIST_DIR)/Spider-$(VERSION)-x86_64/spider
	@cp $(BIN_DIR)/spdctl-darwin-amd64   $(DIST_DIR)/Spider-$(VERSION)-x86_64/spdctl
	@cp $(INSTALLER)/install.sh          $(DIST_DIR)/Spider-$(VERSION)-x86_64/install.sh
	@cp $(INSTALLER)/uninstall.sh        $(DIST_DIR)/Spider-$(VERSION)-x86_64/uninstall.sh
	@cp $(INSTALLER)/spider.plist        $(DIST_DIR)/Spider-$(VERSION)-x86_64/spider.plist
	@chmod +x $(DIST_DIR)/Spider-$(VERSION)-x86_64/install.sh $(DIST_DIR)/Spider-$(VERSION)-x86_64/uninstall.sh
	@cd $(DIST_DIR) && zip -qr Spider-$(VERSION)-x86_64.zip Spider-$(VERSION)-x86_64/
	@rm -rf $(DIST_DIR)/Spider-$(VERSION)-x86_64
	$(call log_ok,dist/Spider-$(VERSION)-arm64.zip)
	$(call log_ok,dist/Spider-$(VERSION)-x86_64.zip)

publish: build-linux
	$(call log_h1,发布二进制到 DataDir)
	@mkdir -p ~/.spider/bin
	@cp $(BIN_DIR)/spider-linux-amd64 ~/.spider/bin/spider-linux-amd64
	$(call log_ok,已复制到 ~/.spider/bin/spider-linux-amd64)

install:
	$(call log_h1,安装)
	@go install -ldflags "$(LDFLAGS)" ./cmd/spider ./cmd/spdctl
	$(call log_ok,已安装到 $$GOPATH/bin)

clean:
	$(call log_warn,删除 $(BIN_DIR)/ $(DIST_DIR)/ 和前端产物...)
	@rm -rf $(BIN_DIR) $(WEB_DIST) $(DIST_DIR)
	$(call log_ok,清理完成)

tidy:
	$(call log_info,go mod tidy)
	@go mod tidy
	$(call log_ok,依赖整理完成)
