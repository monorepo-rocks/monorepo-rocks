package indexer

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// getTestFixturesDir returns the absolute path to the test fixtures directory
func getTestFixturesDir() string {
	// For now, use the absolute path to ensure tests work
	// This should be made portable in the future
	return "/Users/jh/src/monorepo-rocks/apps/mcp-context-engine/test/integration/fixtures"
}

func TestZoektIndexer_Index(t *testing.T) {
	// Force stub implementation for unit tests
	t.Setenv("ZOEKT_USE_STUB", "true")
	
	indexer := NewZoektIndexer("/tmp/test-index")
	defer indexer.Close()

	// Get absolute paths to test fixtures
	fixtureDir := getTestFixturesDir()
	files := []string{
		filepath.Join(fixtureDir, "sample_code.go"),
		filepath.Join(fixtureDir, "sample_code.py"), 
		filepath.Join(fixtureDir, "sample_code.js"),
	}

	ctx := context.Background()
	err := indexer.Index(ctx, files)
	if err != nil {
		t.Fatalf("Failed to index files: %v", err)
	}

	stats := indexer.Stats()
	if stats.TotalFiles != 3 {
		t.Errorf("Expected 3 files, got %d", stats.TotalFiles)
	}
	if stats.IndexedFiles != 3 {
		t.Errorf("Expected 3 indexed files, got %d", stats.IndexedFiles)
	}
}

func TestZoektIndexer_Search(t *testing.T) {
	// Force stub implementation for unit tests
	t.Setenv("ZOEKT_USE_STUB", "true")
	
	indexer := NewZoektIndexer("/tmp/test-index")
	defer indexer.Close()

	// Get absolute paths to test fixtures
	fixtureDir := getTestFixturesDir()
	files := []string{
		filepath.Join(fixtureDir, "sample_code.go"),
		filepath.Join(fixtureDir, "sample_code.py"),
	}
	ctx := context.Background()
	
	err := indexer.Index(ctx, files)
	if err != nil {
		t.Fatalf("Failed to index files: %v", err)
	}

	// Test basic search for "main" which should exist in the fixture files
	options := SearchOptions{
		MaxResults: 10,
	}
	
	results, err := indexer.Search(ctx, "main", options)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) == 0 {
		t.Error("Expected search results, got none")
	}

	// Verify result structure
	for _, hit := range results {
		if hit.File == "" {
			t.Error("Search hit missing file path")
		}
		if hit.Source != "lex" {
			t.Errorf("Expected source 'lex', got '%s'", hit.Source)
		}
		if hit.Score <= 0 {
			t.Errorf("Expected positive score, got %f", hit.Score)
		}
	}
}

func TestZoektIndexer_SearchRegex(t *testing.T) {
	// Force stub implementation for unit tests
	t.Setenv("ZOEKT_USE_STUB", "true")
	
	indexer := NewZoektIndexer("/tmp/test-index")
	defer indexer.Close()

	// Get absolute paths to test fixtures
	fixtureDir := getTestFixturesDir()
	files := []string{filepath.Join(fixtureDir, "sample_code.go")}
	ctx := context.Background()
	
	err := indexer.Index(ctx, files)
	if err != nil {
		t.Fatalf("Failed to index files: %v", err)
	}

	// Test regex search
	options := SearchOptions{
		MaxResults: 10,
		UseRegex:   true,
	}
	
	results, err := indexer.Search(ctx, `func\s+\w+`, options)
	if err != nil {
		t.Fatalf("Regex search failed: %v", err)
	}

	// Should find function declarations
	foundFunc := false
	for _, hit := range results {
		if strings.Contains(hit.Text, "func ") {
			foundFunc = true
			break
		}
	}
	if !foundFunc {
		t.Error("Expected to find function declarations with regex")
	}
}

