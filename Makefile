GO := go

BINARY_NAME := better-sync

BUILD_DIR := build

VERSION := 1.0.0
GIT_COMMIT := $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.gitCommit=$(GIT_COMMIT) -X main.buildDate=$(BUILD_DATE) -w -s"

BUILDFLAGS := -trimpath

# Detect architecture
UNAME_MACHINE := $(shell uname -m)
ifeq ($(UNAME_MACHINE),arm64)
    # Apple Silicon
    GOARCH := arm64
    LIBUSB_PATH := /opt/homebrew/Cellar/libusb/1.0.27/lib
    LIBUSB_INCLUDE := /opt/homebrew/Cellar/libusb/1.0.27/include
else
    # Intel Mac
    GOARCH := amd64
    LIBUSB_PATH := /usr/local/opt/libusb/lib
    LIBUSB_INCLUDE := /usr/local/opt/libusb/include
endif

$(shell mkdir -p $(BUILD_DIR))

# Define dylib files to copy
DYLIBS := libusb-1.0.0.dylib

.PHONY: all
all: build

.PHONY: copy-dylibs
copy-dylibs:
	@echo "Copying dynamic libraries to build directory..."
	@mkdir -p $(BUILD_DIR)
	@if [ "$(GOARCH)" = "arm64" ]; then \
		if [ -f "$(LIBUSB_PATH)/libusb-1.0.0.dylib" ]; then \
			cp -f "$(LIBUSB_PATH)/libusb-1.0.0.dylib" $(BUILD_DIR)/libusb.dylib; \
			chmod 755 $(BUILD_DIR)/libusb.dylib; \
			echo "  Copied libusb-1.0.0.dylib (arm64) to $(BUILD_DIR)/libusb.dylib"; \
		else \
			echo "  Warning: libusb-1.0.0.dylib not found at $(LIBUSB_PATH)"; \
		fi; \
	else \
		if [ -f "$(LIBUSB_PATH)/libusb-1.0.0.dylib" ]; then \
			cp -f "$(LIBUSB_PATH)/libusb-1.0.0.dylib" $(BUILD_DIR)/libusb.dylib; \
			chmod 755 $(BUILD_DIR)/libusb.dylib; \
			echo "  Copied libusb-1.0.0.dylib (amd64) to $(BUILD_DIR)/libusb.dylib"; \
		else \
			echo "  Warning: libusb-1.0.0.dylib not found at $(LIBUSB_PATH)"; \
		fi; \
	fi

.PHONY: fix-library-paths
fix-library-paths:
	@echo "Fixing library paths in executables..."
	@if [ -f "$(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64" ]; then \
		install_name_tool -change /opt/homebrew/Cellar/libusb/1.0.27/lib/libusb-1.0.0.dylib @executable_path/libusb.dylib $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 || true; \
		echo "  Fixed paths in $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64"; \
	fi
	@if [ -f "$(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64" ]; then \
		install_name_tool -change /usr/local/opt/libusb/lib/libusb-1.0.0.dylib @executable_path/libusb.dylib $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 || true; \
		echo "  Fixed paths in $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64"; \
	fi
	@if [ -f "$(BUILD_DIR)/$(BINARY_NAME)" ]; then \
		if [ "$(GOARCH)" = "arm64" ]; then \
			install_name_tool -change /opt/homebrew/Cellar/libusb/1.0.27/lib/libusb-1.0.0.dylib @executable_path/libusb.dylib $(BUILD_DIR)/$(BINARY_NAME) || true; \
		else \
			install_name_tool -change /usr/local/opt/libusb/lib/libusb-1.0.0.dylib @executable_path/libusb.dylib $(BUILD_DIR)/$(BINARY_NAME) || true; \
		fi; \
		echo "  Fixed paths in $(BUILD_DIR)/$(BINARY_NAME)"; \
	fi

.PHONY: sign-binaries
sign-binaries:
	@echo "Code signing binaries..."
	@if [ -f "$(BUILD_DIR)/libusb.dylib" ]; then \
		codesign --force --sign - $(BUILD_DIR)/libusb.dylib; \
		echo "  Signed $(BUILD_DIR)/libusb.dylib"; \
	fi
	@if [ -f "$(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64" ]; then \
		codesign --force --sign - $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64; \
		echo "  Signed $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64"; \
	fi
	@if [ -f "$(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64" ]; then \
		codesign --force --sign - $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64; \
		echo "  Signed $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64"; \
	fi
	@if [ -f "$(BUILD_DIR)/$(BINARY_NAME)" ]; then \
		codesign --force --sign - $(BUILD_DIR)/$(BINARY_NAME); \
		echo "  Signed $(BUILD_DIR)/$(BINARY_NAME)"; \
	fi

