# Makefile for building LanceDB Go CGO bindings
export BUILDKIT_PROGRESS ?= plain

.PHONY: all build clean test example generate-pc

# Determine current platform
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
PLATFORM := $(GOOS)-$(GOARCH)

# Library path for local development
LOCAL_LIB_PATH := $(CURDIR)/libs/$(PLATFORM)
# PkgConfig path for local development
LOCAL_PKG_CONFIG_PATH := $(CURDIR)/libs/pkgconfig

# Default target
all: build

# Build the Rust CGO library
build-rust:
	cd rust-cgo && cargo build --release

# Generate local pkg-config file
generate-pc:
	@mkdir -p $(LOCAL_PKG_CONFIG_PATH)
	@echo "Generating lancedb.pc for $(PLATFORM)..."
	@echo "Name: lancedb" > $(LOCAL_PKG_CONFIG_PATH)/lancedb.pc
	@echo "Description: LanceDB Static Library" >> $(LOCAL_PKG_CONFIG_PATH)/lancedb.pc
	@echo "Version: 0.1.0" >> $(LOCAL_PKG_CONFIG_PATH)/lancedb.pc
	@if [ "$(GOOS)" = "darwin" ]; then \
		echo "Libs: -L$(LOCAL_LIB_PATH) -llancedb_cgo -lm -ldl -lresolv -framework CoreFoundation -framework Security -framework SystemConfiguration" >> $(LOCAL_PKG_CONFIG_PATH)/lancedb.pc; \
	else \
		echo "Libs: -L$(LOCAL_LIB_PATH) -llancedb_cgo -lm -ldl -lpthread" >> $(LOCAL_PKG_CONFIG_PATH)/lancedb.pc; \
	fi
	@echo "Cflags: " >> $(LOCAL_PKG_CONFIG_PATH)/lancedb.pc

# Build the Go package (also builds Rust library via cgo)
build-go: generate-pc
	# Ensure library exists for current platform
	@if [ ! -f "libs/$(PLATFORM)/liblancedb_cgo.a" ]; then \
		echo "Building pre-built library for $(PLATFORM)..."; \
		./scripts/build-prebuilt-libs.sh; \
	fi
	PKG_CONFIG_PATH=$(LOCAL_PKG_CONFIG_PATH) go build .

# Build everything
build: build-go

# Run tests
test: generate-pc
	# Ensure library exists for current platform
	@if [ ! -f "libs/$(PLATFORM)/liblancedb_cgo.a" ]; then \
		echo "Building pre-built library for $(PLATFORM)..."; \
		./scripts/build-prebuilt-libs.sh; \
	fi
	PKG_CONFIG_PATH=$(LOCAL_PKG_CONFIG_PATH) go test -v ./...

# Build example
example: generate-pc
	# Ensure library exists for current platform
	@if [ ! -f "libs/$(PLATFORM)/liblancedb_cgo.a" ]; then \
		echo "Building pre-built library for $(PLATFORM)..."; \
		./scripts/build-prebuilt-libs.sh; \
	fi
	PKG_CONFIG_PATH=$(LOCAL_PKG_CONFIG_PATH) go build -o examples/basic/example ./examples/basic && \
	[ -f examples/basic/example ] && (PKG_CONFIG_PATH=$(LOCAL_PKG_CONFIG_PATH) ./examples/basic/example && rm -f examples/basic/example)

# Clean build artifacts
clean:
	cd rust-cgo && cargo clean
	go clean ./...
	rm -rf libs/
	[ -f examples/basic/example ] && rm -f examples/basic/example

# Install dependencies
deps:
	cd rust-cgo && cargo fetch
	go mod tidy

# Supported platforms for cross-compilation
OS_ARCHES := linux-amd64 linux-arm64 darwin-amd64 darwin-arm64

RUST_TARGET_linux-amd64 := x86_64-unknown-linux-gnu
RUST_TARGET_linux-arm64 := aarch64-unknown-linux-gnu
RUST_TARGET_darwin-amd64 := x86_64-apple-darwin
RUST_TARGET_darwin-arm64 := aarch64-apple-darwin

GOOS_linux-amd64 := linux
GOARCH_linux-amd64 := amd64
GOOS_linux-arm64 := linux
GOARCH_linux-arm64 := arm64
GOOS_darwin-amd64 := darwin
GOARCH_darwin-amd64 := amd64
GOOS_darwin-arm64 := darwin
GOARCH_darwin-arm64 := arm64

define BUILD_TARGET
build-$(1):
	cd rust-cgo && cargo build --release --target $(RUST_TARGET_$(1))
	# We don't run go build here because it requires the PKG_CONFIG_PATH to be set correctly for the target
	# which is hard to do in a cross-compilation make target without extra complexity.
	@echo "Built Rust library for $(1)"
endef

$(foreach target,$(OS_ARCHES),$(eval $(call BUILD_TARGET,$(target))))

# Build pre-built libraries for distribution
build-prebuilt-libs:
	@./scripts/build-prebuilt-libs.sh

# Help target
help:
	@echo "Available targets:"
	@echo "  build              - Build Go package (automatically builds Rust lib for current platform)"
	@echo "  test               - Run Go tests"
	@echo "  example            - Build and run the example program"
	@echo "  clean              - Clean build artifacts"
	@echo "  deps               - Install/update dependencies"
	@echo ""
	@echo "Cross-compilation targets (Rust lib only):"
	@echo "  build-linux-amd64  - Build for Linux x86_64"
	@echo "  build-linux-arm64  - Build for Linux ARM64"
	@echo "  build-darwin-amd64 - Build for macOS x86_64"
	@echo "  build-darwin-arm64 - Build for macOS ARM64 (Apple Silicon)"

docker-dev:
	docker build -t local/go-lancedb-builder:dev -f .docker/Dev.dockerfile \
	--build-arg GO_USER_NAME=dev \
	.

run-docker-dev:
	docker run -it --rm -v $(PWD):/ws -w /ws local/go-lancedb-builder:dev
