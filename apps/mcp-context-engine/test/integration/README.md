# Integration Test Suite

This directory contains comprehensive integration tests for the MCP Context Engine. The tests cover all major components and their interactions with real backend implementations.

## Test Structure

```
test/integration/
├── README.md                                    # This file
├── fixtures/                                    # Test data and fixtures
│   ├── sample_code.go                          # Go code samples
│   ├── sample_code.py                          # Python code samples  
│   ├── sample_code.js                          # JavaScript code samples
│   ├── config.yaml                             # Configuration file
│   └── test_data.json                          # Test metadata
├── backends/                                    # Backend integration tests
│   ├── zoekt_real_integration_test.go          # Real Zoekt backend tests
│   └── faiss_real_integration_test.go          # Real FAISS backend tests
├── embedders/                                  # Embedder integration tests
│   └── embedder_integration_test.go            # TF-IDF and ONNX embedder tests
├── fusion/                                     # Fusion ranking tests
│   └── enhanced_fusion_integration_test.go     # Enhanced fusion ranking tests
├── mcp/                                        # MCP protocol tests
│   └── mcp_protocol_integration_test.go        # MCP stdio protocol tests
├── watcher/                                    # File watcher tests
│   └── file_watcher_integration_test.go        # File watcher integration tests
└── performance/                                # Performance benchmarks
    └── benchmark_integration_test.go           # Performance and benchmark tests
```

## Test Categories

### 1. Backend Integration Tests (`backends/`)

Tests for the real backend implementations:

- **Zoekt Real Backend Tests**: Tests for the real Zoekt indexer implementation
  - Basic operations (index, search, delete)
  - Advanced search features (regex, language filtering, file patterns)
  - Incremental indexing and updates
  - Error handling and edge cases
  - Concurrency and performance

- **FAISS Real Backend Tests**: Tests for the real FAISS vector indexer
  - Vector addition and search operations
  - Dimension validation and error handling
  - Similarity calculations and ranking
  - Vector deletion and index rebuilding
  - Persistence (save/load operations)
  - Large-scale operations and performance

### 2. Embedder Integration Tests (`embedders/`)

Tests for text embedding implementations:

- **TF-IDF Embedder Tests**: 
  - Basic text embedding functionality
  - Vocabulary building across multiple texts
  - Batch processing capabilities
  - Caching behavior and performance
  - Cross-language code similarity

- **ONNX Embedder Tests**:
  - ONNX runtime availability and fallback
  - Semantic understanding capabilities
  - Performance comparison with TF-IDF
  - Error handling for missing models

- **Configuration and Fallback Tests**:
  - Default configuration handling
  - Invalid configuration graceful degradation
  - Performance comparison between embedders

### 3. Enhanced Fusion Ranking Tests (`fusion/`)

Tests for the fusion ranking system:

- **Basic Fusion Operations**:
  - Lexical and semantic result combination
  - Weight adjustment effects on ranking
  - Score normalization and consistency

- **Query Type Handling**:
  - Different query patterns (exact, conceptual, technical)
  - Language-specific queries
  - Complex multi-term queries

- **Performance and Edge Cases**:
  - Large result set handling
  - Concurrent search operations
  - Empty queries and error scenarios

### 4. File Watcher Integration Tests (`watcher/`)

Tests for file system monitoring:

- **File Operations Detection**:
  - File creation, modification, deletion events
  - Directory operations and subdirectory monitoring
  - Batch file operations

- **Integration with Indexers**:
  - Real-time indexing updates
  - Incremental re-indexing on modifications
  - Index cleanup on file deletions

- **Performance and Reliability**:
  - High-frequency file operations
  - Large file handling
  - Error recovery and robustness

### 5. MCP Protocol Integration Tests (`mcp/`)

Tests for the Model Context Protocol implementation:

- **Protocol Compliance**:
  - JSON-RPC 2.0 message formatting
  - Initialization and capability negotiation
  - Tool discovery and invocation

- **Search Operations**:
  - Code search through MCP interface
  - File indexing via MCP commands
  - Result formatting and error handling

- **Integration Modes**:
  - Stdio mode testing
  - Library mode testing
  - Concurrent request handling

### 6. Performance Benchmark Tests (`performance/`)

Comprehensive performance testing:

- **Backend Performance**:
  - Indexing throughput and latency
  - Search response times
  - Memory usage patterns

- **Scalability Testing**:
  - Large dataset handling
  - Concurrent operation performance
  - Resource utilization monitoring

- **Regression Testing**:
  - Performance baseline establishment
  - Automated performance regression detection
  - Cross-platform performance comparison

## Running Tests

### Quick Start

```bash
# Run all integration tests
make test-integration

# Run specific test categories
make test-backends
make test-embedders
make test-fusion
make test-mcp
make test-watcher
make test-performance
```

### Environment Setup

The tests can run in different configurations:

#### Stub Mode (Default)
Uses stub implementations for external dependencies:
```bash
make test-stub-backends
```

