# Using go-lancedb as a Dependency

This package uses static linking - your Go binary includes everything it needs. No runtime library dependencies, no copying `.dylib` or `.so` files around.

## Quick Start

```bash
# Add the dependency
go get github.com/aqua777/go-lancedb

# Build your app - produces a single binary
go build -o myapp .

# Run it - just works
./myapp
```

That's it. The static library is linked at compile time.

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
      - run: go build -v ./...
      - run: go test -v ./...
```

### GitLab CI

```yaml
build:
  image: golang:1.21
  script:
    - go build -v ./...
    - go test -v ./...
```

## Troubleshooting

### "cannot find -llancedb_cgo"

The static library for your platform isn't available. Build from source:

```bash
git clone https://github.com/aqua777/go-lancedb.git
cd go-lancedb
./scripts/build-prebuilt-libs.sh
```

### CGO Not Enabled

```bash
export CGO_ENABLED=1
go build
```

### Git LFS Not Installed

The static libraries are stored with Git LFS. Install it:

```bash
# macOS
brew install git-lfs

# Ubuntu/Debian
apt install git-lfs

# Then pull the files
git lfs install
git lfs pull
```

## API Reference

See [README.md](README.md) for API documentation and [examples/](examples/) for working code.
