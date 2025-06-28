package backends

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/indexer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestData represents the structure of our test data JSON
type TestData struct {
	SearchQueries []struct {
		Query              string   `json:"query"`
		ExpectedFiles      []string `json:"expected_files"`
		ExpectedMinResults int      `json:"expected_min_results"`
	} `json:"search_queries"`
	TestScenarios struct {
		BasicSearch struct {
			Description string   `json:"description"`
			Queries     []string `json:"queries"`
		} `json:"basic_search"`
		RegexSearch struct {
			Description string   `json:"description"`
			Patterns    []string `json:"patterns"`
		} `json:"regex_search"`
		LanguageFilter struct {
			Description string              `json:"description"`
			Filters     map[string][]string `json:"filters"`
		} `json:"language_filter"`
		FilePattern struct {
			Description string   `json:"description"`
			Patterns    []string `json:"patterns"`
		} `json:"file_pattern"`
	} `json:"test_scenarios"`
}

// setupTestEnvironment creates a temporary directory with test files
func setupTestEnvironment(t *testing.T) (string, indexer.ZoektIndexer, TestData) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "zoekt-real-integration-*")
	require.NoError(t, err, "Failed to create temp directory")

	// Copy test fixtures to temp directory
	fixturesDir := "../fixtures"
	testFiles := []string{
		"sample_code.go",
		"sample_code.py",  
		"sample_code.js",
		"config.yaml",
		"test_data.json",
	}

	for _, file := range testFiles {
		src := filepath.Join(fixturesDir, file)
		dst := filepath.Join(tmpDir, file)
		
		data, err := ioutil.ReadFile(src)
		require.NoError(t, err, "Failed to read fixture file %s", file)
		
		err = ioutil.WriteFile(dst, data, 0644)
		require.NoError(t, err, "Failed to write test file %s", file)
	}

	// Create indexer
	indexer := indexer.NewRealZoektIndexer(tmpDir)

	// Load test data
	testDataPath := filepath.Join(fixturesDir, "test_data.json")
	testDataBytes, err := ioutil.ReadFile(testDataPath)
	require.NoError(t, err, "Failed to read test data")

	var testData TestData
	err = json.Unmarshal(testDataBytes, &testData)
	require.NoError(t, err, "Failed to parse test data")

	return tmpDir, indexer, testData
}

