# FAISS Integration

This document describes the FAISS (Facebook AI Similarity Search) integration in the MCP Context Engine.

## Overview

The MCP Context Engine supports two vector indexing implementations:

1. **Stub Implementation** (default): A pure Go implementation for development and testing
2. **Real FAISS Implementation**: Uses the FAISS C++ library for production-grade vector search

## Implementation Details

### Architecture

- **Interface**: `FAISSIndexer` defines the common interface for both implementations
- **Factory Function**: `NewFAISSIndexer()` creates the appropriate implementation based on environment variables and system capabilities
- **Build Constraints**: The real FAISS implementation requires CGO and is conditionally compiled

### Files

- `faiss.go` - Interface definition and stub implementation
- `faiss_real.go` - Real FAISS implementation (CGO required)
- `faiss_stub.go` - Stub fallback for non-CGO builds
- `faiss_test.go` - Common tests for both implementations
- `faiss_real_test.go` - Tests specific to real FAISS implementation
- `faiss_env_test.go` - Tests for environment variable behavior

## Usage

### Environment Variables

- `FAISS_USE_REAL=true` - Use real FAISS implementation (requires proper setup)
- `FAISS_USE_REAL=false` or unset - Use stub implementation (default)

### Basic Usage

```go
// Create indexer (automatically chooses implementation)
indexer := NewFAISSIndexer("/path/to/index", 768)
defer indexer.Close()

// Add vectors
embeddings := []types.Embedding{
    {ChunkID: "chunk1", Vector: vector1},
    {ChunkID: "chunk2", Vector: vector2},
}
err := indexer.AddVectors(ctx, embeddings)

// Search
options := VectorSearchOptions{MinScore: 0.5}
results, err := indexer.Search(ctx, queryVector, 10, options)

// Save/load index
err = indexer.Save(ctx, "/path/to/index.faiss")
err = indexer.Load(ctx, "/path/to/index.faiss")
```

## Real FAISS Setup

### Prerequisites

To use the real FAISS implementation, you need:

1. **FAISS C++ Library**: Install FAISS with C API support
2. **CGO**: Enable CGO compilation (`CGO_ENABLED=1`)
3. **Build Tools**: C++ compiler and development headers

### Installing FAISS

#### Ubuntu/Debian

```bash
# Install dependencies
sudo apt-get update
sudo apt-get install -y build-essential cmake libblas-dev liblapack-dev

# Clone and build FAISS
git clone https://github.com/facebookresearch/faiss.git
cd faiss

# Configure with C API
cmake -B build \
  -DFAISS_ENABLE_GPU=OFF \
  -DFAISS_ENABLE_C_API=ON \
  -DBUILD_SHARED_LIBS=ON \
  .

# Build and install
make -C build -j$(nproc)
sudo make -C build install

# Install dynamic library
sudo cp build/c_api/libfaiss_c.so /usr/lib/
sudo ldconfig
```

#### macOS

```bash
# Install dependencies
brew install cmake openblas

# Clone and build FAISS
git clone https://github.com/facebookresearch/faiss.git
cd faiss

# Configure with C API
cmake -B build \
  -DFAISS_ENABLE_GPU=OFF \
  -DFAISS_ENABLE_C_API=ON \
  -DBUILD_SHARED_LIBS=ON \
  -DOpenMP_CXX_FLAGS="-Xpreprocessor -fopenmp -I/usr/local/include" \
  -DOpenMP_C_FLAGS="-Xpreprocessor -fopenmp -I/usr/local/include" \
  .

# Build and install
make -C build -j$(sysctl -n hw.ncpu)
sudo make -C build install

# Install dynamic library
sudo cp build/c_api/libfaiss_c.dylib /usr/local/lib/
```

#### Docker

```dockerfile
FROM ubuntu:22.04

RUN apt-get update && apt-get install -y \
    build-essential \
    cmake \
    libblas-dev \
    liblapack-dev \
    git \
    golang-1.21

# Install FAISS
RUN git clone https://github.com/facebookresearch/faiss.git /tmp/faiss && \
    cd /tmp/faiss && \
    cmake -B build -DFAISS_ENABLE_GPU=OFF -DFAISS_ENABLE_C_API=ON -DBUILD_SHARED_LIBS=ON . && \
    make -C build -j$(nproc) && \
    make -C build install && \
    cp build/c_api/libfaiss_c.so /usr/lib/ && \
    ldconfig && \
    rm -rf /tmp/faiss

# Build application with real FAISS
ENV CGO_ENABLED=1
ENV FAISS_USE_REAL=true
```

