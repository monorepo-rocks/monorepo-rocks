.PHONY: all build build-all test clean install-deps \
	test-unit test-integration test-backends test-embedders test-fusion test-mcp test-watcher test-performance \
	test-all test-coverage test-coverage-integration benchmark benchmark-all \
	test-zoekt test-faiss test-quick test-slow

# Binary name
BINARY_NAME=mcpce
MAIN_PATH=./src/go/main.go
BIN_DIR=./bin

# Build flags
LDFLAGS=-ldflags "-s -w"
BUILD_FLAGS=-trimpath

# Default target
all: build

# Help
help:
	@echo "MCP Context Engine - Build Targets"
	@echo ""
	@echo "Build targets:"
	@echo "  make build         - Build with real FAISS support (requires 'brew install faiss')"
	@echo "  make build-stub    - Build with stub implementations (no external dependencies)"
	@echo "  make build-all     - Build for all platforms (stub mode)"
	@echo "  make dev           - Development build with debug symbols"
	@echo ""
	@echo "Test targets:"
	@echo "  make test          - Run all tests"
	@echo "  make test-faiss    - Run FAISS-specific tests"
	@echo "  make test-help     - Show detailed test options"
	@echo ""
	@echo "Other:"
	@echo "  make clean         - Remove build artifacts"
	@echo "  make fmt           - Format code"
	@echo "  make lint          - Run linters"
	@echo ""
	@echo "Prerequisites for FAISS support:"
	@echo "  brew install faiss"
	@echo ""

# Install Go dependencies
install-deps:
	go mod download
	go mod tidy

# Build for current platform with FAISS support (requires FAISS C library)
build: install-deps
	@echo "Building for current platform with FAISS support..."
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=1 CGO_CFLAGS="-I/opt/homebrew/include" CGO_LDFLAGS="-L/opt/homebrew/lib -lfaiss_c" \
		go build $(BUILD_FLAGS) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME) $(MAIN_PATH)

# Build with stub implementations (no external dependencies)
build-stub: install-deps
	@echo "Building with stub implementations..."
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 go build $(BUILD_FLAGS) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME)-stub $(MAIN_PATH)

# Build for all platforms
build-all: install-deps
	@echo "Building for all platforms..."
	@mkdir -p $(BIN_DIR)
	
	# Darwin (macOS) AMD64
	GOOS=darwin GOARCH=amd64 go build $(BUILD_FLAGS) $(LDFLAGS) \
		-o $(BIN_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_PATH)
	
	# Darwin (macOS) ARM64
	GOOS=darwin GOARCH=arm64 go build $(BUILD_FLAGS) $(LDFLAGS) \
		-o $(BIN_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_PATH)
	
	# Linux AMD64
	GOOS=linux GOARCH=amd64 go build $(BUILD_FLAGS) $(LDFLAGS) \
		-o $(BIN_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PATH)
	
	# Linux ARM64
	GOOS=linux GOARCH=arm64 go build $(BUILD_FLAGS) $(LDFLAGS) \
		-o $(BIN_DIR)/$(BINARY_NAME)-linux-arm64 $(MAIN_PATH)
	
	# Windows AMD64
	GOOS=windows GOARCH=amd64 go build $(BUILD_FLAGS) $(LDFLAGS) \
		-o $(BIN_DIR)/$(BINARY_NAME)-windows-amd64.exe $(MAIN_PATH)
	
	@echo "Build complete. Binaries in $(BIN_DIR)/"

# Run tests
test:
	go test -v ./...

# Run tests with coverage
test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Clean build artifacts
clean:
	rm -rf $(BIN_DIR)
	rm -f coverage.out coverage.html

# Development build (with debug symbols)
dev: install-deps
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(BINARY_NAME) $(MAIN_PATH)

# Run the binary
run: build
	$(BIN_DIR)/$(BINARY_NAME)

# Format code
fmt:
	go fmt ./...

# Lint code
lint:
	golangci-lint run

# Check for security issues
sec:
	gosec ./...

# === Testing Targets ===

# Run only unit tests (excluding integration tests)
test-unit:
	@echo "Running unit tests..."
	go test -v -short ./src/go/...

# Run all integration tests
test-integration:
	@echo "Running integration tests..."
	go test -v -tags=integration ./test/integration/...

# Run backend integration tests
test-backends:
	@echo "Running backend integration tests..."
	go test -v ./test/integration/backends/...

# Run embedder integration tests  
test-embedders:
	@echo "Running embedder integration tests..."
	go test -v ./test/integration/embedders/...

# Run fusion ranking integration tests
test-fusion:
	@echo "Running fusion ranking integration tests..."
	go test -v ./test/integration/fusion/...

# Run MCP protocol integration tests
test-mcp:
	@echo "Running MCP protocol integration tests..."
	go test -v ./test/integration/mcp/...

# Run file watcher integration tests
test-watcher:
	@echo "Running file watcher integration tests..."
	go test -v ./test/integration/watcher/...

# Run performance tests
test-performance:
	@echo "Running performance tests..."
	go test -v ./test/integration/performance/...

# Run specific backend tests
test-zoekt:
	@echo "Running Zoekt backend tests..."
	go test -v ./test/integration/backends/ -run ".*Zoekt.*"

test-faiss:
	@echo "Running FAISS backend tests..."
	go test -v ./test/integration/backends/ -run ".*FAISS.*"

# Run all tests (unit + integration)
test-all: test-unit test-integration