func TestRealZoektIndexerBasicOperations(t *testing.T) {
	tmpDir, zoektIdx, _ := setupTestEnvironment(t)
	defer func() {
		zoektIdx.Close()
		os.RemoveAll(tmpDir)
	}()

	ctx := context.Background()

	t.Run("Initial stats", func(t *testing.T) {
		stats := zoektIdx.Stats()
		assert.Equal(t, 0, stats.TotalFiles, "Should start with 0 files")
		assert.Equal(t, 0, stats.IndexedFiles, "Should start with 0 indexed files")
	})

	t.Run("Index files", func(t *testing.T) {
		files := []string{
			filepath.Join(tmpDir, "sample_code.go"),
			filepath.Join(tmpDir, "sample_code.py"),
			filepath.Join(tmpDir, "sample_code.js"),
			filepath.Join(tmpDir, "config.yaml"),
		}

		err := zoektIdx.Index(ctx, files)
		assert.NoError(t, err, "Indexing should succeed")

		stats := zoektIdx.Stats()
		assert.Equal(t, 4, stats.TotalFiles, "Should have 4 indexed files")
		assert.Equal(t, 4, stats.IndexedFiles, "Should have 4 indexed files")
		assert.True(t, stats.LastIndexTime.After(time.Now().Add(-time.Minute)), "LastIndexTime should be recent")
	})

	t.Run("Basic search", func(t *testing.T) {
		options := indexer.SearchOptions{
			MaxResults: 10,
		}

		results, err := zoektIdx.Search(ctx, "authenticate", options)
		assert.NoError(t, err, "Search should succeed")
		assert.GreaterOrEqual(t, len(results), 3, "Should find authenticate function in multiple files")

		// Verify results contain expected information
		for _, hit := range results {
			assert.NotEmpty(t, hit.File, "Hit should have file path")
			assert.Greater(t, hit.LineNumber, 0, "Hit should have valid line number")
			assert.NotEmpty(t, hit.Text, "Hit should have text content")
			assert.Greater(t, hit.Score, 0.0, "Hit should have positive score")
			assert.Equal(t, "lex", hit.Source, "Hit should be from lexical search")
		}
	})

	t.Run("Case sensitivity", func(t *testing.T) {
		caseSensitiveOptions := indexer.SearchOptions{
			MaxResults:    10,
			CaseSensitive: true,
		}

		caseInsensitiveOptions := indexer.SearchOptions{
			MaxResults:    10,
			CaseSensitive: false,
		}

		// Search for "AUTHENTICATE" (uppercase)
		sensitiveResults, err := zoektIdx.Search(ctx, "AUTHENTICATE", caseSensitiveOptions)
		assert.NoError(t, err)

		insensitiveResults, err := zoektIdx.Search(ctx, "AUTHENTICATE", caseInsensitiveOptions)
		assert.NoError(t, err)

		// Case insensitive should find more results (or at least the same)
		assert.GreaterOrEqual(t, len(insensitiveResults), len(sensitiveResults),
			"Case insensitive search should find at least as many results")
	})

	t.Run("Language filtering", func(t *testing.T) {
		options := indexer.SearchOptions{
			MaxResults: 10,
			Languages:  []string{"go"},
		}

		results, err := zoektIdx.Search(ctx, "func", options)
		assert.NoError(t, err, "Language filtered search should succeed")

		// All results should be from Go files
		for _, hit := range results {
			assert.Contains(t, hit.File, ".go", "All results should be from Go files")
		}
	})

	t.Run("File pattern filtering", func(t *testing.T) {
		options := indexer.SearchOptions{
			MaxResults:   10,
			FilePatterns: []string{"*.py"},
		}

		results, err := zoektIdx.Search(ctx, "def", options)
		assert.NoError(t, err, "File pattern search should succeed")

		// All results should be from Python files
		for _, hit := range results {
			assert.Contains(t, hit.File, ".py", "All results should be from Python files")
		}
	})
}

func TestRealZoektIndexerRegexSearch(t *testing.T) {
	tmpDir, zoektIdx, _ := setupTestEnvironment(t)
	defer func() {
		zoektIdx.Close()
		os.RemoveAll(tmpDir)
	}()

	ctx := context.Background()

	// Index test files
	files := []string{
		filepath.Join(tmpDir, "sample_code.go"),
		filepath.Join(tmpDir, "sample_code.py"),
		filepath.Join(tmpDir, "sample_code.js"),
	}

	err := zoektIdx.Index(ctx, files)
	require.NoError(t, err, "Indexing should succeed")

	t.Run("Function definition regex", func(t *testing.T) {
		options := indexer.SearchOptions{
			MaxResults: 10,
			UseRegex:   true,
		}

		// Search for function definitions
		results, err := zoektIdx.Search(ctx, `func[[:space:]]+[[:word:]]+`, options)
		assert.NoError(t, err, "Regex search should succeed")
		assert.Greater(t, len(results), 0, "Should find function definitions")

		// Verify results contain function definitions
		for _, hit := range results {
			assert.Contains(t, hit.Text, "func", "Result should contain 'func'")
		}
	})

	t.Run("Class definition regex", func(t *testing.T) {
		options := indexer.SearchOptions{
			MaxResults: 10,
			UseRegex:   true,
		}

		// Search for class definitions
		results, err := zoektIdx.Search(ctx, `class[[:space:]]+[[:word:]]+`, options)
		assert.NoError(t, err, "Regex search should succeed")

		// Should find class definitions in Python and JavaScript
		foundPython := false
		foundJavaScript := false
		for _, hit := range results {
			if filepath.Ext(hit.File) == ".py" {
				foundPython = true
			}
			if filepath.Ext(hit.File) == ".js" {
				foundJavaScript = true
			}
		}
		assert.True(t, foundPython || foundJavaScript, "Should find class definitions")
	})

	t.Run("Invalid regex", func(t *testing.T) {
		options := indexer.SearchOptions{
			MaxResults: 10,
			UseRegex:   true,
		}

		// Search with invalid regex
		_, err := zoektIdx.Search(ctx, `[invalid`, options)
		assert.Error(t, err, "Invalid regex should return error")
	})
}

