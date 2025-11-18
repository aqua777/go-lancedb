# Making LanceDB Go Bindings a Standalone Repository

This document explains how to extract the `golang` directory into an independent repository.

## Current Architecture

```
lancedb/
├── rust/lancedb/              # Main LanceDB Rust library
└── golang/
    ├── rust-cgo/              # Rust CGO wrapper (currently path dep)
    └── cgo/                   # Go bindings
```

**Current Dependency** (`golang/rust-cgo/Cargo.toml` line 13):
```toml
lancedb = { path = "../../rust/lancedb", default-features = false }
```

This creates a **local path dependency** on the Rust source code.

## Making It Standalone

### Option 1: Use Published Crate (Recommended)

Change `golang/rust-cgo/Cargo.toml` to use the published crate:

```toml
# BEFORE (path dependency)
lancedb = { path = "../../rust/lancedb", default-features = false }

# AFTER (published dependency)
lancedb = { version = "0.10.0", default-features = false }
```

**Benefits**:
- ✅ No dependency on Rust source tree
- ✅ Uses stable, published versions
- ✅ Easier for users to integrate
- ✅ Faster builds (no need to compile LanceDB from source)

**Trade-off**:
- ⚠️ Must wait for LanceDB releases on crates.io
- ⚠️ Can't immediately use unreleased features

### Option 2: Use Git Dependency

Depend on the LanceDB GitHub repository:

```toml
lancedb = { 
    git = "https://github.com/lancedb/lancedb.git", 
    tag = "v0.10.0",
    default-features = false 
}
```

**Benefits**:
- ✅ No local path dependency
- ✅ Can pin to specific versions/tags
- ✅ Can use unreleased versions from main branch

**Trade-off**:
- ⚠️ Slower builds (compiles LanceDB from source)
- ⚠️ Network dependency during build

## Steps to Create Standalone Repository

### 1. Create New Repository

```bash
# Create new repo
mkdir lancedb-go
cd lancedb-go
git init

# Copy golang directory contents
cp -r /path/to/lancedb/golang/* .

# Directory structure will be:
# lancedb-go/
# ├── rust-cgo/
# ├── cgo/
# ├── scripts/
# ├── README.md
# ├── FEATURES.md
# └── ...
```

### 2. Update Cargo.toml

Edit `rust-cgo/Cargo.toml`:

```toml
[package]
name = "lancedb-cgo"
description = "CGO bindings for LanceDB"
version = "0.10.0"
edition = "2021"
license = "Apache-2.0"
publish = false  # Keep false unless publishing to crates.io

[lib]
crate-type = ["cdylib"]

[dependencies]
# CHANGE THIS LINE:
lancedb = { version = "0.10.0", default-features = false }

# Rest stays the same
lance = { version = "=1.0.0-beta.2", tag = "v1.0.0-beta.2", git = "https://github.com/lancedb/lance.git" }
arrow = { version = "56.2", features = ["ffi"] }
arrow-array = "56.2"
arrow-schema = "56.2"
tokio = "1.46"
snafu = "0.8"
lazy_static = "1"
serde = { version = "^1" }
serde_json = { version = "1" }
libc = "0.2"
futures = "0.3"

[features]
default = ["lancedb/default"]
```

### 3. Update Go Module Path

Edit `cgo/go.mod`:

```go
// BEFORE
module github.com/lancedb/lancedb/golang/cgo

// AFTER
module github.com/yourusername/lancedb-go
```

### 4. Update Documentation

Update all documentation to reflect the new repository:

- `README.md` - Change installation instructions
- `USAGE_AS_DEPENDENCY.md` - Update import paths
- Examples - Update import statements

**Before**:
```go
import "github.com/lancedb/lancedb/golang/cgo"
```

**After**:
```go
import "github.com/yourusername/lancedb-go"
```

### 5. Test the Standalone Build

```bash
# Build Rust library
cd rust-cgo
cargo build --release

# Should download lancedb from crates.io
# No path dependency needed!

# Build Go package
cd ../cgo
go build

# Run tests
go test -v
```

### 6. Update CI/CD

Your standalone repo won't need the Rust source tree:

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
        # No submodules needed!
      
      - name: Setup Rust
        uses: actions-rs/toolchain@v1
        with:
          toolchain: stable
      
      - name: Setup Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      
      - name: Build Rust CGO library
        run: |
          cd rust-cgo
          cargo build --release
        # Will download lancedb from crates.io
      
      - name: Build Go package
        run: |
          cd cgo
          go build -v ./...
      
      - name: Run tests
        run: |
          cd cgo
          go test -v ./...