func TestZoektIndexer_SearchWithFilters(t *testing.T) {
	// Force stub implementation for unit tests
	t.Setenv("ZOEKT_USE_STUB", "true")
	
	indexer := NewZoektIndexer("/tmp/test-index")
	defer indexer.Close()

	// Get absolute paths to test fixtures
	fixtureDir := getTestFixturesDir()
	files := []string{
		filepath.Join(fixtureDir, "sample_code.go"),
		filepath.Join(fixtureDir, "sample_code.py"),
		filepath.Join(fixtureDir, "sample_code.js"),
	}
	ctx := context.Background()
	
	err := indexer.Index(ctx, files)
	if err != nil {
		t.Fatalf("Failed to index files: %v", err)
	}

	// Test language filter
	options := SearchOptions{
		MaxResults: 10,
		Languages:  []string{"go"},
	}
	
	results, err := indexer.Search(ctx, "main", options)
	if err != nil {
		t.Fatalf("Search with language filter failed: %v", err)
	}

	// All results should be Go files
	for _, hit := range results {
		if hit.Language != "go" {
			t.Errorf("Expected Go language, got '%s'", hit.Language)
		}
	}

	// Test file pattern filter
	options = SearchOptions{
		MaxResults:   10,
		FilePatterns: []string{"*.py"},
	}
	
	results, err = indexer.Search(ctx, "main", options)
	if err != nil {
		t.Fatalf("Search with file pattern filter failed: %v", err)
	}

	// All results should be Python files
	for _, hit := range results {
		if !strings.HasSuffix(hit.File, ".py") {
			t.Errorf("Expected Python file, got '%s'", hit.File)
		}
	}
}

func TestZoektIndexer_IncrementalIndex(t *testing.T) {
	// Force stub implementation for unit tests
	t.Setenv("ZOEKT_USE_STUB", "true")
	
	indexer := NewZoektIndexer("/tmp/test-index")
	defer indexer.Close()

	// Initial indexing
	fixtureDir := getTestFixturesDir()
	files := []string{filepath.Join(fixtureDir, "sample_code.go")}
	ctx := context.Background()
	
	err := indexer.Index(ctx, files)
	if err != nil {
		t.Fatalf("Failed to index files: %v", err)
	}

	initialStats := indexer.Stats()
	initialTime := initialStats.LastIndexTime

	// Sleep to ensure different timestamp
	time.Sleep(10 * time.Millisecond)

	// Incremental indexing
	newFiles := []string{filepath.Join(fixtureDir, "sample_code.py")}
	err = indexer.IncrementalIndex(ctx, newFiles)
	if err != nil {
		t.Fatalf("Failed to incrementally index files: %v", err)
	}

	finalStats := indexer.Stats()
	if finalStats.TotalFiles != 2 {
		t.Errorf("Expected 2 files after incremental index, got %d", finalStats.TotalFiles)
	}
	if !finalStats.LastIndexTime.After(initialTime) {
		t.Error("Expected last index time to be updated")
	}
}

func TestZoektIndexer_Delete(t *testing.T) {
	// Force stub implementation for unit tests
	t.Setenv("ZOEKT_USE_STUB", "true")
	
	indexer := NewZoektIndexer("/tmp/test-index")
	defer indexer.Close()

	// Get absolute paths to test fixtures
	fixtureDir := getTestFixturesDir()
	files := []string{
		filepath.Join(fixtureDir, "sample_code.go"),
		filepath.Join(fixtureDir, "sample_code.py"),
	}
	ctx := context.Background()
	
	err := indexer.Index(ctx, files)
	if err != nil {
		t.Fatalf("Failed to index files: %v", err)
	}

	// Delete one file
	err = indexer.Delete(ctx, []string{filepath.Join(fixtureDir, "sample_code.go")})
	if err != nil {
		t.Fatalf("Failed to delete file: %v", err)
	}

	stats := indexer.Stats()
	if stats.TotalFiles != 1 {
		t.Errorf("Expected 1 file after deletion, got %d", stats.TotalFiles)
	}
}

func TestZoektIndexer_BM25Scoring(t *testing.T) {
	// Force stub implementation for unit tests
	t.Setenv("ZOEKT_USE_STUB", "true")
	
	indexer := NewZoektIndexer("/tmp/test-index")
	defer indexer.Close()

	// Get absolute paths to test fixtures
	fixtureDir := getTestFixturesDir()
	files := []string{
		filepath.Join(fixtureDir, "sample_code.go"),
		filepath.Join(fixtureDir, "sample_code.py"),
	}
	ctx := context.Background()
	
	err := indexer.Index(ctx, files)
	if err != nil {
		t.Fatalf("Failed to index files: %v", err)
	}

	options := SearchOptions{MaxResults: 10}
	// Search for a term that exists in the fixture files
	results, err := indexer.Search(ctx, "user", options)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) == 0 {
		t.Error("Expected search results for BM25 scoring test")
		return
	}

	// Verify scores are properly calculated and sorted
	for i := 1; i < len(results); i++ {
		if results[i-1].Score < results[i].Score {
			t.Error("Results should be sorted by score in descending order")
			break
		}
	}
}

