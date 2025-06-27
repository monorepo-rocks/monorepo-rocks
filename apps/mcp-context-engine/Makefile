.PHONY: all build build-all test clean install-deps

# Binary name
BINARY_NAME=mcpce
MAIN_PATH=./src/go/main.go
BIN_DIR=./bin

# Build flags
LDFLAGS=-ldflags "-s -w"
BUILD_FLAGS=-trimpath

# Default target
all: build

# Install Go dependencies
install-deps:
	go mod download
	go mod tidy

# Build for current platform
build: install-deps
	@echo "Building for current platform..."
	@mkdir -p $(BIN_DIR)
	go build $(BUILD_FLAGS) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME) $(MAIN_PATH)

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