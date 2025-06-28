package indexer

import (
	"context"
	"os"
	"testing"
	"path/filepath"
)

func TestRealZoektIntegration(t *testing.T) {
	// Create a temporary directory with test files
	tmpDir, err := os.MkdirTemp("", "zoekt-integration-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	
	// Create test files
	testFiles := map[string]string{
		"main.go": `package main

import "fmt"

func main() {
	fmt.Println("Hello, world!")
}
`,
		"utils.py": `def hello_world():
    print("Hello from Python!")

if __name__ == "__main__":
    hello_world()
`,
		"config.json": `{
  "name": "test-project",
  "version": "1.0.0",
  "main": "index.js"
}
`,
	}
	
	var filePaths []string
	for filename, content := range testFiles {
		filePath := filepath.Join(tmpDir, filename)
		err := os.WriteFile(filePath, []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to write test file %s: %v", filename, err)
		}
		filePaths = append(filePaths, filePath)
	}
	
	// Test real Zoekt indexer
	indexer := NewRealZoektIndexer(tmpDir)
	defer indexer.Close()
	
	ctx := context.Background()
	
	// Test indexing
	err = indexer.Index(ctx, filePaths)
	if err != nil {
		t.Fatalf("Failed to index files: %v", err)
	}
	
	// Verify stats
	stats := indexer.Stats()
	if stats.TotalFiles != 3 {
		t.Errorf("Expected 3 files, got %d", stats.TotalFiles)
	}
	if stats.IndexedFiles != 3 {
		t.Errorf("Expected 3 indexed files, got %d", stats.IndexedFiles)
	}
	
	// Test basic search
	options := SearchOptions{
		MaxResults: 10,
	}
	
	results, err := indexer.Search(ctx, "hello", options)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	
	if len(results) == 0 {
		t.Error("Expected search results for 'hello', got none")
	}
	
	t.Logf("Found %d results for 'hello'", len(results))
	for i, hit := range results {
		t.Logf("Result %d: File=%s, Line=%d, Score=%.2f, Text=%s", 
			i+1, hit.File, hit.LineNumber, hit.Score, hit.Text)
	}
	
	// Test regex search
	regexOptions := SearchOptions{
		MaxResults: 10,
		UseRegex:   true,
	}
	
	results, err = indexer.Search(ctx, `func[[:space:]]+[[:word:]]+`, regexOptions)
	if err != nil {
		t.Fatalf("Regex search failed: %v", err)
	}
	
	t.Logf("Found %d results for regex search", len(results))
	
	// Test file pattern search
	jsonOptions := SearchOptions{
		MaxResults:   10,
		FilePatterns: []string{"*.json"},
	}
	
	results, err = indexer.Search(ctx, "main", jsonOptions)
	if err != nil {
		t.Fatalf("File pattern search failed: %v", err)
	}
	
	t.Logf("Found %d results for 'main' in JSON files", len(results))
	
	// Test incremental indexing
	newFile := filepath.Join(tmpDir, "new.txt")
	err = os.WriteFile(newFile, []byte("This is a new file with hello world content"), 0644)
	if err != nil {
		t.Fatalf("Failed to write new test file: %v", err)
	}
	
	err = indexer.IncrementalIndex(ctx, []string{newFile})
	if err != nil {
		t.Fatalf("Incremental index failed: %v", err)
	}
	
	// Verify new file is indexed
	finalStats := indexer.Stats()
	if finalStats.TotalFiles != 4 {
		t.Errorf("Expected 4 files after incremental index, got %d", finalStats.TotalFiles)
	}
	
	// Test deletion
	err = indexer.Delete(ctx, []string{newFile})
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	
	deleteStats := indexer.Stats()
	if deleteStats.TotalFiles != 3 {
		t.Errorf("Expected 3 files after deletion, got %d", deleteStats.TotalFiles)
	}
}

func TestStubZoektIntegration(t *testing.T) {
	// Set environment to use stub
	original := os.Getenv("ZOEKT_USE_STUB")
	os.Setenv("ZOEKT_USE_STUB", "true")
	defer func() {
		if original == "" {
			os.Unsetenv("ZOEKT_USE_STUB")
		} else {
			os.Setenv("ZOEKT_USE_STUB", original)
		}
	}()
	
	// Create a temporary directory with test files
	tmpDir, err := os.MkdirTemp("", "zoekt-stub-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	
	// Create a simple test file
	testFile := filepath.Join(tmpDir, "test.go")
	err = os.WriteFile(testFile, []byte("package main\nfunc main() { println(\"test\") }"), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}
	
	// Test with stub implementation
	indexer := NewZoektIndexer(tmpDir)
	defer indexer.Close()
	
	ctx := context.Background()
	
	// Test indexing
	err = indexer.Index(ctx, []string{testFile})
	if err != nil {
		t.Fatalf("Failed to index file with stub: %v", err)
	}
	
	// Verify it's using the stub
	stats := indexer.Stats()
	if stats.TotalFiles != 1 {
		t.Errorf("Expected 1 file with stub, got %d", stats.TotalFiles)
	}
	
	t.Logf("Stub implementation successfully indexed %d file", stats.TotalFiles)
}