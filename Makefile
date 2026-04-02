BIN_DIR   := bin
SPIDER    := $(BIN_DIR)/spider
SPDCTL    := $(BIN_DIR)/spdctl
LDFLAGS   := -s -w

.PHONY: all build spider spdctl install clean tidy

all: build

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
