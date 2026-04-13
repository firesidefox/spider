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

.PHONY: all build web spider spider-only spdctl install clean tidy help

all: build

help:
	@echo "Spider 智能运维平台"
	@echo ""
	@echo "用法:"
	@echo "  make build         编译前端 + spider + spdctl"
	@echo "  make web           仅编译前端 (需要 Node.js)"
	@echo "  make spider        编译 spider（含前端，需先 make web）"
	@echo "  make spider-only   编译 spider（跳过前端，开发用）"
	@echo "  make spdctl        仅编译 CLI 管理工具 (bin/spdctl)"
	@echo "  make install       安装到 \$$GOPATH/bin"
	@echo "  make clean         删除 bin/ 和前端产物"
	@echo "  make tidy          go mod tidy"
	@echo ""
	@echo "开发模式:"
	@echo "  make spider-only   # 启动后端"
	@echo "  cd web && npm run dev  # 启动前端开发服务器（代理到 :8000）"

build: web spider spdctl

web:
	cd $(WEB_DIR) && npm install && npm run build

spider:
	@mkdir -p $(BIN_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(SPIDER) ./cmd/spider

spider-only:
	@mkdir -p $(BIN_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(SPIDER) ./cmd/spider

spdctl:
	@mkdir -p $(BIN_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(SPDCTL) ./cmd/spdctl

install:
	go install -ldflags "$(LDFLAGS)" ./cmd/spider ./cmd/spdctl

clean:
	rm -rf $(BIN_DIR) $(WEB_DIST)

tidy:
	go mod tidy
