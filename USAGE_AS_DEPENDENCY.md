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

### Strategy 1: Pre-built Libraries (Recommended for Production)

Build the Rust library for target platforms and distribute it:

```bash
# Build for different platforms
cd golang/rust-cgo

# macOS ARM64 (Apple Silicon)
cargo build --release --target aarch64-apple-darwin

# macOS x86_64 (Intel)
cargo build --release --target x86_64-apple-darwin

# Linux x86_64
cargo build --release --target x86_64-unknown-linux-gnu

# Linux ARM64
cargo build --release --target aarch64-unknown-linux-gnu

# Windows x86_64
cargo build --release --target x86_64-pc-windows-msvc
```

**Package structure:**
```
your-app/
├── bin/
│   └── myapp
├── lib/
│   ├── darwin-arm64/
│   │   └── liblancedb_cgo.dylib
│   ├── darwin-amd64/
│   │   └── liblancedb_cgo.dylib
│   ├── linux-amd64/
│   │   └── liblancedb_cgo.so
│   └── windows-amd64/
│       └── lancedb_cgo.dll
└── README.md
```

**Set library path at runtime:**

```go
package main

import (
    "os"
    "runtime"
    "path/filepath"
)

func init() {
    // Determine platform
    libPath := filepath.Join("lib", runtime.GOOS+"-"+runtime.GOARCH)
    
    // Add to library path
    if runtime.GOOS == "linux" {
        os.Setenv("LD_LIBRARY_PATH", libPath+":"+os.Getenv("LD_LIBRARY_PATH"))
    } else if runtime.GOOS == "darwin" {
        os.Setenv("DYLD_LIBRARY_PATH", libPath+":"+os.Getenv("DYLD_LIBRARY_PATH"))
    }
}
```

### Strategy 2: Embedding with CGO

You can embed the library path in the Go binary:

```go
// In your project's lancedb_wrapper.go

package main

/*
#cgo LDFLAGS: -L${SRCDIR}/lib/${GOOS}-${GOARCH} -llancedb_cgo -lm -ldl
#cgo darwin LDFLAGS: -framework CoreFoundation -framework Security
*/
import "C"
```

### Strategy 3: System-wide Installation

Install the library system-wide:

```bash
# macOS
sudo cp liblancedb_cgo.dylib /usr/local/lib/

# Linux
sudo cp liblancedb_cgo.so /usr/local/lib/
sudo ldconfig

# Verify
ldconfig -p | grep lancedb  # Linux
ls -l /usr/local/lib/liblancedb*  # macOS
```

Then your Go code can find it automatically.

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
go build -o my-rag-app
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

