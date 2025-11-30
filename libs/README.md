# Pre-built Static Libraries

This directory contains pre-built **static libraries** (`.a` files) for supported platforms. These are linked directly into your Go binary at compile time - no runtime dependencies.

## Directory Structure

```
libs/
├── darwin-arm64/liblancedb_cgo.a   # macOS Apple Silicon
├── darwin-amd64/liblancedb_cgo.a   # macOS Intel
├── linux-arm64/liblancedb_cgo.a    # Linux ARM64
├── linux-amd64/liblancedb_cgo.a    # Linux x86_64
└── README.md
```

## How It Works

The CGO directives in `lancedb.go` automatically select the correct static library based on your `GOOS` and `GOARCH`. When you run `go build`, the library is statically linked into your binary.

**Result:** A single binary with no runtime library dependencies.

## Building Libraries

### For Current Platform

```bash
./scripts/build-prebuilt-libs.sh
```

### For All Platforms

Requires cross-compilation setup (Rust targets + appropriate linkers):

```bash
./scripts/build-prebuilt-libs.sh --all
```

### For Specific Platform

```bash
./scripts/build-prebuilt-libs.sh --target linux-amd64
```

### Manual Build

```bash
cd rust-cgo

# Native build
cargo build --release
cp target/release/liblancedb_cgo.a ../libs/<os>-<arch>/

# Cross-compile (requires target installed)
rustup target add x86_64-unknown-linux-gnu
cargo build --release --target x86_64-unknown-linux-gnu
cp target/x86_64-unknown-linux-gnu/release/liblancedb_cgo.a ../libs/linux-amd64/
```

## Supported Platforms

| Platform | Rust Target | Status |
|----------|-------------|--------|
| darwin-arm64 | aarch64-apple-darwin | Available |
| darwin-amd64 | x86_64-apple-darwin | Build required |
| linux-arm64 | aarch64-unknown-linux-gnu | Available |
| linux-amd64 | x86_64-unknown-linux-gnu | Build required |

## Requirements

- Rust 1.70+
- For cross-compilation: appropriate Rust targets and system linkers

## Git LFS

Static libraries are stored using Git LFS due to their size (~40-100MB each). Make sure Git LFS is installed:

```bash
git lfs install
git lfs pull
```
