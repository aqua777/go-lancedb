# Using This Package as a Dependency

## Current Status: **Requires Rust Compilation**

**Short answer**: No, it does **NOT** work out of the box. Users need to compile the Rust library.

## Why?

When someone uses this package as a dependency:

```bash
go get github.com/aqua777/lancedb/go-lancedb@v0.0.1
```

Go downloads the module to the module cache (e.g., `$GOPATH/pkg/mod/github.com/aqua777/lancedb/go-lancedb@v0.0.1/`).

The CGO linker looks for libraries in:
1. `${SRCDIR}/../libs/${GOOS}-${GOARCH}/` → **Empty** (no pre-built libs committed)
2. `${SRCDIR}/../rust-cgo/target/release/` → **Empty** (Rust not compiled)

**Result**: Linker fails with "cannot find -llancedb_cgo"

## Making It Work Out of the Box

### Option 1: Commit Pre-built Libraries (Recommended for linux/arm64 & darwin/arm64)

**Pros:**
- ✅ Works immediately for supported platforms
- ✅ No Rust toolchain needed for end users
- ✅ Faster builds

**Cons:**
- ⚠️ Binary files in git (consider Git LFS for large files)
- ⚠️ Only works for platforms you pre-build
- ⚠️ Repository size increases

**Steps:**

1. Build the libraries:
```bash
make build-prebuilt-libs
```

2. Commit them to git:
```bash
git add libs/linux-arm64/liblancedb_cgo.so
git add libs/darwin-arm64/liblancedb_cgo.dylib
git commit -m "Add pre-built libraries for linux/arm64 and darwin/arm64"
```

3. Users on linux/arm64 or darwin/arm64 can now use it directly:
```bash
go get github.com/aqua777/lancedb/go-lancedb@v0.0.1
go build  # Works!
```

4. Users on other platforms still need to build from source (see Option 2)

### Option 2: Include Rust Source + Build Instructions

**Pros:**
- ✅ Works for all platforms
- ✅ No binary bloat in git
- ✅ Users get latest optimizations

**Cons:**
- ⚠️ Requires Rust toolchain (1.70+)
- ⚠️ Slower first build
- ⚠️ More complex setup

**Steps:**

1. Ensure `rust-cgo/` directory is included in the repository
2. Document build requirements in README
3. Users must build before using:

```bash
go get github.com/aqua777/lancedb/go-lancedb@v0.0.1
cd $GOPATH/pkg/mod/github.com/aqua777/lancedb/go-lancedb@v0.0.1
cd ../rust-cgo
cargo build --release
cd ../go-lancedb
go build  # Now works
```

**Better approach**: Provide a build script that users can run:

```bash
# In their project
go get github.com/aqua777/lancedb/go-lancedb@v0.0.1
go run github.com/aqua777/lancedb/go-lancedb/scripts/build-rust-lib
go build
```

### Option 3: Hybrid Approach (Best of Both Worlds)

**Strategy:**
- Pre-build libraries for common platforms (linux/arm64, darwin/arm64) → commit to git
- Include Rust source for other platforms → users build themselves
- Provide clear documentation

**Implementation:**

1. Pre-build for linux/arm64 and darwin/arm64, commit to `libs/`
2. Keep `rust-cgo/` in repository for other platforms
3. Update README with platform-specific instructions

## Recommended Solution

For **linux/arm64** and **darwin/arm64** specifically:

1. **Commit pre-built libraries** to `libs/` directory
2. **Use Git LFS** if libraries are large (>100MB)
3. **Document** that other platforms need to build from source

### Git LFS Setup (if needed)

```bash
# Install Git LFS
git lfs install

# Track library files
git lfs track "libs/**/*.so"
git lfs track "libs/**/*.dylib"
git lfs track "libs/**/*.dll"

# Add and commit
git add .gitattributes
git add libs/
git commit -m "Add pre-built libraries via Git LFS"
```

### Update .gitignore

Make sure `.gitignore` allows `libs/` but still ignores `rust-cgo/target/`:

```gitignore
# Allow libs directory
!libs/
libs/**/*.so
libs/**/*.dylib
libs/**/*.dll

# But track via Git LFS (handled by .gitattributes)
```

Actually, if using Git LFS, don't ignore them - Git LFS handles it.

## Testing as a Dependency

To test if it works as a dependency:

```bash
# In a test project
mkdir /tmp/test-lancedb-dep
cd /tmp/test-lancedb-dep
go mod init test-lancedb

# Add dependency
go get github.com/aqua777/lancedb/go-lancedb@v0.0.1

# Try to build
go build  # Should work if libs/ are committed

# Check what's in module cache
ls -la $GOPATH/pkg/mod/github.com/aqua777/lancedb/go-lancedb@v0.0.1/../libs/
```

## Summary

| Approach | Works OOTB? | Platforms | User Requirements |
|----------|-------------|-----------|-------------------|
| **Pre-built libs committed** | ✅ Yes | linux/arm64, darwin/arm64 | None |
| **Rust source included** | ❌ No | All | Rust 1.70+, cargo |
| **Hybrid** | ✅ Partial | linux/arm64, darwin/arm64 = Yes<br>Others = No | Others need Rust |

**Recommendation**: Use the **hybrid approach** - commit pre-built libraries for linux/arm64 and darwin/arm64, and include Rust source for other platforms.

