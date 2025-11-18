# Pre-built Rust CGO Libraries

This directory contains pre-built Rust CGO libraries for specific architectures.

## Directory Structure

```
libs/
├── linux-arm64/
│   └── liblancedb_cgo.so
└── darwin-arm64/
    └── liblancedb_cgo.dylib
```

## Usage

The Go bindings automatically detect and use pre-built libraries from this directory when available. The CGO linker will:

1. First check `libs/${GOOS}-${GOARCH}/` for a pre-built library
2. Fall back to `rust-cgo/target/release/` if no pre-built library is found

This means you can:
- **Use pre-built libraries**: Place libraries in the appropriate `libs/` subdirectory
- **Build from source**: If no pre-built library exists, it will use the locally built version

## Building Pre-built Libraries

To build libraries for all supported architectures:

```bash
./scripts/build-prebuilt-libs.sh
```

This script will:
- Install required Rust targets if needed
- Build release libraries for linux/arm64 and darwin/arm64
- Copy them to the appropriate `libs/` subdirectories

## Building for Specific Architecture

To build for a specific architecture manually:

```bash
cd rust-cgo

# Linux ARM64
cargo build --release --target aarch64-unknown-linux-gnu
cp target/aarch64-unknown-linux-gnu/release/liblancedb_cgo.so ../libs/linux-arm64/

# macOS ARM64 (Apple Silicon)
cargo build --release --target aarch64-apple-darwin
cp target/aarch64-apple-darwin/release/liblancedb_cgo.dylib ../libs/darwin-arm64/
```

## Requirements

- Rust 1.70+ with rustup
- Required Rust targets installed (script will install automatically):
  - `aarch64-unknown-linux-gnu` for Linux ARM64
  - `aarch64-apple-darwin` for macOS ARM64

## Notes

- Pre-built libraries are optimized release builds (`--release` flag)
- Libraries are architecture-specific and cannot be cross-used
- The Go bindings will automatically select the correct library based on `GOOS` and `GOARCH`

