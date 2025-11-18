# Makefile for building LanceDB Go CGO bindings

.PHONY: all build clean test example

# Default target
all: build

# Build the Rust CGO library
build-rust:
	cd rust-cgo && cargo build --release

# Build the Go package (also builds Rust library via cgo)
build-go:
	cd cgo && go build .

# Build everything
build: build-rust build-go

# Run tests
test:
	cd cgo && go test -v .

# Build example
example:
	cd cgo && go build -o example ./example

# Clean build artifacts
clean:
	cd rust-cgo && cargo clean
	cd cgo && go clean
	rm -f cgo/example

# Install dependencies
deps:
	cd rust-cgo && cargo fetch
	cd cgo && go mod tidy

# Cross-compilation targets
build-linux-amd64:
	cd rust-cgo && cargo build --release --target x86_64-unknown-linux-gnu
	cd cgo && CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build .

build-linux-arm64:
	cd rust-cgo && cargo build --release --target aarch64-unknown-linux-gnu
	cd cgo && CGO_ENABLED=1 GOOS=linux GOARCH=arm64 go build .

build-darwin-amd64:
	cd rust-cgo && cargo build --release --target x86_64-apple-darwin
	cd cgo && CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build .

build-darwin-arm64:
	cd rust-cgo && cargo build --release --target aarch64-apple-darwin
	cd cgo && CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build .

build-windows-amd64:
	cd rust-cgo && cargo build --release --target x86_64-pc-windows-gnu
	cd cgo && CGO_ENABLED=1 GOOS=windows GOARCH=amd64 go build .

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