```

## Advantages of Standalone Repository

### For Users

1. **Simpler Integration**
   ```bash
   go get github.com/yourusername/lancedb-go
   ```

2. **Clearer Documentation**
   - Focused solely on Go bindings
   - No confusion with Rust/Python/TypeScript docs

3. **Easier to Find**
   - Searchable as "lancedb-go" on GitHub
   - Clear Go-specific README

4. **Independent Releases**
   - Can release Go bindings independently
   - Don't need to wait for LanceDB core releases

### For Maintainers

1. **Independent Versioning**
   - Version Go bindings separately (v1.0.0, v1.1.0, etc.)
   - Not tied to LanceDB core versions

2. **Focused Issues/PRs**
   - Issues specific to Go bindings
   - No noise from other language bindings

3. **Separate CI/CD**
   - Faster builds (no full LanceDB compilation)
   - Go-specific workflows

4. **Easier Contribution**
   - Contributors don't need full LanceDB setup
   - Just Rust + Go

## Version Synchronization Strategy

### Approach 1: Manual Sync
Pin to specific LanceDB versions:

```toml
lancedb = { version = "=0.10.0", default-features = false }
```

Update when ready to adopt new LanceDB features.

### Approach 2: Compatible Versions
Allow minor updates:

```toml
lancedb = { version = "^0.10", default-features = false }
```

Automatically picks up bug fixes and features.

### Approach 3: Track Main Branch
Use latest development version:

```toml
lancedb = { 
    git = "https://github.com/lancedb/lancedb.git", 
    branch = "main",
    default-features = false 
}
```

⚠️ Warning: May break on LanceDB API changes!

## Recommended Repository Structure

```
lancedb-go/
├── .github/
│   └── workflows/
│       ├── build.yml
│       └── test.yml
├── rust-cgo/              # Rust CGO wrapper
│   ├── src/
│   ├── Cargo.toml
│   └── Cargo.lock
├── cgo/                   # Go bindings
│   ├── *.go
│   ├── *_test.go
│   ├── example/
│   ├── go.mod
│   └── go.sum
├── scripts/
│   └── setup-project.sh
├── docs/
│   ├── FEATURES.md
│   ├── USAGE_AS_DEPENDENCY.md
│   └── QUICK_REFERENCE.md
├── README.md
├── LICENSE
└── CHANGELOG.md
```

## Publishing Strategy

### 1. Go Module Publishing

Once standalone, users can use it directly:

```bash
go get github.com/yourusername/lancedb-go@v0.10.0
```

No special setup needed - Go modules work automatically with GitHub!

### 2. Pre-built Binaries (Optional)

For easier distribution, provide pre-built CGO libraries:

```
Releases/
├── v0.10.0/
│   ├── darwin-arm64/
│   │   └── liblancedb_cgo.dylib
│   ├── darwin-amd64/
│   │   └── liblancedb_cgo.dylib
│   ├── linux-amd64/
│   │   └── liblancedb_cgo.so
│   ├── linux-arm64/
│   │   └── liblancedb_cgo.so
│   └── windows-amd64/
│       └── lancedb_cgo.dll
```

Users can download pre-built libraries instead of compiling Rust.

### 3. Docker Images (Optional)

Provide ready-to-use Docker images:

```bash
docker pull yourusername/lancedb-go:0.10.0
```

## Migration Path for Existing Users

If this becomes standalone, existing users would need to update:

### Before (monorepo):
```go
import "github.com/lancedb/lancedb/golang/cgo"
```

### After (standalone):
```go
import "github.com/yourusername/lancedb-go"
```

**Migration Notice** in README:
```markdown
## ⚠️ Breaking Change: New Repository

LanceDB Go bindings have moved to a standalone repository.

**Old import** (deprecated):
`github.com/lancedb/lancedb/golang/cgo`

**New import**:
`github.com/yourusername/lancedb-go`

Update your code:
```bash
# Update import paths
go mod edit -replace github.com/lancedb/lancedb/golang/cgo=github.com/yourusername/lancedb-go@latest
go mod tidy
```

## Example: Complete Migration

```bash
# 1. Create standalone repo
git clone https://github.com/lancedb/lancedb.git
cd lancedb
cp -r golang ../lancedb-go
cd ../lancedb-go

# 2. Update Cargo.toml
sed -i 's|path = "../../rust/lancedb"|version = "0.10.0"|' rust-cgo/Cargo.toml

# 3. Update go.mod
cd cgo
go mod edit -module github.com/yourusername/lancedb-go
cd ..

# 4. Initialize new repo
git init
git add .
git commit -m "Initial commit: LanceDB Go bindings"

# 5. Test build
cd rust-cgo && cargo build --release
cd ../cgo && go build && go test -v

# 6. Push to GitHub
git remote add origin https://github.com/yourusername/lancedb-go.git
git push -u origin main

# 7. Tag release
git tag v0.10.0
git push origin v0.10.0
```

## Conclusion

**Yes, the `golang` directory can be completely independent!**

**Required Change**: Just update one line in `Cargo.toml`:
```toml
# From this:
lancedb = { path = "../../rust/lancedb", default-features = false }

# To this:
lancedb = { version = "0.10.0", default-features = false }
```

**Benefits**:
- ✅ Cleaner separation of concerns
- ✅ Easier for Go developers to discover and use
- ✅ Independent release cycles
- ✅ Focused documentation and issues
- ✅ No Rust source tree dependency

**Considerations**:
- Must keep in sync with LanceDB versions
- Need separate CI/CD setup
- May need migration path for existing users

---

**Recommendation**: Creating a standalone `lancedb-go` repository would be **beneficial** for the Go community and make the bindings more discoverable and easier to use.