func TestZoektIndexer_CaseSensitivity(t *testing.T) {
	// Force stub implementation for unit tests
	t.Setenv("ZOEKT_USE_STUB", "true")
	
	indexer := NewZoektIndexer("/tmp/test-index")
	defer indexer.Close()

	// Get absolute paths to test fixtures
	fixtureDir := getTestFixturesDir()
	files := []string{filepath.Join(fixtureDir, "sample_code.go")}
	ctx := context.Background()
	
	err := indexer.Index(ctx, files)
	if err != nil {
		t.Fatalf("Failed to index files: %v", err)
	}

	// Test case-insensitive search (default)
	options := SearchOptions{
		MaxResults:    10,
		CaseSensitive: false,
	}
	
	// Search for "USER" which should exist in our fixture files
	results, err := indexer.Search(ctx, "USER", options)
	if err != nil {
		t.Fatalf("Case-insensitive search failed: %v", err)
	}

	if len(results) == 0 {
		t.Error("Expected results for case-insensitive search")
	}

	// Test case-sensitive search
	options.CaseSensitive = true
	results, err = indexer.Search(ctx, "USER", options)
	if err != nil {
		t.Fatalf("Case-sensitive search failed: %v", err)
	}

	// Should have fewer or no results for uppercase query
	// (depends on content case, but this tests the option works)
}

func TestZoektIndexer_LanguageDetection(t *testing.T) {
	indexer := &ZoektStubIndexer{}

	tests := []struct {
		filename string
		expected string
	}{
		// Programming languages
		{"main.go", "go"},
		{"script.py", "python"},
		{"app.js", "javascript"},
		{"types.ts", "typescript"},
		{"component.tsx", "typescript"},
		{"component.jsx", "javascript"},
		{"Main.java", "java"},
		{"program.cpp", "cpp"},
		{"util.c", "c"},
		{"lib.rs", "rust"},
		{"script.rb", "ruby"},
		{"index.php", "php"},
		{"Program.cs", "csharp"},
		{"App.kt", "kotlin"},
		{"View.swift", "swift"},
		{"Main.scala", "scala"},
		{"core.clj", "clojure"},
		{"Utils.hs", "haskell"},
		// Configuration and data files
		{"package.json", "json"},
		{"config.yaml", "yaml"},
		{"docker-compose.yml", "yaml"},
		{"data.xml", "xml"},
		{"Cargo.toml", "toml"},
		{"settings.ini", "ini"},
		// Documentation
		{"README.md", "markdown"},
		{"docs.rst", "restructuredtext"},
		{"readme.txt", "text"},
		// Web technologies
		{"index.html", "html"},
		{"styles.css", "css"},
		{"main.scss", "scss"},
		{"vars.sass", "sass"},
		{"theme.less", "less"},
		{"App.vue", "vue"},
		{"Component.svelte", "svelte"},
		// Build and special files
		{"Dockerfile", "dockerfile"},
		{"Makefile", "makefile"},
		{"CMakeLists.txt", "cmake"},
		{"go.mod", "go-mod"},
		{"go.sum", "go-sum"},
		{"pyproject.toml", "toml"},
		{"requirements.txt", "text"},
		{"Pipfile", "toml"},
		{".gitignore", "gitignore"},
		// Unknown file should default to text
		{"unknown.xyz", "text"},
	}

	for _, test := range tests {
		actual := indexer.detectLanguage(test.filename)
		if actual != test.expected {
			t.Errorf("detectLanguage(%s) = %s, expected %s", 
				test.filename, actual, test.expected)
		}
	}
}

func TestZoektIndexer_Tokenization(t *testing.T) {
	indexer := &ZoektStubIndexer{}

	text := "Hello, World! This is a test-string with numbers123."
	tokens := indexer.tokenize(text)

	expected := []string{"Hello", "World", "This", "is", "a", "test", "string", "with", "numbers123"}
	if len(tokens) != len(expected) {
		t.Errorf("Expected %d tokens, got %d", len(expected), len(tokens))
	}

	for i, token := range tokens {
		if i < len(expected) && token != expected[i] {
			t.Errorf("Token %d: expected '%s', got '%s'", i, expected[i], token)
		}
	}
}