# Run quick tests only (short mode)
test-quick:
	@echo "Running quick tests..."
	go test -v -short ./...

# Run comprehensive tests (may take longer)
test-slow:
	@echo "Running comprehensive tests..."
	go test -v -timeout=30m ./...

# === Coverage Targets ===

# Run tests with coverage (unit tests only)
test-coverage:
	@echo "Running unit tests with coverage..."
	go test -v -short -coverprofile=coverage.out ./src/go/...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run integration tests with coverage
test-coverage-integration:
	@echo "Running integration tests with coverage..."
	go test -v -coverprofile=coverage-integration.out ./test/integration/...
	go tool cover -html=coverage-integration.out -o coverage-integration.html
	@echo "Integration coverage report generated: coverage-integration.html"

# Run all tests with coverage
test-coverage-all:
	@echo "Running all tests with coverage..."
	go test -v -coverprofile=coverage-all.out ./...
	go tool cover -html=coverage-all.out -o coverage-all.html
	@echo "Complete coverage report generated: coverage-all.html"

# === Benchmark Targets ===

# Run performance benchmarks
benchmark:
	@echo "Running performance benchmarks..."
	go test -v -bench=. -benchmem ./test/integration/performance/

# Run all benchmarks (including unit test benchmarks)
benchmark-all:
	@echo "Running all benchmarks..."
	go test -v -bench=. -benchmem ./...

# Run benchmarks with CPU profiling
benchmark-cpu:
	@echo "Running benchmarks with CPU profiling..."
	go test -v -bench=. -benchmem -cpuprofile=cpu.prof ./test/integration/performance/
	@echo "CPU profile saved to cpu.prof"

# Run benchmarks with memory profiling
benchmark-mem:
	@echo "Running benchmarks with memory profiling..."
	go test -v -bench=. -benchmem -memprofile=mem.prof ./test/integration/performance/
	@echo "Memory profile saved to mem.prof"

# === Environment-specific Targets ===

# Run tests with real backends (requires environment setup)
test-real-backends:
	@echo "Running tests with real backends..."
	@echo "Note: This requires FAISS and Zoekt to be properly installed"
	ZOEKT_USE_STUB=false go test -v ./test/integration/backends/

# Run tests with stub backends (no external dependencies)
test-stub-backends:
	@echo "Running tests with stub backends..."
	ZOEKT_USE_STUB=true SKIP_ONNX_TESTS=true go test -v ./test/integration/backends/

# Run MCP integration tests (requires binary build)
test-mcp-integration:
	@echo "Building binary for MCP integration tests..."
	@$(MAKE) build
	@echo "Running MCP integration tests..."
	go test -v ./test/integration/mcp/

# Skip MCP integration tests
test-no-mcp:
	@echo "Running all tests except MCP integration..."
	SKIP_MCP_INTEGRATION=true go test -v ./test/integration/...

# === CI/CD Targets ===

# Run tests suitable for CI environment
test-ci:
	@echo "Running CI tests..."
	go test -v -short -race ./src/go/...
	ZOEKT_USE_STUB=true SKIP_ONNX_TESTS=true SKIP_MCP_INTEGRATION=true go test -v ./test/integration/...

# Run tests with race detection
test-race:
	@echo "Running tests with race detection..."
	go test -v -race ./...

# Run tests with verbose output and timeout
test-verbose:
	@echo "Running tests with verbose output..."
	go test -v -timeout=30m ./...

# === Utility Targets ===

# Clean test artifacts
clean-test:
	@echo "Cleaning test artifacts..."
	rm -f coverage*.out coverage*.html
	rm -f cpu.prof mem.prof
	rm -rf test/integration/*/tmp*
	rm -rf /tmp/zoekt-*
	rm -rf /tmp/faiss-*
	rm -rf /tmp/mcp-*

# Setup test environment
setup-test:
	@echo "Setting up test environment..."
	@mkdir -p test/integration/fixtures
	go mod download
	go mod tidy

# Validate test setup
validate-test:
	@echo "Validating test setup..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		echo "✓ golangci-lint available"; \
	else \
		echo "⚠ golangci-lint not available (optional)"; \
	fi
	@if go version | grep -q "go1."; then \
		echo "✓ Go version: $$(go version)"; \
	else \
		echo "✗ Go not available"; \
		exit 1; \
	fi
	@echo "Test environment validation complete"

# Show test help
test-help:
	@echo "Available test targets:"
	@echo "  test-unit              - Run unit tests only"
	@echo "  test-integration       - Run integration tests only"
	@echo "  test-all              - Run all tests"
	@echo "  test-backends         - Run backend integration tests"
	@echo "  test-embedders        - Run embedder integration tests"
	@echo "  test-fusion           - Run fusion ranking tests"
	@echo "  test-mcp              - Run MCP protocol tests"
	@echo "  test-watcher          - Run file watcher tests"
	@echo "  test-performance      - Run performance tests"
	@echo "  test-zoekt            - Run Zoekt-specific tests"
	@echo "  test-faiss            - Run FAISS-specific tests"
	@echo "  test-quick            - Run quick tests (short mode)"
	@echo "  test-slow             - Run comprehensive tests"
	@echo "  test-coverage         - Run tests with coverage"
	@echo "  test-ci               - Run CI-suitable tests"
	@echo "  benchmark             - Run performance benchmarks"
	@echo "  clean-test            - Clean test artifacts"