### Building with Real FAISS

```bash
# Enable CGO and build
CGO_ENABLED=1 go build -tags cgo ./...

# Run with real FAISS
FAISS_USE_REAL=true CGO_ENABLED=1 go run -tags cgo ./cmd/your-app
```

### Testing

```bash
# Test stub implementation (default)
go test ./src/go/indexer

# Test with real FAISS (requires FAISS installation)
CGO_ENABLED=1 FAISS_USE_REAL=true go test -tags cgo ./src/go/indexer
```

## Performance Considerations

### Stub Implementation

- **Pros**: No external dependencies, fast builds, cross-platform
- **Cons**: Limited performance for large datasets, memory usage
- **Use Cases**: Development, testing, small datasets (<10k vectors)

### Real FAISS Implementation

- **Pros**: Highly optimized, supports large datasets, multiple index types
- **Cons**: CGO dependency, complex setup, platform-specific
- **Use Cases**: Production, large datasets (>10k vectors), performance-critical applications

### Index Types

The current implementation uses:

- **IndexFlatL2**: Exhaustive search with L2 distance
- **Vector Normalization**: Converts L2 distance to cosine similarity
- **Memory Mapping**: Efficient index loading

Future enhancements could include:

- **IndexHNSWFlat**: Approximate nearest neighbor search for speed
- **IndexIVFFlat**: Inverted file index for memory efficiency
- **GPU Support**: CUDA acceleration for large-scale searches

## Vector Operations

### Distance Metrics

- **Cosine Similarity**: Primary metric used in the interface
- **L2 Distance**: Internal FAISS representation
- **Conversion Formula**: `cosine_similarity = 1 - (l2_distance² / 2)` for normalized vectors

### Vector Normalization

All vectors are automatically normalized to unit length for consistent cosine similarity computation:

```go
// Normalization ensures cosine similarity works correctly
func normalizeVector(vector []float32) []float32 {
    norm := sqrt(sum(v² for v in vector))
    return [v/norm for v in vector]
}
```

### Index Persistence

- **FAISS Format**: Native binary format for index data
- **Metadata**: JSON format for chunk ID mappings and statistics
- **Files**: `index.faiss` (FAISS data) + `index.faiss.meta` (metadata)

## Error Handling

### Graceful Fallbacks

1. **Missing FAISS Library**: Falls back to stub implementation with warning
2. **CGO Disabled**: Uses stub implementation automatically
3. **Index Corruption**: Returns clear error messages
4. **Memory Issues**: Handled by FAISS internal error management

### Common Issues

1. **"faiss/c_api/index_io_c.h not found"**
   - Solution: Install FAISS C++ library with C API support

2. **"libfaiss_c.so not found"**
   - Solution: Copy shared library to system path and run `ldconfig`

3. **CGO compilation errors**
   - Solution: Ensure `CGO_ENABLED=1` and C++ compiler available

4. **Dimension mismatch errors**
   - Solution: Ensure all vectors have consistent dimensions (default: 768)

## Migration from Stub to Real FAISS

1. **Install FAISS library** following the setup instructions above
2. **Set environment variable**: `FAISS_USE_REAL=true`
3. **Enable CGO**: `CGO_ENABLED=1`
4. **Rebuild application**: `go build -tags cgo`
5. **Test thoroughly**: Verify vector operations work correctly

Existing indexes in stub format cannot be directly loaded by real FAISS implementation. You'll need to re-index your data.

## Monitoring and Debugging

### Index Statistics

```go
stats := indexer.VectorStats()
fmt.Printf("Vectors: %d, Dimension: %d, Size: %d bytes, Updated: %v\n",
    stats.TotalVectors, stats.Dimension, stats.IndexSize, stats.LastUpdated)
```

### Logging

The implementation provides detailed logging for:
- Index creation and loading
- Vector operations (add, search, delete)
- Fallback scenarios
- Performance metrics

### Performance Metrics

Monitor these key metrics:
- **Search Latency**: Time per k-NN query
- **Index Size**: Memory/disk usage
- **Throughput**: Vectors indexed per second
- **Accuracy**: Similarity score distributions