BINARY ?= doraemon
CMD ?= ./cmd/doraemon
OUT_DIR ?= dist
GO ?= go
GOFLAGS ?= -mod=mod
GOCACHE ?= $(CURDIR)/.cache/go-build
GOMODCACHE ?= $(CURDIR)/.cache/go-mod
GOENV = GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE)

.PHONY: build linux linux-amd64 linux-arm64 linux-armv7 clean

build:
	$(GOENV) $(GO) build $(GOFLAGS) -o $(BINARY) $(CMD)

linux: linux-amd64 linux-arm64 linux-armv7

linux-amd64:
	mkdir -p $(OUT_DIR)
	$(GOENV) CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(OUT_DIR)/$(BINARY)-linux-amd64 $(CMD)

linux-arm64:
	mkdir -p $(OUT_DIR)
	$(GOENV) CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build $(GOFLAGS) -o $(OUT_DIR)/$(BINARY)-linux-arm64 $(CMD)

clean:
	rm -rf $(OUT_DIR) $(BINARY)
