BIN_DIR   := bin
SPIDER    := $(BIN_DIR)/spider
SPDCTL    := $(BIN_DIR)/spdctl
LDFLAGS   := -s -w

.PHONY: all build spider spdctl install clean tidy help

all: build

help:
	@echo "Spider 智能运维平台"
	@echo ""
	@echo "用法:"
	@echo "  make build       编译 spider 和 spdctl 到 bin/"
	@echo "  make spider      仅编译 MCP server (bin/spider)"
	@echo "  make spdctl      仅编译 CLI 管理工具 (bin/spdctl)"
	@echo "  make install     安装到 \$$GOPATH/bin"
	@echo "  make clean       删除 bin/"
	@echo "  make tidy        go mod tidy"
	@echo ""
	@echo "快速开始:"
	@echo "  bin/spdctl host add --name web01 --ip 10.0.0.1 --user root --auth key --key ~/.ssh/id_rsa"
	@echo "  bin/spdctl host list"
	@echo "  bin/spdctl ping web01"
	@echo "  bin/spdctl exec web01 'df -h'"
	@echo "  bin/spdctl history"
	@echo ""
	@echo "注册 MCP server 到 Claude Code:"
	@echo "  claude mcp add spider \$$(pwd)/bin/spider"

build: spider spdctl

spider:
	@mkdir -p $(BIN_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(SPIDER) ./cmd/spider

spdctl:
	@mkdir -p $(BIN_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(SPDCTL) ./cmd/spdctl

install:
	go install -ldflags "$(LDFLAGS)" ./cmd/spider ./cmd/spdctl

clean:
	rm -rf $(BIN_DIR)

tidy:
	go mod tidy
