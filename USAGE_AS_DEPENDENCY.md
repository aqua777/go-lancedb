# Using LanceDB Go Bindings as a Dependency

This guide explains how to use the LanceDB Go bindings in your own projects.

## Table of Contents

1. [Quick Start (Local Development)](#quick-start-local-development)
2. [Module Setup](#module-setup)
3. [Building the CGO Library](#building-the-cgo-library)
4. [Distribution Strategies](#distribution-strategies)
5. [Docker Deployment](#docker-deployment)
6. [CI/CD Integration](#cicd-integration)
7. [Troubleshooting](#troubleshooting)

---

## Important: Dynamic Library Dependency

**⚠️ Critical:** This library uses a dynamic Rust library (`liblancedb_cgo.dylib`/`.so`/`.dll`) that must be distributed alongside your compiled binary.

### For Development

During development, the library is automatically found using the embedded rpath pointing to the module cache. You can simply:

```bash
go get github.com/aqua777/go-lancedb
go run main.go  # Works out of the box
```

### For Distribution

When building a binary for distribution, you **must**:

1. **Set the CGO flag whitelist** (allows `@executable_path` rpath):
   ```bash
   export CGO_LDFLAGS_ALLOW='-Wl,-rpath,@executable_path'
   ```

2. **Build your binary**:
   ```bash
   go build -o myapp
   ```

3. **Copy the dynamic library** next to your binary:
   ```bash
   # macOS (from go module cache or libs directory)
   cp $(go list -f '{{.Dir}}' github.com/aqua777/go-lancedb)/libs/darwin-arm64/liblancedb_cgo.dylib .
   
   # Linux
   cp $(go list -f '{{.Dir}}' github.com/aqua777/go-lancedb)/libs/linux-amd64/liblancedb_cgo.so .
   ```

4. **Distribute both files together**:
   ```
   your-release/
   ├── myapp                    # Your binary
   └── liblancedb_cgo.dylib    # The dynamic library (must be in same directory)
   ```

### Platform-Specific Libraries

Available in the `libs/` directory of the module:
- macOS ARM64: `libs/darwin-arm64/liblancedb_cgo.dylib`
- macOS Intel: `libs/darwin-amd64/liblancedb_cgo.dylib`
- Linux AMD64: `libs/linux-amd64/liblancedb_cgo.so`
- Linux ARM64: `libs/linux-arm64/liblancedb_cgo.so`

---

## Quick Start (Local Development)

### Option 1: Direct Path (Development)

If the LanceDB repository is cloned locally:

```bash
# In your project directory
cd /path/to/your-project

# Initialize Go module if needed
go mod init myproject

# Add replace directive to use local version
go mod edit -replace github.com/lancedb/lancedb/golang/cgo=/path/to/lancedb/golang/cgo

# Add the dependency
go get github.com/lancedb/lancedb/golang/cgo
```

**Your `go.mod` will look like:**

```go
module myproject

go 1.21

require github.com/lancedb/lancedb/golang/cgo v0.0.0

replace github.com/lancedb/lancedb/golang/cgo => /path/to/lancedb/golang/cgo
```

**Build the Rust library first:**

```bash
cd /path/to/lancedb/golang/rust-cgo
cargo build --release
```

**Your project code:**

```go
package main

import (
    "log"
    lancedb "github.com/lancedb/lancedb/golang/cgo"
)

func main() {
    db, err := lancedb.Connect("./my_db")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()
    
    // Your code here...
}
```

### Option 2: Git Submodule (Recommended for Projects)

Add LanceDB as a submodule:

```bash
# Add submodule
cd /path/to/your-project
git submodule add https://github.com/lancedb/lancedb.git vendor/lancedb

# Update go.mod
go mod edit -replace github.com/lancedb/lancedb/golang/cgo=./vendor/lancedb/golang/cgo

# Build the CGO library
cd vendor/lancedb/golang/rust-cgo
cargo build --release

# Back to your project
cd ../../../..
go build
```

---

## Module Setup

### Project Structure

```
your-project/
├── go.mod
├── go.sum
├── main.go
├── vendor/                    # Optional: vendored dependencies
│   └── lancedb/              # Git submodule
└── .gitmodules               # If using submodules
```

### go.mod Configuration

**Option A: Local Development**
```go
module github.com/yourusername/your-project

go 1.21

require (
    github.com/lancedb/lancedb/golang/cgo v0.0.0
    github.com/apache/arrow/go/v17 v17.0.0
)

replace github.com/lancedb/lancedb/golang/cgo => ./vendor/lancedb/golang/cgo
```

**Option B: Future (Once Published)**
```go
module github.com/yourusername/your-project

go 1.21

require (
    github.com/lancedb/lancedb/golang/cgo v0.10.0
    github.com/apache/arrow/go/v17 v17.0.0
)
```

---

## Building the CGO Library

The Rust CGO library must be built before you can use the Go bindings.

### Manual Build

```bash
cd /path/to/lancedb/golang/rust-cgo

# Development build (faster)
cargo build

# Production build (optimized, recommended)
cargo build --release
```

The library will be at:
- Development: `target/debug/liblancedb_cgo.{dylib,so,dll}`
- Release: `target/release/liblancedb_cgo.{dylib,so,dll}`

### Makefile Helper

If you're using the submodule approach, create a Makefile:

```makefile
# Makefile in your project root

.PHONY: build-lancedb
build-lancedb:
	cd vendor/lancedb/golang/rust-cgo && cargo build --release

.PHONY: build
build: build-lancedb
	go build -o myapp .

.PHONY: test
test: build-lancedb
	go test ./...

.PHONY: clean
clean:
	rm -f myapp
	cd vendor/lancedb/golang/rust-cgo && cargo clean
```

Usage:
```bash
make build    # Builds Rust library and Go binary
make test     # Runs tests
make clean    # Cleans build artifacts
```

---

## Distribution Strategies

### Strategy 1: Simple Distribution (Recommended)

The library is configured with `@executable_path` rpath, so the dynamic library is found next to your binary at runtime.

**Build script example:**

```bash
#!/bin/bash
# build.sh

# Set required CGO flag
export CGO_LDFLAGS_ALLOW='-Wl,-rpath,@executable_path'

# Build your app
go build -o myapp

# Copy the appropriate dynamic library
case "$(uname -s)" in
    Darwin)
        if [[ "$(uname -m)" == "arm64" ]]; then
            cp $(go list -f '{{.Dir}}' github.com/aqua777/go-lancedb)/libs/darwin-arm64/liblancedb_cgo.dylib .
        else
            cp $(go list -f '{{.Dir}}' github.com/aqua777/go-lancedb)/libs/darwin-amd64/liblancedb_cgo.dylib .
        fi
        ;;
    Linux)
        if [[ "$(uname -m)" == "aarch64" ]]; then
            cp $(go list -f '{{.Dir}}' github.com/aqua777/go-lancedb)/libs/linux-arm64/liblancedb_cgo.so .
        else
            cp $(go list -f '{{.Dir}}' github.com/aqua777/go-lancedb)/libs/linux-amd64/liblancedb_cgo.so .
        fi
        ;;
esac

echo "Built myapp with dynamic library"
```

**Package structure:**
```
your-release/
├── myapp                    # Your compiled binary
└── liblancedb_cgo.dylib    # Dynamic library (same directory)
```

**Ship both files together** - that's it! The binary will find the library at runtime.

### Strategy 2: Subdirectory Organization

If you prefer organizing libraries in a subdirectory:

```bash
# Build with custom rpath
export CGO_LDFLAGS_ALLOW='-Wl,-rpath,@executable_path/lib'
go build -ldflags="-r \$ORIGIN/lib" -o myapp

# Create lib directory and copy
mkdir -p lib
cp $(go list -f '{{.Dir}}' github.com/aqua777/go-lancedb)/libs/darwin-arm64/liblancedb_cgo.dylib lib/
```

**Note:** You'll need to modify the `lancedb.go` LDFLAGS in your vendored copy to use `-Wl,-rpath,@executable_path/lib` instead.

**Package structure:**
```
your-release/
├── myapp
└── lib/
    └── liblancedb_cgo.dylib
```

### Strategy 3: Cross-Platform Releases

For multi-platform releases with goreleaser or similar:

```yaml
# .goreleaser.yml
builds:
  - env:
      - CGO_ENABLED=1
      - CGO_LDFLAGS_ALLOW=-Wl,-rpath,@executable_path
    goos:
      - darwin
      - linux
    goarch:
      - amd64
      - arm64
    hooks:
      post:
        # Copy the appropriate dynamic library
        - bash -c 'cp $(go list -f "{{.Dir}}" github.com/aqua777/go-lancedb)/libs/{{ .Os }}-{{ .Arch }}/liblancedb_cgo.{{ if eq .Os "darwin" }}dylib{{ else }}so{{ end }} {{ .Path }}/../'

archives:
  - format: tar.gz
    files:
      - liblancedb_cgo.*
```

### Strategy 4: System-wide Installation (Not Recommended)

Only for system-level deployments where you control the environment:

```bash
# macOS
sudo cp liblancedb_cgo.dylib /usr/local/lib/
sudo update_dyld_shared_cache  # macOS 11+

# Linux
sudo cp liblancedb_cgo.so /usr/local/lib/
sudo ldconfig
```

**Downsides:** Requires sudo, conflicts between versions, harder to uninstall.

---

## Docker Deployment

### Dockerfile Example

```dockerfile
# Build stage
FROM rust:1.75 AS rust-builder

WORKDIR /build

# Copy Rust CGO library
COPY vendor/lancedb/golang/rust-cgo ./rust-cgo
WORKDIR /build/rust-cgo

# Build Rust library
RUN cargo build --release

# Go build stage
FROM golang:1.21 AS go-builder

WORKDIR /app

# Copy go.mod and go.sum
COPY go.mod go.sum ./
RUN go mod download

# Copy Rust library from previous stage
COPY --from=rust-builder /build/rust-cgo/target/release/liblancedb_cgo.so /usr/local/lib/

# Copy source code
COPY . .

# Build Go application
ENV CGO_LDFLAGS_ALLOW='-Wl,-rpath,@executable_path'
RUN CGO_ENABLED=1 go build -o myapp .

# Runtime stage
FROM debian:bookworm-slim

# Install runtime dependencies
RUN apt-get update && apt-get install -y \
    ca-certificates \
    libssl3 \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy binary and library
COPY --from=go-builder /app/myapp .
COPY --from=go-builder /usr/local/lib/liblancedb_cgo.so /usr/local/lib/

# Update library cache
RUN ldconfig

# Run
CMD ["./myapp"]
```

### Docker Compose Example

```yaml
version: '3.8'

services:
  app:
    build:
      context: .
      dockerfile: Dockerfile
    volumes:
      - ./data:/app/data  # Persist LanceDB data
    environment:
      - LANCEDB_PATH=/app/data/lancedb
    ports:
      - "8080:8080"
```

---

## CI/CD Integration

### GitHub Actions Example

```yaml
# .github/workflows/build.yml
name: Build and Test

on: [push, pull_request]

jobs:
  build:
    runs-on: ubuntu-latest
    
    steps:
      - name: Checkout code
        uses: actions/checkout@v4
        with:
          submodules: recursive  # Important if using submodules
      
      - name: Setup Rust
        uses: actions-rs/toolchain@v1
        with:
          toolchain: stable
          override: true
      
      - name: Setup Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      
      - name: Cache Rust dependencies
        uses: actions/cache@v3
        with:
          path: |
            ~/.cargo/registry
            ~/.cargo/git
            vendor/lancedb/golang/rust-cgo/target
          key: ${{ runner.os }}-cargo-${{ hashFiles('**/Cargo.lock') }}
      
      - name: Build Rust CGO library
        run: |
          cd vendor/lancedb/golang/rust-cgo
          cargo build --release
      
      - name: Build Go application
        env:
          CGO_LDFLAGS_ALLOW: '-Wl,-rpath,@executable_path'
        run: go build -v ./...
      
      - name: Run tests
        run: go test -v ./...
      
      - name: Upload artifacts
        uses: actions/upload-artifact@v3
        with:
          name: lancedb-cgo-linux
          path: vendor/lancedb/golang/rust-cgo/target/release/liblancedb_cgo.so
```

### GitLab CI Example

```yaml
# .gitlab-ci.yml
stages:
  - build-rust
  - build-go
  - test

variables:
  CARGO_HOME: $CI_PROJECT_DIR/.cargo

cache:
  paths:
    - .cargo/
    - vendor/lancedb/golang/rust-cgo/target/

build-rust:
  stage: build-rust
  image: rust:1.75
  script:
    - cd vendor/lancedb/golang/rust-cgo
    - cargo build --release
  artifacts:
    paths:
      - vendor/lancedb/golang/rust-cgo/target/release/liblancedb_cgo.so
    expire_in: 1 hour

build-go:
  stage: build-go
  image: golang:1.21
  dependencies:
    - build-rust
  variables:
    CGO_LDFLAGS_ALLOW: '-Wl,-rpath,@executable_path'
  script:
    - cp vendor/lancedb/golang/rust-cgo/target/release/liblancedb_cgo.so /usr/local/lib/
    - ldconfig
    - go build -v ./...
  artifacts:
    paths:
      - myapp

test:
  stage: test
  image: golang:1.21
  dependencies:
    - build-rust
  script:
    - cp vendor/lancedb/golang/rust-cgo/target/release/liblancedb_cgo.so /usr/local/lib/
    - ldconfig
    - go test -v ./...
```

---

## Troubleshooting

### Issue: Library Not Found

**Error:**
```
dyld: Library not loaded: liblancedb_cgo.dylib
```

**Solution:**

1. **Check library exists:**
   ```bash
   ls -l golang/rust-cgo/target/release/liblancedb_cgo.*
   ```

2. **Set library path (macOS):**
   ```bash
   export DYLD_LIBRARY_PATH=/path/to/lancedb/golang/rust-cgo/target/release:$DYLD_LIBRARY_PATH
   ```

3. **Set library path (Linux):**
   ```bash
   export LD_LIBRARY_PATH=/path/to/lancedb/golang/rust-cgo/target/release:$LD_LIBRARY_PATH
   ```

4. **Or install system-wide:**
   ```bash
   # macOS
   sudo cp liblancedb_cgo.dylib /usr/local/lib/
   
   # Linux
   sudo cp liblancedb_cgo.so /usr/local/lib/
   sudo ldconfig
   ```

### Issue: Invalid LDFLAGS

**Error:**
```
invalid flag in #cgo LDFLAGS: -Wl,-rpath,@executable_path
```

**Solution:**

Go's security sandbox blocks the `@executable_path` flag by default. Whitelist it:

```bash
export CGO_LDFLAGS_ALLOW='-Wl,-rpath,@executable_path'
go build
```

Add to your shell profile for persistence:
```bash
# ~/.zshrc or ~/.bashrc
export CGO_LDFLAGS_ALLOW='-Wl,-rpath,@executable_path'
```

Or create a build script:
```bash
#!/bin/bash
export CGO_LDFLAGS_ALLOW='-Wl,-rpath,@executable_path'
go build "$@"
```

### Issue: CGO Not Enabled

**Error:**
```
package github.com/lancedb/lancedb/golang/cgo: C source files not allowed when not using cgo
```

**Solution:**
```bash
export CGO_ENABLED=1
go build
```

### Issue: Wrong Architecture

**Error:**
```
ld: warning: ignoring file liblancedb_cgo.dylib, building for macOS-arm64 but attempting to link with file built for macOS-x86_64
```

**Solution:**
Build for correct target:
```bash
# Apple Silicon
cargo build --release --target aarch64-apple-darwin

# Intel Mac
cargo build --release --target x86_64-apple-darwin
```

### Issue: Module Not Found

**Error:**
```
cannot find module github.com/lancedb/lancedb/golang/cgo
```

**Solution:**
Add replace directive in `go.mod`:
```go
replace github.com/lancedb/lancedb/golang/cgo => ./vendor/lancedb/golang/cgo
```

---

## Best Practices

### 1. Version Pinning

Pin to specific versions in production:

```go
// go.mod
require (
    github.com/lancedb/lancedb/golang/cgo v0.10.0
    github.com/apache/arrow/go/v17 v17.0.0
)
```

### 2. Library Caching

Cache the built library in CI/CD to speed up builds:

```yaml
# GitHub Actions
- uses: actions/cache@v3
  with:
    path: vendor/lancedb/golang/rust-cgo/target
    key: ${{ runner.os }}-lancedb-${{ hashFiles('**/Cargo.lock') }}
```

### 3. Cross-Platform Builds

Use build tags for platform-specific code:

```go
// +build linux darwin

package myapp

func platformSpecificInit() {
    // Unix-specific code
}
```

```go
// +build windows

package myapp

func platformSpecificInit() {
    // Windows-specific code
}
```

### 4. Graceful Degradation

Check if library is available:

```go
package main

import (
    "errors"
    lancedb "github.com/lancedb/lancedb/golang/cgo"
)

func NewDB(path string) (*lancedb.Connection, error) {
    db, err := lancedb.Connect(path)
    if err != nil {
        return nil, errors.New("LanceDB not available: " + err.Error())
    }
    return db, nil
}
```

### 5. Resource Cleanup

Always use `defer` for cleanup:

```go
func processData() error {
    db, err := lancedb.Connect("./db")
    if err != nil {
        return err
    }
    defer db.Close()  // Ensures cleanup even on error
    
    table, err := db.OpenTable("data")
    if err != nil {
        return err
    }
    defer table.Close()  // Nested defers work correctly
    
    // Your code...
    return nil
}
```

---

## Example: Complete Application Setup

```bash
# 1. Create your project
mkdir my-rag-app
cd my-rag-app

# 2. Initialize Go module
go mod init github.com/myuser/my-rag-app

# 3. Add LanceDB as submodule
git submodule add https://github.com/lancedb/lancedb.git vendor/lancedb

# 4. Update go.mod
cat >> go.mod << 'EOF'

replace github.com/lancedb/lancedb/golang/cgo => ./vendor/lancedb/golang/cgo
EOF

go get github.com/lancedb/lancedb/golang/cgo
go get github.com/apache/arrow/go/v17

# 5. Build Rust library
cd vendor/lancedb/golang/rust-cgo
cargo build --release
cd ../../../..

# 6. Create your application
cat > main.go << 'EOF'
package main

import (
    "fmt"
    "log"
    lancedb "github.com/lancedb/lancedb/golang/cgo"
)

func main() {
    db, err := lancedb.Connect("./my_db")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()
    
    fmt.Println("LanceDB connected successfully!")
}
EOF

# 7. Build and run
export CGO_LDFLAGS_ALLOW='-Wl,-rpath,@executable_path'
go build -o my-rag-app

# Copy the dynamic library for distribution
cp vendor/lancedb/golang/cgo/libs/darwin-arm64/liblancedb_cgo.dylib .

# Run
./my-rag-app
```

---

## Next Steps

1. See [README.md](README.md) for API documentation
2. Check [FEATURES.md](FEATURES.md) for capabilities
3. Review [examples/](cgo/example/) for working code
4. Read [IMPLEMENTATION_STATUS.md](IMPLEMENTATION_STATUS.md) for feature completeness

---

**Need help?** Check the [Troubleshooting](#troubleshooting) section or open an issue on GitHub.

