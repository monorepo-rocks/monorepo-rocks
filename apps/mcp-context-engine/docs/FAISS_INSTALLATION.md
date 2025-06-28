# FAISS Installation Guide

This guide explains how to install and configure FAISS for use with the MCP Context Engine on macOS.

## Prerequisites

- macOS (Intel or Apple Silicon)
- Homebrew package manager
- Go 1.21 or later

## Installation Steps

### 1. Install FAISS via Homebrew

```bash
brew install faiss
```

This will install:
- FAISS C++ library
- FAISS C API headers (required for CGO)
- Dependencies (OpenBLAS, OpenMP)

### 2. Verify Installation

Check that FAISS headers are installed:

```bash
ls -la /opt/homebrew/include/faiss/c_api/
```

You should see files like `Index_c.h`, `IndexFlat_c.h`, etc.

### 3. Build MCP Context Engine with FAISS

```bash
# Build with real FAISS support (default)
make build

# Or explicitly with environment variables
CGO_ENABLED=1 \
CGO_CFLAGS="-I/opt/homebrew/include" \
CGO_LDFLAGS="-L/opt/homebrew/lib -lfaiss_c" \
go build ./src/go/main.go
```

### 4. Run with Real FAISS

Set environment variables to use real implementations:

```bash
export FAISS_USE_REAL=true
export ZOEKT_USE_REAL=true
./bin/mcpce serve
```

## Build Options

### With FAISS Support (Default)
```bash
make build
```
- Requires FAISS C library installed
- Provides high-performance vector search
- Uses CGO for C++ interop

### Stub Implementation (No Dependencies)
```bash
make build-stub
```
- No external dependencies required
- Uses in-memory Go implementation
- Good for development/testing

## Troubleshooting

### Build Errors

If you see errors like:
```
fatal error: 'faiss/c_api/IndexFlat_c.h' file not found
```

1. Ensure FAISS is installed: `brew install faiss`
2. Check your architecture: `uname -m`
3. Verify Homebrew paths:
   - Intel Mac: `/usr/local/`
   - Apple Silicon: `/opt/homebrew/`

### Runtime Errors

If the binary fails to load FAISS library:

1. Check library is installed:
   ```bash
   ls -la /opt/homebrew/lib/libfaiss_c.*
   ```

2. Set library path if needed:
   ```bash
   export DYLD_LIBRARY_PATH="/opt/homebrew/lib:$DYLD_LIBRARY_PATH"
   ```

### Performance Testing

Run FAISS-specific tests:
```bash
make test-faiss
```

Benchmark vector search performance:
```bash
make benchmark
```

## Architecture Support

- **Apple Silicon (M1/M2/M3)**: Full support via Homebrew ARM64 build
- **Intel Mac**: Full support via Homebrew x86_64 build
- **Cross-compilation**: Use `build-stub` for other platforms

## Environment Variables

- `FAISS_USE_REAL=true` - Use real FAISS implementation (default: auto-detect)
- `FAISS_USE_STUB=true` - Force stub implementation
- `CGO_ENABLED=1` - Enable CGO for FAISS C bindings
- `CGO_CFLAGS` - C compiler flags for finding headers
- `CGO_LDFLAGS` - Linker flags for finding libraries

## Next Steps

1. Index a repository:
   ```bash
   ./bin/mcpce index /path/to/repo
   ```

2. Start the MCP server:
   ```bash
   ./bin/mcpce stdio
   ```

3. Use the HTTP API:
   ```bash
   ./bin/mcpce serve --port 8080
   ```

For more information, see the [main README](../README.md).