#### Real Backends Mode
Requires real FAISS and Zoekt installations:
```bash
make test-real-backends
```

#### CI Mode
Optimized for continuous integration environments:
```bash
make test-ci
```

### Environment Variables

- `ZOEKT_USE_STUB=true` - Use stub Zoekt implementation
- `SKIP_ONNX_TESTS=true` - Skip ONNX embedder tests
- `SKIP_MCP_INTEGRATION=true` - Skip MCP protocol tests
- `SKIP_PERFORMANCE_TESTS=true` - Skip performance benchmarks

### Test Targets

| Target | Description |
|--------|-------------|
| `test-unit` | Run unit tests only |
| `test-integration` | Run all integration tests |
| `test-all` | Run both unit and integration tests |
| `test-backends` | Run backend integration tests |
| `test-embedders` | Run embedder tests |
| `test-fusion` | Run fusion ranking tests |
| `test-mcp` | Run MCP protocol tests |
| `test-watcher` | Run file watcher tests |
| `test-performance` | Run performance tests |
| `test-zoekt` | Run Zoekt-specific tests |
| `test-faiss` | Run FAISS-specific tests |
| `test-quick` | Run quick tests (short mode) |
| `test-slow` | Run comprehensive tests |
| `test-coverage` | Run tests with coverage |
| `test-ci` | Run CI-suitable tests |
| `benchmark` | Run performance benchmarks |

## Test Data and Fixtures

### Test Files

The `fixtures/` directory contains sample code files in different languages:

- `sample_code.go` - Go authentication and user management code
- `sample_code.py` - Python user management with database operations  
- `sample_code.js` - JavaScript API client with authentication
- `config.yaml` - Configuration file with various settings
- `test_data.json` - Test metadata including expected search results

### Test Scenarios

The test data includes scenarios for:

- **Authentication Functions**: Testing search for auth-related code across languages
- **User Management**: Testing object-oriented patterns and database operations
- **Configuration Handling**: Testing configuration file parsing and search
- **API Integration**: Testing REST API patterns and client implementations

## Performance Benchmarks

### Benchmark Categories

1. **Indexing Performance**
   - Small dataset (10 files)
   - Medium dataset (100 files)  
   - Large dataset (500+ files)

2. **Search Performance**
   - Simple keyword queries
   - Complex regex patterns
   - Large result sets
   - Concurrent searches

3. **Vector Operations**
   - Vector addition throughput
   - Similarity search latency
   - Large-scale vector operations

4. **End-to-End Performance**
   - Complete indexing workflows
   - Fusion search operations
   - MCP protocol overhead

### Running Benchmarks

```bash
# Run all benchmarks
make benchmark

# Run with profiling
make benchmark-cpu
make benchmark-mem

# Performance regression testing
make test-performance
```

## Test Results and Reporting

### Coverage Reports

Generate coverage reports for different test categories:

```bash
make test-coverage                # Unit test coverage
make test-coverage-integration    # Integration test coverage  
make test-coverage-all           # Complete coverage
```

### Performance Reports

Performance tests generate detailed reports including:

- Operation throughput (ops/second)
- Memory usage patterns
- Response time distributions
- Resource utilization metrics

## Debugging and Troubleshooting

### Common Issues

1. **FAISS Tests Failing**
   - Ensure CGO is enabled: `export CGO_ENABLED=1`
   - Install FAISS development headers
   - Use stub mode: `make test-stub-backends`

2. **MCP Tests Timing Out**
   - Ensure binary builds successfully: `make build`
   - Check for port conflicts
   - Use skip flag: `SKIP_MCP_INTEGRATION=true make test-integration`

3. **File Watcher Tests Flaky**
   - Filesystem-dependent behavior
   - May need adjustment for different OS
   - Use shorter timeouts in CI

### Debug Mode

Run tests with verbose output:

```bash
make test-verbose
```

Enable race detection:

```bash
make test-race
```

### Test Environment Validation

Validate your test environment:

```bash
make validate-test
```

## Contributing

When adding new integration tests:

1. Follow the existing test structure and naming conventions
2. Include both positive and negative test cases
3. Add performance considerations for new features
4. Update this README with new test categories
5. Ensure tests work in both stub and real backend modes
6. Add appropriate environment variable controls

### Test Naming Convention

- Test files: `*_integration_test.go`
- Test functions: `Test<Component><Feature>Integration`
- Benchmark functions: `Benchmark<Component><Operation>`

### Test Organization

- Group related tests in the same file
- Use subtests (`t.Run()`) for test variations
- Include setup and cleanup functions
- Use table-driven tests for multiple scenarios

## CI/CD Integration

The integration tests are designed to work in CI environments:

- Use `make test-ci` for CI pipelines
- Tests automatically use stub implementations when real backends unavailable
- Performance tests have reasonable timeouts
- Coverage reports are generated in standard formats

For continuous integration, consider:

- Running `test-quick` on pull requests
- Running `test-all` on main branch commits
- Running `test-performance` periodically for regression detection
- Archiving coverage reports and performance metrics