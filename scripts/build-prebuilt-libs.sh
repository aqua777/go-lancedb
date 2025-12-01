#!/bin/bash
# Build script for pre-built static Rust CGO libraries
# This script builds static libraries (.a) for distribution.
# 
# Usage:
#   ./scripts/build-prebuilt-libs.sh           # Build for current platform
#   ./scripts/build-prebuilt-libs.sh --all     # Build for all supported platforms (requires cross-compilation setup)
#   ./scripts/build-prebuilt-libs.sh --target linux-amd64  # Build for specific target

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
RUST_CGO_DIR="$REPO_ROOT/rust-cgo"
LIBS_DIR="$REPO_ROOT/libs"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Map platform to Rust target (portable - no associative arrays)
get_rust_target() {
    case "$1" in
        darwin-arm64) echo "aarch64-apple-darwin" ;;
        darwin-amd64) echo "x86_64-apple-darwin" ;;
        linux-arm64)  echo "aarch64-unknown-linux-gnu" ;;
        linux-amd64)  echo "x86_64-unknown-linux-gnu" ;;
        *) echo "" ;;
    esac
}

# All supported platforms
ALL_PLATFORMS="darwin-arm64 darwin-amd64 linux-arm64 linux-amd64"

# Check if cargo is available
check_cargo() {
    if command -v cargo >/dev/null 2>&1; then
        return 0
    else
        return 1
    fi
}

# Detect current platform and return lib_dir key
detect_current_platform() {
    local os=$(uname -s | tr '[:upper:]' '[:lower:]')
    local arch=$(uname -m)
    
    # Map architecture
    case "$arch" in
        arm64|aarch64)
            local go_arch="arm64"
            ;;
        x86_64|amd64)
            local go_arch="amd64"
            ;;
        *)
            echo ""
            return 1
            ;;
    esac
    
    # Map OS
    case "$os" in
        darwin|linux)
            echo "${os}-${go_arch}"
            ;;
        *)
            echo ""
            return 1
            ;;
    esac
}

# Install rust target if needed
install_rust_target() {
    local target=$1
    local rust_target=$(get_rust_target "$target")
    
    if ! rustup target list --installed 2>/dev/null | grep -q "$rust_target"; then
        info "Installing Rust target: $rust_target"
        rustup target add "$rust_target"
    fi
}

# Build static library for a specific platform
build_for_platform() {
    local lib_dir=$1
    local rust_target=$(get_rust_target "$lib_dir")
    local current_platform=$(detect_current_platform)
    
    if [ -z "$rust_target" ]; then
        error "Unknown platform: $lib_dir"
        return 1
    fi
    
    info "Building static library for: $lib_dir (Rust target: $rust_target)"
    
    cd "$RUST_CGO_DIR"
    
    # Determine if this is native or cross-compilation
    if [ "$lib_dir" = "$current_platform" ]; then
        # Native build - no target flag needed
        info "Native build for $lib_dir"
        cargo build --release
        local src_dir="$RUST_CGO_DIR/target/release"
    else
        # Cross-compilation
        info "Cross-compiling for $lib_dir"
        install_rust_target "$lib_dir"
        cargo build --release --target "$rust_target"
        local src_dir="$RUST_CGO_DIR/target/$rust_target/release"
    fi
    
    # Create destination directory
    local dest_dir="$LIBS_DIR/$lib_dir"
    mkdir -p "$dest_dir"
    
    # Copy static library
    local src_lib="$src_dir/liblancedb_cgo.a"
    if [ -f "$src_lib" ]; then
        local dest_lib="$dest_dir/liblancedb_cgo.a"
        info "Copying static library to $dest_lib"
        cp "$src_lib" "$dest_lib"
        ls -lh "$dest_lib"
    else
        error "Static library not found at: $src_lib"
        error "Make sure Cargo.toml has 'staticlib' in crate-type"
        return 1
    fi
    
    info "Successfully built $lib_dir"
    return 0
}

# Build for all supported platforms
build_all() {
    local current_platform=$(detect_current_platform)
    local failed=""
    
    info "Building for all supported platforms..."
    info "Current platform: $current_platform"
    
    for platform in $ALL_PLATFORMS; do
        echo ""
        if [ "$platform" = "$current_platform" ]; then
            info "=== Building $platform (native) ==="
        else
            info "=== Building $platform (cross-compile) ==="
        fi
        
        if ! build_for_platform "$platform"; then
            warn "Failed to build for $platform"
            failed="$failed $platform"
        fi
    done
    
    echo ""
    if [ -z "$failed" ]; then
        info "All platforms built successfully!"
    else
        warn "Some platforms failed to build:$failed"
        warn "Cross-compilation may require additional setup (linkers, SDKs)"
        return 1
    fi
}

# Print usage
usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  --all              Build for all supported platforms"
    echo "  --target PLATFORM  Build for specific platform"
    echo "  --list             List supported platforms"
    echo "  --help             Show this help"
    echo ""
    echo "If no options given, builds for current platform only."
    echo ""
    echo "Supported platforms:"
    for platform in $ALL_PLATFORMS; do
        echo "  $platform -> $(get_rust_target $platform)"
    done
}

# Main
main() {
    # Check prerequisites
    if ! check_cargo; then
        error "Rust/Cargo not found. Please install Rust first:"
        error "  Visit: https://www.rust-lang.org/tools/install"
        exit 1
    fi
    
    info "Using Rust: $(cargo --version)"
    
    case "${1:-}" in
        --all)
            build_all
            ;;
        --target)
            if [ -z "${2:-}" ]; then
                error "Missing platform argument for --target"
                usage
                exit 1
            fi
            build_for_platform "$2"
            ;;
        --list)
            echo "Supported platforms:"
            for platform in $ALL_PLATFORMS; do
                echo "  $platform -> $(get_rust_target $platform)"
            done
            ;;
        --help|-h)
            usage
            ;;
        "")
            # Default: build for current platform
            local current=$(detect_current_platform)
            if [ -z "$current" ]; then
                error "Unsupported platform: $(uname -s) $(uname -m)"
                exit 1
            fi
            info "Building for current platform: $current"
            build_for_platform "$current"
            ;;
        *)
            error "Unknown option: $1"
            usage
            exit 1
            ;;
    esac
    
    echo ""
    info "Build completed. Libraries are in: $LIBS_DIR/"
}

main "$@"
