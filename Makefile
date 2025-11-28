# Makefile for building LanceDB Go CGO bindings
export BUILDKIT_PROGRESS ?= plain

.PHONY: all build clean test example

# Default target
all: build

# Build the Rust CGO library
build-rust:
	cd rust-cgo && cargo build --release

# Build the Go package (also builds Rust library via cgo)
build-go:
	go build .

# Build everything
build: build-rust build-go

# Run tests
test:
	go test -v ./...

# Build example
example:
	go build -o examples/basic/example ./examples/basic && \
	[ -f examples/basic/example ] && (./examples/basic/example && rm -f examples/basic/example)

# Clean build artifacts
clean:
	cd rust-cgo && cargo clean
	go clean ./...
	[ -f examples/basic/example ] && rm -f examples/basic/example

# Install dependencies
deps:
	cd rust-cgo && cargo fetch
	go mod tidy

# Cross-compilation targets (parameterized to avoid duplication)
OS_ARCHES := linux-amd64 linux-arm64 darwin-amd64 darwin-arm64 windows-amd64

RUST_TARGET_linux-amd64 := x86_64-unknown-linux-gnu
RUST_TARGET_linux-arm64 := aarch64-unknown-linux-gnu
RUST_TARGET_darwin-amd64 := x86_64-apple-darwin
RUST_TARGET_darwin-arm64 := aarch64-apple-darwin
RUST_TARGET_windows-amd64 := x86_64-pc-windows-gnu

GOOS_linux-amd64 := linux
GOARCH_linux-amd64 := amd64
GOOS_linux-arm64 := linux
GOARCH_linux-arm64 := arm64
GOOS_darwin-amd64 := darwin
GOARCH_darwin-amd64 := amd64
GOOS_darwin-arm64 := darwin
GOARCH_darwin-arm64 := arm64
GOOS_windows-amd64 := windows
GOARCH_windows-amd64 := amd64

define BUILD_TARGET
build-$(1):
	cd rust-cgo && cargo build --release --target $(RUST_TARGET_$(1))
	CGO_ENABLED=1 GOOS=$(GOOS_$(1)) GOARCH=$(GOARCH_$(1)) go build .
endef

$(foreach target,$(OS_ARCHES),$(eval $(call BUILD_TARGET,$(target))))

# Build pre-built libraries for distribution
build-prebuilt-libs:
	@./scripts/build-prebuilt-libs.sh

# Help target
help:
	@echo "Available targets:"
	@echo "  build          - Build both Rust and Go components"
	@echo "  build-rust     - Build only the Rust CGO library"
	@echo "  build-go       - Build only the Go package"
	@echo "  test           - Run Go tests"
	@echo "  example        - Build the example program"
	@echo "  clean          - Clean build artifacts"
	@echo "  deps           - Install/update dependencies"
	@echo "Cross-compilation targets:"
	@echo "  build-linux-amd64     - Build for Linux x86_64"
	@echo "  build-linux-arm64     - Build for Linux ARM64"
	@echo "  build-darwin-amd64    - Build for macOS x86_64"
	@echo "  build-darwin-arm64    - Build for macOS ARM64 (Apple Silicon)"
	@echo "  build-windows-amd64   - Build for Windows x86_64"
	@echo "  build-prebuilt-libs   - Build pre-built libraries for linux/arm64 and darwin/arm64"

docker-dev:
	docker build -t local/go-lancedb-builder:dev -f .docker/Dev.dockerfile \
	--build-arg GO_USER_NAME=dev \
	.

run-docker-dev:
	docker run -it --rm -v $(PWD):/ws -w /ws local/go-lancedb-builder:dev
