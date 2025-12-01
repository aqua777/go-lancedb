# Using go-lancedb as a Dependency

This package uses static linking - your Go binary includes everything it needs. No runtime library dependencies, no copying `.dylib` or `.so` files around.

## Quick Start

```bash
# Step 1: Add the dependency
go get github.com/aqua777/go-lancedb

# Step 2: Install the native library
go run github.com/aqua777/go-lancedb/cmd/lancedb-install@latest

# Step 3: Set CGO_LDFLAGS (the installer outputs the exact command)
# Example for macOS ARM64:
export CGO_LDFLAGS="-L$GOPATH/lib/lancedb/darwin-arm64 -llancedb_cgo -lm -ldl -lresolv -framework CoreFoundation -framework Security -framework SystemConfiguration"

# Step 4: Build your app
go build -o myapp .
```

Add the `CGO_LDFLAGS` export to your `~/.bashrc` or `~/.zshrc` so it persists across sessions.

## Why Two Steps?

The native library (`liblancedb_cgo.a`) is 100MB+ and stored with Git LFS. Unfortunately, `go get` doesn't fetch Git LFS files - you'd just get a 133-byte pointer file.

The installer downloads the real library from GitHub Releases to `$GOPATH/lib/lancedb/` and tells you exactly what `CGO_LDFLAGS` to set.

## Example Code

```go
package main

import (
    "fmt"
    "log"

    lancedb "github.com/aqua777/go-lancedb"
)

func main() {
    db, err := lancedb.Connect("./my_database")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    fmt.Println("Connected to LanceDB!")
}
```

## Installer Options

```bash
# Install latest version
go run github.com/aqua777/go-lancedb/cmd/lancedb-install@latest

# Install specific version
go run github.com/aqua777/go-lancedb/cmd/lancedb-install@latest --version v0.0.7
```

The installer:
- Detects your OS and architecture automatically
- Downloads the correct library to `$GOPATH/lib/lancedb/{os}-{arch}/`
- Outputs the exact `CGO_LDFLAGS` for your platform

## Platform-Specific CGO_LDFLAGS

| Platform | CGO_LDFLAGS |
|----------|-------------|
| macOS ARM64 | `-L$GOPATH/lib/lancedb/darwin-arm64 -llancedb_cgo -lm -ldl -lresolv -framework CoreFoundation -framework Security -framework SystemConfiguration` |
| macOS Intel | `-L$GOPATH/lib/lancedb/darwin-amd64 -llancedb_cgo -lm -ldl -lresolv -framework CoreFoundation -framework Security -framework SystemConfiguration` |
| Linux ARM64 | `-L$GOPATH/lib/lancedb/linux-arm64 -llancedb_cgo -lm -ldl -lpthread` |
| Linux x86_64 | `-L$GOPATH/lib/lancedb/linux-amd64 -llancedb_cgo -lm -ldl -lpthread` |

## Supported Platforms

| Platform | Architecture | Status |
|----------|--------------|--------|
| macOS | Apple Silicon (arm64) | Ready |
| macOS | Intel (amd64) | Build from source |
| Linux | ARM64 | Ready |
| Linux | x86_64 (amd64) | Build from source |