.PHONY: build
build: copy-dylibs
	CGO_ENABLED=1 GOARCH=$(GOARCH) CGO_LDFLAGS="-L$(LIBUSB_PATH) -lusb-1.0" CGO_CFLAGS="-I$(LIBUSB_INCLUDE)" $(GO) build $(BUILDFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/better-sync
	$(MAKE) fix-library-paths
	$(MAKE) sign-binaries

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
	CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 CGO_LDFLAGS="-L/usr/local/opt/libusb/lib -lusb-1.0" CGO_CFLAGS="-I/usr/local/opt/libusb/include" $(GO) build $(BUILDFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/better-sync
	$(MAKE) fix-library-paths
	$(MAKE) sign-binaries

.PHONY: build-darwin-arm64
build-darwin-arm64: copy-dylibs
	CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 CGO_LDFLAGS="-L$(LIBUSB_PATH) -lusb-1.0" CGO_CFLAGS="-I$(LIBUSB_INCLUDE)" $(GO) build $(BUILDFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/better-sync
	$(MAKE) fix-library-paths
	$(MAKE) sign-binaries

.PHONY: deps
deps:
	$(GO) mod tidy
	$(GO) mod download

.PHONY: test
test:
	$(GO) test -v ./...

.PHONY: dev
dev:
	CGO_ENABLED=1 GOARCH=$(GOARCH) CGO_LDFLAGS="-L$(LIBUSB_PATH) -lusb-1.0" CGO_CFLAGS="-I$(LIBUSB_INCLUDE)" $(GO) run ./cmd/better-sync

.PHONY: release
release: build-all
	mkdir -p $(BUILD_DIR)/release
	tar -czf $(BUILD_DIR)/release/$(BINARY_NAME)-$(VERSION)-linux-amd64.tar.gz -C $(BUILD_DIR) $(BINARY_NAME)-linux-amd64 libusb.dylib
	tar -czf $(BUILD_DIR)/release/$(BINARY_NAME)-$(VERSION)-linux-arm64.tar.gz -C $(BUILD_DIR) $(BINARY_NAME)-linux-arm64 libusb.dylib
	tar -czf $(BUILD_DIR)/release/$(BINARY_NAME)-$(VERSION)-darwin-amd64.tar.gz -C $(BUILD_DIR) $(BINARY_NAME)-darwin-amd64 libusb.dylib
	tar -czf $(BUILD_DIR)/release/$(BINARY_NAME)-$(VERSION)-darwin-arm64.tar.gz -C $(BUILD_DIR) $(BINARY_NAME)-darwin-arm64 libusb.dylib

\
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
	@echo "  fix-library-paths  Fix library paths in executables"
	@echo "  sign-binaries      Code sign the binaries for macOS"
	@echo "  sign-wails-app     Code sign the Wails app bundle"

.PHONY: build-cli
build-cli: copy-dylibs
	GOARCH=arm64 CGO_ENABLED=1 CGO_LDFLAGS="-L$(LIBUSB_PATH) -lusb-1.0" CGO_CFLAGS="-I$(LIBUSB_INCLUDE)" $(GO) build $(BUILDFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/better-sync
	$(MAKE) fix-library-paths
	$(MAKE) sign-binaries

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
	CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 CGO_LDFLAGS="-L/usr/local/opt/libusb/lib -lusb-1.0" CGO_CFLAGS="-I/usr/local/opt/libusb/include" $(GO) build $(BUILDFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/better-sync
	$(MAKE) fix-library-paths
	$(MAKE) sign-binaries

.PHONY: build-darwin-arm64-cli
build-darwin-arm64-cli: copy-dylibs
	CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 CGO_LDFLAGS="-L$(LIBUSB_PATH) -lusb-1.0" CGO_CFLAGS="-I$(LIBUSB_INCLUDE)" $(GO) build $(BUILDFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/better-sync
	$(MAKE) fix-library-paths
	$(MAKE) sign-binaries

# Add a new target specifically for signing the Wails app
.PHONY: sign-wails-app
sign-wails-app:
	@echo "Code signing Wails app..."
	@if [ -d "app/build/bin" ]; then \
		find app/build/bin -type f -name "*.app" -o -name "*.framework" -o -name "*.dylib" -o -perm +111 -type f | while read file; do \
			codesign --force --sign - "$$file"; \
			echo "  Signed $$file"; \
		done; \
	fi
	@if [ -d "app/build/bin" ]; then \
		find app/build/bin -name "*.app" -type d | while read appbundle; do \
			codesign --force --deep --sign - "$$appbundle"; \
			echo "  Deep signed app bundle $$appbundle"; \
		done; \
	fi

# Wails development target
app-dev: copy-dylibs
	CGO_ENABLED=1 GOARCH=$(GOARCH) CGO_LDFLAGS="-L$(LIBUSB_PATH) -lusb-1.0" CGO_CFLAGS="-I$(LIBUSB_INCLUDE)" $(GO) build $(BUILDFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/better-sync
	$(MAKE) fix-library-paths
	$(MAKE) sign-binaries
	cd app && GOOS=darwin GOARCH=$(GOARCH) CGO_ENABLED=1 CGO_LDFLAGS="-L$(LIBUSB_PATH) -lusb-1.0" CGO_CFLAGS="-I$(LIBUSB_INCLUDE)" DYLD_LIBRARY_PATH=../$(BUILD_DIR) wails dev

# Run Wails with dylib path set and sign the result
app-build: copy-dylibs
	CGO_ENABLED=1 GOARCH=$(GOARCH) CGO_LDFLAGS="-L$(LIBUSB_PATH) -lusb-1.0" CGO_CFLAGS="-I$(LIBUSB_INCLUDE)" $(GO) build $(BUILDFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/better-sync
	$(MAKE) fix-library-paths
	$(MAKE) sign-binaries
	cd app && GOOS=darwin GOARCH=$(GOARCH) CGO_ENABLED=1 CGO_LDFLAGS="-L$(LIBUSB_PATH) -lusb-1.0" CGO_CFLAGS="-I$(LIBUSB_INCLUDE)" DYLD_LIBRARY_PATH=../$(BUILD_DIR) wails build
	$(MAKE) sign-wails-app