#!/bin/bash
# Performance validation script for MCP Context Engine

set -e

echo "=== MCP Context Engine Performance Validation ==="
echo

# Build the binary first
echo "Building mcpce binary..."
make build

# Create a test directory with sample files
TEST_DIR="/tmp/mcpce_perf_test"
rm -rf "$TEST_DIR"
mkdir -p "$TEST_DIR"

echo "Creating 100 sample files for performance test..."
# Create 100 sample Go files (simulating a small codebase)
for i in {1..100}; do
    cat > "$TEST_DIR/file_$i.go" << EOF
package main

import (
    "fmt"
    "strings"
)

// Function$i performs operation $i
func Function$i(input string) string {
    result := strings.ToUpper(input)
    fmt.Printf("Processing in function %d: %s\n", $i, result)
    return result
}

// Helper$i assists Function$i
func Helper$i(data []string) []string {
    var processed []string
    for _, item := range data {
        processed = append(processed, Function$i(item))
    }
    return processed
}
EOF
done

echo
echo "Indexing $TEST_DIR with 100 files..."
echo "Target: < 200ms for search operations"
echo

# Index the files
time ./bin/mcpce index "$TEST_DIR"

echo
echo "Running search performance tests..."
echo

# Function to measure search time
measure_search() {
    local query="$1"
    local start=$(date +%s%N)
    ./bin/mcpce search -q "$query" > /dev/null 2>&1
    local end=$(date +%s%N)
    local duration=$(( ($end - $start) / 1000000 )) # Convert to milliseconds
    echo "Search for '$query': ${duration}ms"
    return $duration
}

# Run multiple searches and check performance
total_time=0
search_count=0

# Test different query types
queries=(
    "Function"
    "import fmt"
    "ToUpper"
    "Helper AND processed"
    "file:*.go Function"
)

for query in "${queries[@]}"; do
    duration=$(measure_search "$query" || echo 0)
    total_time=$((total_time + duration))
    search_count=$((search_count + 1))
    
    if [ $duration -gt 200 ]; then
        echo "  ⚠️  WARNING: Search exceeded 200ms target!"
    else
        echo "  ✅ Within target (<200ms)"
    fi
done

echo
avg_time=$((total_time / search_count))
echo "Average search time: ${avg_time}ms"

if [ $avg_time -lt 200 ]; then
    echo "✅ PASS: Average search time is within 200ms target"
else
    echo "❌ FAIL: Average search time exceeds 200ms target"
fi

echo
echo "=== Performance Validation Complete ==="

# Cleanup
rm -rf "$TEST_DIR"