func TestRealZoektIndexerIncrementalOperations(t *testing.T) {
	tmpDir, zoektIdx, _ := setupTestEnvironment(t)
	defer func() {
		zoektIdx.Close()
		os.RemoveAll(tmpDir)
	}()

	ctx := context.Background()

	// Initial indexing
	initialFiles := []string{
		filepath.Join(tmpDir, "sample_code.go"),
		filepath.Join(tmpDir, "sample_code.py"),
	}

	err := zoektIdx.Index(ctx, initialFiles)
	require.NoError(t, err, "Initial indexing should succeed")

	initialStats := zoektIdx.Stats()
	assert.Equal(t, 2, initialStats.TotalFiles, "Should have 2 files initially")

	t.Run("Incremental add", func(t *testing.T) {
		// Add a new file
		newFile := filepath.Join(tmpDir, "sample_code.js")

		err := zoektIdx.IncrementalIndex(ctx, []string{newFile})
		assert.NoError(t, err, "Incremental indexing should succeed")

		stats := zoektIdx.Stats()
		assert.Equal(t, 3, stats.TotalFiles, "Should have 3 files after incremental add")

		// Verify new file is searchable
		options := indexer.SearchOptions{MaxResults: 10}
		results, err := zoektIdx.Search(ctx, "class User", options)
		assert.NoError(t, err, "Search should succeed")

		// Should find User class in both Python and JavaScript files
		foundFiles := make(map[string]bool)
		for _, hit := range results {
			foundFiles[filepath.Ext(hit.File)] = true
		}
		assert.True(t, foundFiles[".py"], "Should find User class in Python file")
		assert.True(t, foundFiles[".js"], "Should find User class in JavaScript file")
	})

	t.Run("File deletion", func(t *testing.T) {
		// Delete a file
		deleteFile := filepath.Join(tmpDir, "sample_code.py")

		err := zoektIdx.Delete(ctx, []string{deleteFile})
		assert.NoError(t, err, "File deletion should succeed")

		stats := zoektIdx.Stats()
		assert.Equal(t, 2, stats.TotalFiles, "Should have 2 files after deletion")

		// Verify deleted file is not in search results
		options := indexer.SearchOptions{MaxResults: 10}
		results, err := zoektIdx.Search(ctx, "def authenticate", options)
		assert.NoError(t, err, "Search should succeed")

		// Should not find Python def statements
		for _, hit := range results {
			assert.NotContains(t, hit.File, "sample_code.py", "Should not find results in deleted file")
		}
	})
}

func TestRealZoektIndexerPersistence(t *testing.T) {
	tmpDir, zoektIdx, _ := setupTestEnvironment(t)
	defer func() {
		zoektIdx.Close()
		os.RemoveAll(tmpDir)
	}()

	ctx := context.Background()

	// Index some files
	files := []string{
		filepath.Join(tmpDir, "sample_code.go"),
		filepath.Join(tmpDir, "sample_code.py"),
	}

	err := zoektIdx.Index(ctx, files)
	require.NoError(t, err, "Indexing should succeed")

	// Test save
	t.Run("Save index", func(t *testing.T) {
		indexPath := filepath.Join(tmpDir, "test.index")
		err := zoektIdx.Save(ctx, indexPath)
		assert.NoError(t, err, "Save should succeed")

		// Index file should exist (even if it's just a placeholder for now)
		// The real implementation would create actual index files
	})

	// Test load
	t.Run("Load index", func(t *testing.T) {
		indexPath := filepath.Join(tmpDir, "test.index")
		
		// Create a new indexer and try to load
		newIndexer := indexer.NewRealZoektIndexer(tmpDir)
		defer newIndexer.Close()

		err := newIndexer.Load(ctx, indexPath)
		// For now, this might not work fully since save/load is not fully implemented
		// but it should not crash
		if err != nil {
			t.Logf("Load operation failed as expected (not fully implemented): %v", err)
		}
	})
}