Platforms marked "Build from source" require you to compile the Rust library yourself. See [Building from Source](#building-from-source) below.

## Requirements

- Go 1.21+
- CGO enabled (default on most systems)
- For building from source: Rust toolchain

## Building from Source

If pre-built libraries aren't available for your platform, or you want to customize the build:

### 1. Install Rust

```bash
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
```

### 2. Clone and Build

```bash
# Get the source
git clone https://github.com/aqua777/go-lancedb.git
cd go-lancedb

# Build the Rust library
./scripts/build-prebuilt-libs.sh

# Verify it worked
ls -la libs/$(go env GOOS)-$(go env GOARCH)/liblancedb_cgo.a
```

### 3. Use Local Version

In your project's `go.mod`:

```go
replace github.com/aqua777/go-lancedb => /path/to/go-lancedb
```

## Docker

```dockerfile
FROM golang:1.21 AS builder

WORKDIR /app

# Install the native library
RUN go run github.com/aqua777/go-lancedb/cmd/lancedb-install@latest

# Set CGO_LDFLAGS for Linux (GOPATH defaults to /go in golang image)
ENV CGO_LDFLAGS="-L/go/lib/lancedb/linux-amd64 -llancedb_cgo -lm -ldl -lpthread"

COPY . .
RUN go build -o myapp .

FROM debian:bookworm-slim
COPY --from=builder /app/myapp /usr/local/bin/
CMD ["myapp"]
```

No special library copying needed - the binary is self-contained.

## CI/CD

### GitHub Actions

```yaml
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'

      - name: Install LanceDB native library
        run: go run github.com/aqua777/go-lancedb/cmd/lancedb-install@latest

      - name: Build
        env:
          CGO_LDFLAGS: "-L$HOME/go/lib/lancedb/linux-amd64 -llancedb_cgo -lm -ldl -lpthread"
        run: go build ./...

      - name: Test
        env:
          CGO_LDFLAGS: "-L$HOME/go/lib/lancedb/linux-amd64 -llancedb_cgo -lm -ldl -lpthread"
        run: go test ./...
```

### GitHub Actions (Multi-Platform)

```yaml
jobs:
  build:
    name: Build ${{ matrix.os_arch }}
    runs-on: ${{ matrix.runner }}
    strategy:
      matrix:
        include:
          - os_arch: linux-amd64
            runner: ubuntu-latest
            cgo_ldflags: "-L$HOME/go/lib/lancedb/linux-amd64 -llancedb_cgo -lm -ldl -lpthread"
          - os_arch: linux-arm64
            runner: ubuntu-latest
            cgo_ldflags: "-L$HOME/go/lib/lancedb/linux-arm64 -llancedb_cgo -lm -ldl -lpthread"
          - os_arch: darwin-arm64
            runner: macos-14
            cgo_ldflags: "-L$HOME/go/lib/lancedb/darwin-arm64 -llancedb_cgo -lm -ldl -lresolv -framework CoreFoundation -framework Security -framework SystemConfiguration"

    steps:
      - uses: actions/checkout@v4
      
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'

      - name: Install LanceDB native library
        run: go run github.com/aqua777/go-lancedb/cmd/lancedb-install@latest

      - name: Build
        env:
          CGO_LDFLAGS: ${{ matrix.cgo_ldflags }}
        run: go build ./...
```

### GitLab CI

```yaml
build:
  image: golang:1.21
  before_script:
    - go run github.com/aqua777/go-lancedb/cmd/lancedb-install@latest
  variables:
    CGO_LDFLAGS: "-L/go/lib/lancedb/linux-amd64 -llancedb_cgo -lm -ldl -lpthread"
  script:
    - go build ./...
    - go test ./...
```

## Troubleshooting

### "cannot find -llancedb_cgo"

The native library isn't installed or `CGO_LDFLAGS` isn't set. Run:

```bash
# Install the library
go run github.com/aqua777/go-lancedb/cmd/lancedb-install@latest

# Set CGO_LDFLAGS (use the output from the installer)
export CGO_LDFLAGS="..."

# Retry build
go build ./...
```

### "file too small" or linker errors after install

The installer might have downloaded a Git LFS pointer file instead of the actual library. Check the release assets exist:

```bash
# Check file size (should be 40-100+ MB)
ls -la $GOPATH/lib/lancedb/*/liblancedb_cgo.a

# If tiny, delete and re-download
rm -rf $GOPATH/lib/lancedb
go run github.com/aqua777/go-lancedb/cmd/lancedb-install@latest
```

### CGO Not Enabled

```bash
export CGO_ENABLED=1
go build
```

### Building from Source (Alternative)

If the installer doesn't work for your platform, build from source:

```bash
git clone https://github.com/aqua777/go-lancedb.git
cd go-lancedb
./scripts/build-prebuilt-libs.sh
```

Then use a `replace` directive in your `go.mod`:

```go
replace github.com/aqua777/go-lancedb => /path/to/go-lancedb
```

## API Reference

See [README.md](README.md) for API documentation and [examples/](examples/) for working code.
