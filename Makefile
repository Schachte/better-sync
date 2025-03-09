GO := go

BINARY_NAME := better-sync

BUILD_DIR := build

VERSION := 1.0.0
GIT_COMMIT := $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.gitCommit=$(GIT_COMMIT) -X main.buildDate=$(BUILD_DATE) -w -s"

BUILDFLAGS := -trimpath

$(shell mkdir -p $(BUILD_DIR))

# Define dylib files to copy
DYLIBS := libusb.dylib

.PHONY: all
all: build

.PHONY: copy-dylibs
copy-dylibs:
	@echo "Copying dynamic libraries to build directory..."
	@for lib in $(DYLIBS); do \
		if [ -f $$lib ]; then \
			cp $$lib $(BUILD_DIR)/; \
			echo "  Copied $$lib to $(BUILD_DIR)/"; \
		else \
			echo "  Warning: $$lib not found"; \
		fi \
	done

.PHONY: build
build: copy-dylibs
	$(GO) build $(BUILDFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/better-sync

.PHONY: clean
clean:
	rm -rf $(BUILD_DIR)
	go clean

.PHONY: build-all
build-all: build-linux-amd64 build-linux-arm64 build-darwin-amd64 build-darwin-arm64

.PHONY: build-linux-amd64
build-linux-amd64: copy-dylibs
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 $(GO) build $(BUILDFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/better-sync

.PHONY: build-linux-arm64
build-linux-arm64: copy-dylibs
	CGO_ENABLED=1 GOOS=linux GOARCH=arm64 $(GO) build $(BUILDFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/better-sync

.PHONY: build-darwin-amd64
build-darwin-amd64: copy-dylibs
	CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 $(GO) build $(BUILDFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/better-sync

.PHONY: build-darwin-arm64
build-darwin-arm64: copy-dylibs
	CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 $(GO) build $(BUILDFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/better-sync

.PHONY: deps
deps:
	$(GO) mod tidy
	$(GO) mod download

.PHONY: test
test:
	$(GO) test -v ./...

.PHONY: release
release: build-all
	mkdir -p $(BUILD_DIR)/release
	tar -czf $(BUILD_DIR)/release/$(BINARY_NAME)-$(VERSION)-linux-amd64.tar.gz -C $(BUILD_DIR) $(BINARY_NAME)-linux-amd64 $(DYLIBS)
	tar -czf $(BUILD_DIR)/release/$(BINARY_NAME)-$(VERSION)-linux-arm64.tar.gz -C $(BUILD_DIR) $(BINARY_NAME)-linux-arm64 $(DYLIBS)
	tar -czf $(BUILD_DIR)/release/$(BINARY_NAME)-$(VERSION)-darwin-amd64.tar.gz -C $(BUILD_DIR) $(BINARY_NAME)-darwin-amd64 $(DYLIBS)
	tar -czf $(BUILD_DIR)/release/$(BINARY_NAME)-$(VERSION)-darwin-arm64.tar.gz -C $(BUILD_DIR) $(BINARY_NAME)-darwin-arm64 $(DYLIBS)

.PHONY: dev
dev: copy-dylibs
	DYLD_LIBRARY_PATH=$(BUILD_DIR):$$DYLD_LIBRARY_PATH $(GO) run ./cmd/better-sync \
		$(if $(VERBOSE),--verbose,) \
		$(if $(OP),-op $(OP),) \
		$(if $(SCAN),-scan,) \
		$(if $(TIMEOUT),-timeout $(TIMEOUT),)

.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build              Build for current platform"
	@echo "  clean              Clean build artifacts"
	@echo "  deps               Install dependencies"
	@echo "  test               Run tests"
	@echo "  dev                Run application without building"
	@echo "  build-all          Build for all platforms (Linux/macOS, AMD64/ARM64)"
	@echo "  build-linux-amd64  Build for Linux (AMD64/x86_64)"
	@echo "  build-linux-arm64  Build for Linux (ARM64)"
	@echo "  build-darwin-amd64 Build for macOS (Intel/AMD64)"
	@echo "  build-darwin-arm64 Build for macOS (ARM64/Apple Silicon)"
	@echo "  release            Create release archives for all platforms"
	@echo "  copy-dylibs        Copy dynamic libraries to build directory"

.PHONY: build-cli
build-cli: copy-dylibs
	$(GO) build $(BUILDFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/better-sync

.PHONY: build-all-cli
build-all-cli: build-linux-amd64-cli build-linux-arm64-cli build-darwin-amd64-cli build-darwin-arm64-cli

.PHONY: build-linux-amd64-cli
build-linux-amd64-cli: copy-dylibs
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 $(GO) build $(BUILDFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/better-sync

.PHONY: build-linux-arm64-cli
build-linux-arm64-cli: copy-dylibs
	CGO_ENABLED=1 GOOS=linux GOARCH=arm64 $(GO) build $(BUILDFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/better-sync

.PHONY: build-darwin-amd64-cli
build-darwin-amd64-cli: copy-dylibs
	CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 $(GO) build $(BUILDFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/better-sync

.PHONY: build-darwin-arm64-cli
build-darwin-arm64-cli: copy-dylibs
	CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 $(GO) build $(BUILDFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/better-sync