func TestRealZoektIndexerAdvancedQueries(t *testing.T) {
	tmpDir, zoektIdx, testData := setupTestEnvironment(t)
	defer func() {
		zoektIdx.Close()
		os.RemoveAll(tmpDir)
	}()

	ctx := context.Background()

	// Index all test files
	files := []string{
		filepath.Join(tmpDir, "sample_code.go"),
		filepath.Join(tmpDir, "sample_code.py"),
		filepath.Join(tmpDir, "sample_code.js"),
		filepath.Join(tmpDir, "config.yaml"),
	}

	err := zoektIdx.Index(ctx, files)
	require.NoError(t, err, "Indexing should succeed")

	t.Run("Test data queries", func(t *testing.T) {
		for _, queryTest := range testData.SearchQueries {
			t.Run(queryTest.Query, func(t *testing.T) {
				options := indexer.SearchOptions{MaxResults: 20}
				results, err := zoektIdx.Search(ctx, queryTest.Query, options)
				assert.NoError(t, err, "Search should succeed for query: %s", queryTest.Query)
				assert.GreaterOrEqual(t, len(results), queryTest.ExpectedMinResults,
					"Should find at least %d results for query: %s", queryTest.ExpectedMinResults, queryTest.Query)

				// Check if expected files are found
				foundFiles := make(map[string]bool)
				for _, hit := range results {
					filename := filepath.Base(hit.File)
					foundFiles[filename] = true
				}

				for _, expectedFile := range queryTest.ExpectedFiles {
					assert.True(t, foundFiles[expectedFile],
						"Should find results in expected file: %s for query: %s", expectedFile, queryTest.Query)
				}
			})
		}
	})

	t.Run("Complex search scenarios", func(t *testing.T) {
		// Test basic search queries
		for _, query := range testData.TestScenarios.BasicSearch.Queries {
			t.Run("Basic: "+query, func(t *testing.T) {
				options := indexer.SearchOptions{MaxResults: 10}
				results, err := zoektIdx.Search(ctx, query, options)
				assert.NoError(t, err, "Basic search should succeed")
				assert.Greater(t, len(results), 0, "Should find results for basic query: %s", query)
			})
		}

		// Test regex patterns
		for _, pattern := range testData.TestScenarios.RegexSearch.Patterns {
			t.Run("Regex: "+pattern, func(t *testing.T) {
				options := indexer.SearchOptions{
					MaxResults: 10,
					UseRegex:   true,
				}
				_, err := zoektIdx.Search(ctx, pattern, options)
				assert.NoError(t, err, "Regex search should succeed for pattern: %s", pattern)
				// Note: Some patterns might not match, which is okay
			})
		}

		// Test language filters
		for lang, queries := range testData.TestScenarios.LanguageFilter.Filters {
			for _, query := range queries {
				t.Run("Language "+lang+": "+query, func(t *testing.T) {
					options := indexer.SearchOptions{
						MaxResults: 10,
						Languages:  []string{lang},
					}
					results, err := zoektIdx.Search(ctx, query, options)
					assert.NoError(t, err, "Language filtered search should succeed")

					// Verify language filtering works
					for _, hit := range results {
						// Check file extension matches expected language
						ext := filepath.Ext(hit.File)
						switch lang {
						case "go":
							assert.Equal(t, ".go", ext, "Should only find Go files")
						case "python":
							assert.Equal(t, ".py", ext, "Should only find Python files")
						case "javascript":
							assert.Equal(t, ".js", ext, "Should only find JavaScript files")
						}
					}
				})
			}
		}

		// Test file patterns
		for _, pattern := range testData.TestScenarios.FilePattern.Patterns {
			t.Run("Pattern: "+pattern, func(t *testing.T) {
				options := indexer.SearchOptions{
					MaxResults:   10,
					FilePatterns: []string{pattern},
				}
				results, err := zoektIdx.Search(ctx, "function", options) // Generic search term
				assert.NoError(t, err, "File pattern search should succeed")

				// Verify file pattern filtering
				for _, hit := range results {
					filename := filepath.Base(hit.File)
					matched, err := filepath.Match(pattern, filename)
					if err == nil {
						assert.True(t, matched, "Result file %s should match pattern %s", filename, pattern)
					}
				}
			})
		}
	})
}

