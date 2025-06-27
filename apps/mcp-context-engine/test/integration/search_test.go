package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/embedder"
	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/indexer"
	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/query"
	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/types"
)

func TestEndToEndSearch(t *testing.T) {
	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "mcpce-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test files
	testFiles := map[string]string{
		"main.go": `package main

import "fmt"

func main() {
    fmt.Println("Hello, World!")
}

func authenticate(username, password string) bool {
    // TODO: Implement authentication
    return username == "admin" && password == "secret"
}`,
		"utils.js": `import chalk from 'chalk';

export function logError(message) {
    console.error(chalk.red('Error:', message));
}

export function authenticate(user, pass) {
    // Simple auth check
    return user === 'admin' && pass === 'password';
}`,
		"test.py": `import unittest

def authenticate(username, password):
    """Authenticate a user"""
    return username == "admin" and password == "admin123"

class TestAuth(unittest.TestCase):
    def test_authenticate(self):
        self.assertTrue(authenticate("admin", "admin123"))
`,
	}

	// Write test files
	for name, content := range testFiles {
		path := filepath.Join(tmpDir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write test file %s: %v", name, err)
		}
	}

	// Initialize components
	indexPath := filepath.Join(tmpDir, "indexes")
	zoektIdx := indexer.NewZoektIndexer(indexPath)
	faissIdx := indexer.NewFAISSIndex(indexPath)
	emb := embedder.NewEmbedder(embedder.DefaultConfig())

	ctx := context.Background()

	// Index files
	for name := range testFiles {
		path := filepath.Join(tmpDir, name)
		if err := zoektIdx.IndexFile(ctx, path); err != nil {
			t.Fatalf("Failed to index %s: %v", name, err)
		}
	}

	// Create query service
	querySvc := query.NewService(zoektIdx, faissIdx, emb, 0.45)

	// Test cases
	tests := []struct {
		name     string
		query    string
		expected int // minimum expected results
	}{
		{
			name:     "Search for authenticate function",
			query:    "authenticate function",
			expected: 3, // Should find in all three files
		},
		{
			name:     "Search for import chalk",
			query:    "import chalk",
			expected: 1, // Only in utils.js
		},
		{
			name:     "Search for unittest",
			query:    "unittest",
			expected: 1, // Only in test.py
		},
		{
			name:     "Language-specific search",
			query:    "fmt.Println",
			expected: 1, // Only in main.go
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &types.SearchRequest{
				Query: tt.query,
				TopK:  10,
			}

			resp, err := querySvc.Search(ctx, req)
			if err != nil {
				t.Fatalf("Search failed: %v", err)
			}

			if len(resp.Hits) < tt.expected {
				t.Errorf("Expected at least %d results, got %d", tt.expected, len(resp.Hits))
				for _, hit := range resp.Hits {
					t.Logf("  - %s:%d (score: %.3f)", hit.File, hit.LineNumber, hit.Score)
				}
			}
		})
	}
}

func TestSearchWithFilters(t *testing.T) {
	// Create components
	indexPath := os.TempDir()
	zoektIdx := indexer.NewZoektIndexer(indexPath)
	faissIdx := indexer.NewFAISSIndex(indexPath)
	emb := embedder.NewEmbedder(embedder.DefaultConfig())

	ctx := context.Background()
	querySvc := query.NewService(zoektIdx, faissIdx, emb, 0.45)

	// Test language filter
	req := &types.SearchRequest{
		Query:    "function",
		TopK:     10,
		Language: "javascript",
	}

	resp, err := querySvc.Search(ctx, req)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Verify all results are JavaScript files
	for _, hit := range resp.Hits {
		if hit.Language != "" && hit.Language != "javascript" {
			t.Errorf("Expected JavaScript file, got %s", hit.Language)
		}
	}
}