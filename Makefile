# Makefile for building LanceDB Go CGO bindings
export BUILDKIT_PROGRESS ?= plain

.PHONY: all build clean test example

# Determine current platform
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
PLATFORM := $(GOOS)-$(GOARCH)

# Library path for local development
LOCAL_LIB_PATH := $(CURDIR)/libs/$(PLATFORM)
export CGO_LDFLAGS := -L$(LOCAL_LIB_PATH)

# Default target
all: build

# Build the Rust CGO library
build-rust:
	cd rust-cgo && cargo build --release

# Build the Go package (also builds Rust library via cgo)
build-go:
	# Ensure library exists for current platform
	@if [ ! -f "libs/$(PLATFORM)/liblancedb_cgo.a" ]; then \
		echo "Building pre-built library for $(PLATFORM)..."; \
		./scripts/build-prebuilt-libs.sh; \
	fi
	go build .

# Build everything
build: build-go

# Run tests
test:
	# Ensure library exists for current platform
	@if [ ! -f "libs/$(PLATFORM)/liblancedb_cgo.a" ]; then \
		echo "Building pre-built library for $(PLATFORM)..."; \
		./scripts/build-prebuilt-libs.sh; \
	fi
	go test -v ./...

# Build example
example:
	# Ensure library exists for current platform
	@if [ ! -f "libs/$(PLATFORM)/liblancedb_cgo.a" ]; then \
		echo "Building pre-built library for $(PLATFORM)..."; \
		./scripts/build-prebuilt-libs.sh; \
	fi
	go build -o examples/basic/example ./examples/basic && \
	[ -f examples/basic/example ] && (./examples/basic/example && rm -f examples/basic/example)

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
	# We don't run go build here because it requires the CGO_LDFLAGS to be set correctly for the target
	# which is hard to do in a cross-compilation make target without extra complexity.
	# Users should use the CGO_LDFLAGS env var as documented.
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