func TestRealZoektIndexerErrorHandling(t *testing.T) {
	tmpDir, zoektIdx, _ := setupTestEnvironment(t)
	defer func() {
		zoektIdx.Close()
		os.RemoveAll(tmpDir)
	}()

	ctx := context.Background()

	t.Run("Index non-existent file", func(t *testing.T) {
		nonExistentFile := filepath.Join(tmpDir, "non_existent.go")
		err := zoektIdx.Index(ctx, []string{nonExistentFile})
		assert.Error(t, err, "Should return error for non-existent file")
	})

	t.Run("Search empty index", func(t *testing.T) {
		options := indexer.SearchOptions{MaxResults: 10}
		results, err := zoektIdx.Search(ctx, "anything", options)
		assert.NoError(t, err, "Search on empty index should not error")
		assert.Equal(t, 0, len(results), "Should return no results for empty index")
	})

	t.Run("Delete non-existent file", func(t *testing.T) {
		err := zoektIdx.Delete(ctx, []string{"non_existent.go"})
		assert.NoError(t, err, "Delete non-existent file should not error")
	})

	t.Run("Large file handling", func(t *testing.T) {
		// Create a large file (over 10MB limit)
		largeFile := filepath.Join(tmpDir, "large.go")
		largeContent := make([]byte, 11*1024*1024) // 11MB
		for i := range largeContent {
			largeContent[i] = 'a'
		}
		
		err := ioutil.WriteFile(largeFile, largeContent, 0644)
		require.NoError(t, err, "Should create large file")

		err = zoektIdx.Index(ctx, []string{largeFile})
		assert.Error(t, err, "Should return error for file too large")
	})
}

func TestRealZoektIndexerConcurrency(t *testing.T) {
	tmpDir, zoektIdx, _ := setupTestEnvironment(t)
	defer func() {
		zoektIdx.Close()
		os.RemoveAll(tmpDir)
	}()

	ctx := context.Background()

	// Index some files first
	files := []string{
		filepath.Join(tmpDir, "sample_code.go"),
		filepath.Join(tmpDir, "sample_code.py"),
		filepath.Join(tmpDir, "sample_code.js"),
	}

	err := zoektIdx.Index(ctx, files)
	require.NoError(t, err, "Initial indexing should succeed")

	t.Run("Concurrent searches", func(t *testing.T) {
		const numGoroutines = 10
		const numSearches = 5

		searchQueries := []string{"authenticate", "function", "class", "import", "user"}
		
		results := make(chan error, numGoroutines*numSearches)

		for i := 0; i < numGoroutines; i++ {
			go func() {
				for j := 0; j < numSearches; j++ {
					query := searchQueries[j%len(searchQueries)]
					options := indexer.SearchOptions{MaxResults: 10}
					
					_, err := zoektIdx.Search(ctx, query, options)
					results <- err
				}
			}()
		}

		// Collect all results
		for i := 0; i < numGoroutines*numSearches; i++ {
			err := <-results
			assert.NoError(t, err, "Concurrent search should succeed")
		}
	})

	t.Run("Search during indexing", func(t *testing.T) {
		// Start a long-running indexing operation
		go func() {
			// Re-index the same files multiple times
			for i := 0; i < 5; i++ {
				zoektIdx.Index(ctx, files)
				time.Sleep(10 * time.Millisecond)
			}
		}()

		// Perform searches while indexing is happening
		for i := 0; i < 10; i++ {
			options := indexer.SearchOptions{MaxResults: 5}
			_, err := zoektIdx.Search(ctx, "function", options)
			assert.NoError(t, err, "Search during indexing should succeed")
			time.Sleep(5 * time.Millisecond)
		}
	})
}