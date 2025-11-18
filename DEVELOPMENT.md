# Development Guide

This guide covers building, testing, and contributing to the LanceDB Go bindings.

## Architecture

The bindings consist of two components:

1. **Rust CGO Library** (`rust-cgo/`): A C-compatible library that exposes LanceDB functionality through C FFI
2. **Go Package** (`go-lancedb/`): Go wrapper code that uses CGO to call the Rust library

## Requirements

- **Go**: 1.21 or later
- **Rust**: 1.70 or later
- **C Compiler**: gcc, clang, or MSVC (platform-dependent)

## Building from Source

### Quick Build

```bash
# Build Rust CGO library
cd rust-cgo
cargo build --release

# Build Go package
cd ../go-lancedb
go build .

# Build examples
cd examples/basic
go build main.go
```

### Using Make

```bash
# Build everything
make build

# Build only Rust component
make build-rust

# Build only Go component
make build-go

# Clean all artifacts
make clean
```

### Makefile Targets

| Target | Description |
|--------|-------------|
| `make build` | Build both Rust and Go components |
| `make build-rust` | Build only the Rust CGO library |
| `make build-go` | Build only the Go package |
| `make test` | Run Go tests |
| `make clean` | Clean all build artifacts |
| `make deps` | Update dependencies |

## Testing

### Run All Tests

```bash
cd go-lancedb
go test -v .
```

### Run Specific Tests

```bash
# Vector search tests
go test -v -run TestVectorSearch

# Connection tests
go test -v -run TestConnection

# Index tests
go test -v -run TestIndex
```

### Run Benchmarks

```bash
cd go-lancedb
go test -bench=. -benchmem
```

## Cross-Compilation

### Build for Different Platforms

```bash
# Linux x86_64
make build-linux-amd64

# Linux ARM64
make build-linux-arm64

# macOS Intel
make build-darwin-amd64

# macOS Apple Silicon
make build-darwin-arm64

# Windows x86_64
make build-windows-amd64
```

### Cross-Compilation Notes

- Rust cross-compilation requires installing target toolchains
- Go cross-compilation with CGO requires C cross-compilers
- See Rust documentation for setting up cross-compilation targets

## Dependency Management

### Rust Dependencies

The Rust component uses these key dependencies (locked for version 0.10.0):

```toml
lancedb = "0.10.0"
lance = "0.17.0"
arrow = "52.2"
arrow-array = "52.2"
arrow-schema = "52.2"
snafu = "0.7.5"
```

**Important**: These versions must be kept in sync. The `lance` and `arrow` versions are dictated by `lancedb 0.10.0`.

### Go Dependencies

```bash
# Update Go dependencies
cd go-lancedb
go mod tidy
go mod vendor  # optional: vendor dependencies
```

## Project Structure

```
lancedb/
├── rust-cgo/                 # Rust CGO library
│   ├── src/
│   │   ├── lib.rs           # Main library entry
│   │   ├── connection.rs    # Connection management
│   │   ├── table.rs         # Table operations
│   │   ├── query.rs         # Query builder
│   │   ├── error.rs         # Error handling
│   │   └── arrow_ffi.rs     # Arrow C FFI integration
│   ├── Cargo.toml           # Rust dependencies
│   └── target/release/      # Build output (liblancedb_cgo.dylib/.so/.dll)
│
├── go-lancedb/              # Go package
│   ├── lancedb.go           # Main API and CGO declarations
│   ├── arrow.go             # Arrow integration
│   ├── query.go             # Query builder
│   ├── *_test.go            # Test files
│   └── examples/            # Example programs
│       ├── basic/
│       └── vector_search_with_index/
│
├── README.md                # User documentation
├── DEVELOPMENT.md           # This file
├── IMPLEMENTATION_STATUS.md # Feature tracking
└── Makefile                 # Build automation
```

## Contributing

### Adding New Features

1. **Add C API in Rust** (`rust-cgo/src/`)
   - Implement the feature in Rust
   - Export C-compatible functions with `#[no_mangle]` and `extern "C"`
   - Add proper error handling

