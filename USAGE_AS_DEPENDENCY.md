# Using go-lancedb as a Dependency

This package uses static linking - your Go binary includes everything it needs. No runtime library dependencies, no copying `.dylib` or `.so` files around.

## Quick Start

```bash
# Step 1: Add the dependency
go get github.com/aqua777/go-lancedb

# Step 2: Install the native library
go run github.com/aqua777/go-lancedb/cmd/lancedb-install@latest

# Step 3: Set PKG_CONFIG_PATH (the installer outputs the exact command)
# Example:
export PKG_CONFIG_PATH="$PKG_CONFIG_PATH:$HOME/go/lib/pkgconfig"

# Step 4: Build your app
go build -o myapp .
```

Add the `PKG_CONFIG_PATH` export to your `~/.bashrc` or `~/.zshrc` so it persists across sessions.

## Why Two Steps?

The native library (`liblancedb_cgo.a`) is 100MB+ and stored with Git LFS. Unfortunately, `go get` doesn't fetch Git LFS files - you'd just get a 133-byte pointer file.

The installer downloads the real library from GitHub Releases to `$GOPATH/lib/lancedb/` and generates a `pkg-config` file to help Go link against it.

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
- Generates a `.pc` file in `$GOPATH/lib/pkgconfig/`
- Tells you exactly what `PKG_CONFIG_PATH` to set

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
- `pkg-config` installed (often available by default or via `brew install pkg-config` / `apt install pkg-config`)
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

# Install pkg-config
RUN apt-get update && apt-get install -y pkg-config

# Install the native library
RUN go run github.com/aqua777/go-lancedb/cmd/lancedb-install@latest

# Set PKG_CONFIG_PATH for Linux (GOPATH defaults to /go in golang image)
ENV PKG_CONFIG_PATH="/go/lib/pkgconfig"

COPY . .
RUN go build -o myapp .

FROM debian:bookworm-slim
COPY --from=builder /app/myapp /usr/local/bin/
CMD ["myapp"]
```

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

      - name: Install pkg-config
        run: sudo apt-get update && sudo apt-get install -y pkg-config

      - name: Install LanceDB native library
        run: go run github.com/aqua777/go-lancedb/cmd/lancedb-install@latest

      - name: Build
        env:
          PKG_CONFIG_PATH: "$HOME/go/lib/pkgconfig"
        run: go build ./...

      - name: Test
        env:
          PKG_CONFIG_PATH: "$HOME/go/lib/pkgconfig"
        run: go test ./...
```

### GitLab CI

```yaml
build:
  image: golang:1.21
  before_script:
    - apt-get update && apt-get install -y pkg-config
    - go run github.com/aqua777/go-lancedb/cmd/lancedb-install@latest
  variables:
    PKG_CONFIG_PATH: "/go/lib/pkgconfig"
  script:
    - go build ./...
    - go test ./...
```

## Troubleshooting

### "pkg-config: exec: \"pkg-config\": executable file not found in $PATH"

You need to install `pkg-config`.
- macOS: `brew install pkg-config`
- Ubuntu/Debian: `sudo apt-get install pkg-config`

### "Package lancedb was not found in the pkg-config search path"

You haven't set `PKG_CONFIG_PATH` correctly.
Run `go run github.com/aqua777/go-lancedb/cmd/lancedb-install@latest` again and copy the export command.

### "cannot find -llancedb_cgo"

The native library isn't installed or `pkg-config` isn't working. Verify:
```bash
pkg-config --libs lancedb
# Should output: -L... -llancedb_cgo ...
```
If it outputs nothing or an error, your `PKG_CONFIG_PATH` is wrong.