2. **Update CGO Declarations** (`go-lancedb/lancedb.go`)
   - Add C function declarations in the `import "C"` block
   - Update CGO compiler/linker flags if needed

3. **Implement Go Wrappers** (`go-lancedb/*.go`)
   - Create idiomatic Go API wrapping the C functions
   - Add proper error handling and memory management
   - Ensure `Close()` methods are implemented for cleanup

4. **Add Tests** (`go-lancedb/*_test.go`)
   - Write comprehensive unit tests
   - Add integration tests
   - Update benchmarks if performance-sensitive

5. **Update Documentation**
   - Update README.md with usage examples
   - Add comments to exported functions
   - Update IMPLEMENTATION_STATUS.md

### Code Style

**Rust**:
- Follow standard Rust formatting (`cargo fmt`)
- Use `cargo clippy` for linting
- Add documentation comments for public functions

**Go**:
- Follow Go conventions (`gofmt`, `go vet`)
- Use meaningful variable names
- Document all exported functions and types
- Handle errors explicitly

### Debugging

#### Enable CGO Debugging

```bash
# Show CGO compiler commands
CGO_CFLAGS="-g -O0" go build -x

# Run with CGO tracing
GODEBUG=cgocheck=2 go run main.go
```

#### Rust Debug Builds

```bash
# Build with debug info
cd rust-cgo
cargo build  # debug build (much larger, slower)

# Update Go to use debug build
# Edit go-lancedb/lancedb.go CGO flags to use ../rust-cgo/target/debug
```

#### Memory Leak Detection

```bash
# Use valgrind (Linux)
valgrind --leak-check=full ./your_program

# Use AddressSanitizer
CGO_CFLAGS="-fsanitize=address" CGO_LDFLAGS="-fsanitize=address" go build
```

## Troubleshooting

### Build Issues

**"cannot find -llancedb_cgo"**
- Ensure the Rust library is built: `cd rust-cgo && cargo build --release`
- Check that the dylib/so/dll exists in `rust-cgo/target/release/`

**"version conflict" in Cargo**
- The dependencies must match lancedb 0.10.0 requirements
- Don't manually update arrow or lance versions
- If stuck, run: `cd rust-cgo && rm Cargo.lock && cargo build --release`

**CGO compiler errors**
- Ensure a C compiler is installed (gcc, clang, MSVC)
- On macOS: `xcode-select --install`
- On Linux: `sudo apt install build-essential`
- On Windows: Install MinGW-w64 or MSVC

### Runtime Issues

**"dyld: Library not loaded" (macOS)**
- The dylib must be findable at runtime
- Set `DYLD_LIBRARY_PATH` or use `install_name_tool`
- Or copy the dylib to your executable directory

**"cannot open shared object file" (Linux)**
- Set `LD_LIBRARY_PATH`: `export LD_LIBRARY_PATH=/path/to/rust-cgo/target/release:$LD_LIBRARY_PATH`

**"The specified module could not be found" (Windows)**
- Copy the DLL next to your executable
- Or add the directory to your PATH

### Performance Issues

- **Use release builds**: Debug builds are 10-100x slower
- **Enable optimizations**: Rust release builds use `-O3` by default
- **Profile before optimizing**: Use Go's pprof and Rust's flamegraphs
- **Batch operations**: Insert data in batches, not row-by-row

## Release Process

1. Update version numbers in:
   - `rust-cgo/Cargo.toml`
   - `go-lancedb/go.mod` (if applicable)

2. Update IMPLEMENTATION_STATUS.md with changes

3. Build and test all platforms:
   ```bash
   make clean
   make build
   make test
   ```

4. Tag the release:
   ```bash
   git tag -a v0.x.x -m "Release v0.x.x"
   git push origin v0.x.x
   ```

5. Build release artifacts for all platforms

6. Update documentation and examples

## Getting Help

- Check existing tests for usage patterns
- Review the LanceDB Rust documentation
- Look at the C FFI Arrow documentation
- Open an issue on GitHub for bugs or feature requests

