package watcher

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/indexer"
	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/watcher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockIndexer implements a simple indexer for testing file watcher integration
type MockIndexer struct {
	indexedFiles   map[string]time.Time
	deletedFiles   map[string]time.Time
	indexCallCount int
	deleteCallCount int
}

func NewMockIndexer() *MockIndexer {
	return &MockIndexer{
		indexedFiles: make(map[string]time.Time),
		deletedFiles: make(map[string]time.Time),
	}
}

func (m *MockIndexer) IndexFile(ctx context.Context, path string) error {
	m.indexCallCount++
	m.indexedFiles[path] = time.Now()
	// Remove from deleted files if it was previously deleted
	delete(m.deletedFiles, path)
	return nil
}

func (m *MockIndexer) DeleteFile(ctx context.Context, path string) error {
	m.deleteCallCount++
	m.deletedFiles[path] = time.Now()
	// Remove from indexed files
	delete(m.indexedFiles, path)
	return nil
}

func (m *MockIndexer) IsIndexed(path string) bool {
	_, exists := m.indexedFiles[path]
	return exists
}

func (m *MockIndexer) IsDeleted(path string) bool {
	_, exists := m.deletedFiles[path]
	return exists
}

func (m *MockIndexer) GetStats() (int, int) {
	return m.indexCallCount, m.deleteCallCount
}

// setupWatcherTestEnvironment creates a test environment for file watcher tests
func setupWatcherTestEnvironment(t *testing.T) (string, *MockIndexer, func()) {
	tmpDir, err := os.MkdirTemp("", "watcher-integration-*")
	require.NoError(t, err, "Failed to create temp directory")

	mockIndexer := NewMockIndexer()

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return tmpDir, mockIndexer, cleanup
}

// waitForWatcherEvents waits for the file watcher to process events
func waitForWatcherEvents() {
	time.Sleep(100 * time.Millisecond) // Give watcher time to process events
}

func TestFileWatcherBasicOperations(t *testing.T) {
	tmpDir, mockIndexer, cleanup := setupWatcherTestEnvironment(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create watcher
	w, err := watcher.NewWatcher(tmpDir)
	require.NoError(t, err, "Failed to create watcher")
	defer w.Close()

	// Start watching
	eventChan := make(chan watcher.FileEvent, 100)
	err = w.Start(ctx, eventChan)
	require.NoError(t, err, "Failed to start watcher")

	t.Run("File creation detection", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "test.go")
		testContent := `package main

import "fmt"

func main() {
    fmt.Println("Hello, World!")
}`

		// Create file
		err := ioutil.WriteFile(testFile, []byte(testContent), 0644)
		require.NoError(t, err, "Failed to create test file")

		// Wait for event
		select {
		case event := <-eventChan:
			assert.Equal(t, watcher.EventCreate, event.Type, "Should detect file creation")
			assert.Equal(t, testFile, event.Path, "Should have correct file path")
			
			// Process the event
			err := mockIndexer.IndexFile(ctx, event.Path)
			assert.NoError(t, err, "Should index created file")
			
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for file creation event")
		}

		// Verify file was indexed
		assert.True(t, mockIndexer.IsIndexed(testFile), "File should be indexed")
	})

	t.Run("File modification detection", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "modify_test.go")
		initialContent := `package main

func hello() {
    println("hello")
}`

		// Create file first
		err := ioutil.WriteFile(testFile, []byte(initialContent), 0644)
		require.NoError(t, err, "Failed to create test file")

		// Wait for initial creation event
		waitForWatcherEvents()
		
		// Clear channel
		for len(eventChan) > 0 {
			<-eventChan
		}

		// Modify file
		modifiedContent := `package main

func hello() {
    println("hello world")
}

func goodbye() {
    println("goodbye")
}`

		err = ioutil.WriteFile(testFile, []byte(modifiedContent), 0644)
		require.NoError(t, err, "Failed to modify test file")

		// Wait for modification event
		select {
		case event := <-eventChan:
			assert.Equal(t, watcher.EventModify, event.Type, "Should detect file modification")
			assert.Equal(t, testFile, event.Path, "Should have correct file path")
			
			// Process the event (re-index)
			err := mockIndexer.IndexFile(ctx, event.Path)
			assert.NoError(t, err, "Should re-index modified file")
			
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for file modification event")
		}
	})

	t.Run("File deletion detection", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "delete_test.go")
		testContent := `package main

func main() {
    println("to be deleted")
}`

		// Create file first
		err := ioutil.WriteFile(testFile, []byte(testContent), 0644)
		require.NoError(t, err, "Failed to create test file")

		// Index it
		mockIndexer.IndexFile(ctx, testFile)
		
		// Wait for creation event to be processed
		waitForWatcherEvents()
		
		// Clear channel
		for len(eventChan) > 0 {
			<-eventChan
		}

		// Delete file
		err = os.Remove(testFile)
		require.NoError(t, err, "Failed to delete test file")

		// Wait for deletion event
		select {
		case event := <-eventChan:
			assert.Equal(t, watcher.EventDelete, event.Type, "Should detect file deletion")
			assert.Equal(t, testFile, event.Path, "Should have correct file path")
			
			// Process the event
			err := mockIndexer.DeleteFile(ctx, event.Path)
			assert.NoError(t, err, "Should delete file from index")
			
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for file deletion event")
		}

		// Verify file was deleted from index
		assert.True(t, mockIndexer.IsDeleted(testFile), "File should be deleted from index")
	})
}

func TestFileWatcherWithRealIndexer(t *testing.T) {
	tmpDir, _, cleanup := setupWatcherTestEnvironment(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create real Zoekt indexer
	zoektIndexer := indexer.NewRealZoektIndexer(tmpDir)
	defer zoektIndexer.Close()

	// Create watcher
	w, err := watcher.NewWatcher(tmpDir)
	require.NoError(t, err, "Failed to create watcher")
	defer w.Close()

	// Start watching
	eventChan := make(chan watcher.FileEvent, 100)
	err = w.Start(ctx, eventChan)
	require.NoError(t, err, "Failed to start watcher")

	t.Run("Integration with real indexer", func(t *testing.T) {
		testFiles := map[string]string{
			"user.go": `package main

type User struct {
    ID   int    \`json:"id"\`
    Name string \`json:"name"\`
}

func (u *User) Authenticate(password string) bool {
    return password == "secret"
}`,
			"auth.py": `def authenticate(username, password):
    return username == "admin" and password == "secret"

class AuthManager:
    def __init__(self):
        self.users = {}`,
			"config.json": `{
    "database": {
        "host": "localhost",
        "port": 5432
    },
    "auth": {
        "enabled": true
    }
}`,
		}

		// Create files and process events
		for filename, content := range testFiles {
			filePath := filepath.Join(tmpDir, filename)
			
			// Create file
			err := ioutil.WriteFile(filePath, []byte(content), 0644)
			require.NoError(t, err, "Failed to create file %s", filename)

			// Wait for and process event
			select {
			case event := <-eventChan:
				assert.Equal(t, watcher.EventCreate, event.Type, "Should detect creation of %s", filename)
				
				// Index with real indexer
				err := zoektIndexer.Index(ctx, []string{event.Path})
				assert.NoError(t, err, "Should index file %s", filename)
				
			case <-time.After(2 * time.Second):
				t.Fatalf("Timeout waiting for creation event for %s", filename)
			}
		}

		// Verify indexing worked by searching
		stats := zoektIndexer.Stats()
		assert.Equal(t, 3, stats.TotalFiles, "Should have indexed 3 files")

		// Test search functionality
		searchOptions := indexer.SearchOptions{MaxResults: 10}
		results, err := zoektIndexer.Search(ctx, "authenticate", searchOptions)
		assert.NoError(t, err, "Search should succeed")
		assert.GreaterOrEqual(t, len(results), 2, "Should find authenticate function in multiple files")
	})

	t.Run("File modification with re-indexing", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "modify_real.go")
		initialContent := `package main

func oldFunction() {
    println("old implementation")
}`

		// Create and index initial file
		err := ioutil.WriteFile(testFile, []byte(initialContent), 0644)
		require.NoError(t, err, "Failed to create test file")

		// Wait for creation event and index
		select {
		case event := <-eventChan:
			err := zoektIndexer.Index(ctx, []string{event.Path})
			assert.NoError(t, err, "Should index initial file")
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for creation event")
		}

		// Search for old content
		searchOptions := indexer.SearchOptions{MaxResults: 10}
		results, err := zoektIndexer.Search(ctx, "oldFunction", searchOptions)
		assert.NoError(t, err, "Search should succeed")
		assert.Greater(t, len(results), 0, "Should find old function")

		// Modify file
		modifiedContent := `package main

func newFunction() {
    println("new implementation")
}

func authenticate(user, pass string) bool {
    return user == "admin" && pass == "secret"
}`

		err = ioutil.WriteFile(testFile, []byte(modifiedContent), 0644)
		require.NoError(t, err, "Failed to modify test file")

		// Wait for modification event and re-index
		select {
		case event := <-eventChan:
			assert.Equal(t, watcher.EventModify, event.Type, "Should detect modification")
			
			// Re-index the modified file
			err := zoektIndexer.Index(ctx, []string{event.Path})
			assert.NoError(t, err, "Should re-index modified file")
			
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for modification event")
		}

		// Search for new content
		results, err = zoektIndexer.Search(ctx, "newFunction", searchOptions)
		assert.NoError(t, err, "Search should succeed")
		assert.Greater(t, len(results), 0, "Should find new function")

		// Search for authenticate function added in modification
		results, err = zoektIndexer.Search(ctx, "authenticate", searchOptions)
		assert.NoError(t, err, "Search should succeed")
		
		// Should find the authenticate function in the modified file
		foundInModifiedFile := false
		for _, hit := range results {
			if hit.File == testFile {
				foundInModifiedFile = true
				break
			}
		}
		assert.True(t, foundInModifiedFile, "Should find authenticate function in modified file")
	})
}

func TestFileWatcherDirectoryOperations(t *testing.T) {
	tmpDir, mockIndexer, cleanup := setupWatcherTestEnvironment(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create watcher
	w, err := watcher.NewWatcher(tmpDir)
	require.NoError(t, err, "Failed to create watcher")
	defer w.Close()

	// Start watching
	eventChan := make(chan watcher.FileEvent, 100)
	err = w.Start(ctx, eventChan)
	require.NoError(t, err, "Failed to start watcher")

	t.Run("Subdirectory creation and file operations", func(t *testing.T) {
		// Create subdirectory
		subDir := filepath.Join(tmpDir, "src")
		err := os.MkdirAll(subDir, 0755)
		require.NoError(t, err, "Failed to create subdirectory")

		// Create file in subdirectory
		testFile := filepath.Join(subDir, "module.go")
		testContent := `package src

func ModuleFunction() {
    println("module function")
}`

		err = ioutil.WriteFile(testFile, []byte(testContent), 0644)
		require.NoError(t, err, "Failed to create file in subdirectory")

		// Wait for file creation event
		select {
		case event := <-eventChan:
			assert.Equal(t, watcher.EventCreate, event.Type, "Should detect file creation in subdirectory")
			assert.Equal(t, testFile, event.Path, "Should have correct file path")
			
			// Process the event
			err := mockIndexer.IndexFile(ctx, event.Path)
			assert.NoError(t, err, "Should index file in subdirectory")
			
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for file creation event in subdirectory")
		}

		assert.True(t, mockIndexer.IsIndexed(testFile), "File in subdirectory should be indexed")
	})

	t.Run("Multiple file operations", func(t *testing.T) {
		// Create multiple files rapidly
		files := []string{"file1.go", "file2.py", "file3.js"}
		
		for i, filename := range files {
			filePath := filepath.Join(tmpDir, filename)
			content := fmt.Sprintf(`// File %d
function test%d() {
    console.log("test %d");
}`, i+1, i+1, i+1)
			
			err := ioutil.WriteFile(filePath, []byte(content), 0644)
			require.NoError(t, err, "Failed to create file %s", filename)
		}

		// Process all events
		processedFiles := make(map[string]bool)
		timeout := time.After(5 * time.Second)
		
		for len(processedFiles) < len(files) {
			select {
			case event := <-eventChan:
				assert.Equal(t, watcher.EventCreate, event.Type, "Should detect creation")
				
				// Process the event
				err := mockIndexer.IndexFile(ctx, event.Path)
				assert.NoError(t, err, "Should index file")
				
				processedFiles[filepath.Base(event.Path)] = true
				
			case <-timeout:
				t.Fatalf("Timeout waiting for all file events. Processed: %v", processedFiles)
			}
		}

		// Verify all files were processed
		for _, filename := range files {
			filePath := filepath.Join(tmpDir, filename)
			assert.True(t, mockIndexer.IsIndexed(filePath), "File %s should be indexed", filename)
		}
	})
}

func TestFileWatcherErrorHandling(t *testing.T) {
	tmpDir, mockIndexer, cleanup := setupWatcherTestEnvironment(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Run("Invalid directory handling", func(t *testing.T) {
		invalidDir := "/path/that/does/not/exist"
		
		_, err := watcher.NewWatcher(invalidDir)
		assert.Error(t, err, "Should return error for invalid directory")
	})

	t.Run("Watcher behavior after directory deletion", func(t *testing.T) {
		// Create a temporary subdirectory to watch
		watchDir := filepath.Join(tmpDir, "watch_me")
		err := os.MkdirAll(watchDir, 0755)
		require.NoError(t, err, "Failed to create watch directory")

		// Create watcher for subdirectory
		w, err := watcher.NewWatcher(watchDir)
		require.NoError(t, err, "Failed to create watcher")
		defer w.Close()

		eventChan := make(chan watcher.FileEvent, 100)
		err = w.Start(ctx, eventChan)
		require.NoError(t, err, "Failed to start watcher")

		// Create a file
		testFile := filepath.Join(watchDir, "test.go")
		err = ioutil.WriteFile(testFile, []byte("package main"), 0644)
		require.NoError(t, err, "Failed to create test file")

		// Wait for event
		select {
		case event := <-eventChan:
			assert.Equal(t, watcher.EventCreate, event.Type, "Should detect file creation")
			mockIndexer.IndexFile(ctx, event.Path)
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for file creation event")
		}

		// Remove the entire watch directory
		err = os.RemoveAll(watchDir)
		require.NoError(t, err, "Failed to remove watch directory")

		// Watcher should handle this gracefully (implementation dependent)
		// At minimum, it shouldn't crash the test
		time.Sleep(100 * time.Millisecond)
	})

	t.Run("Context cancellation", func(t *testing.T) {
		w, err := watcher.NewWatcher(tmpDir)
		require.NoError(t, err, "Failed to create watcher")
		defer w.Close()

		cancelCtx, cancel := context.WithCancel(context.Background())
		eventChan := make(chan watcher.FileEvent, 100)
		
		err = w.Start(cancelCtx, eventChan)
		require.NoError(t, err, "Failed to start watcher")

		// Cancel context
		cancel()

		// Watcher should stop gracefully
		time.Sleep(100 * time.Millisecond)

		// Try to create a file - might or might not be detected depending on timing
		testFile := filepath.Join(tmpDir, "after_cancel.go")
		err = ioutil.WriteFile(testFile, []byte("package main"), 0644)
		require.NoError(t, err, "Failed to create test file")

		// Should not receive events after cancellation (or very few due to timing)
		select {
		case event := <-eventChan:
			t.Logf("Received event after cancellation (timing dependent): %+v", event)
		case <-time.After(500 * time.Millisecond):
			// This is expected - no events after cancellation
		}
	})
}

func TestFileWatcherPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	tmpDir, mockIndexer, cleanup := setupWatcherTestEnvironment(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	w, err := watcher.NewWatcher(tmpDir)
	require.NoError(t, err, "Failed to create watcher")
	defer w.Close()

	eventChan := make(chan watcher.FileEvent, 1000) // Large buffer
	err = w.Start(ctx, eventChan)
	require.NoError(t, err, "Failed to start watcher")

	t.Run("Many files creation", func(t *testing.T) {
		const numFiles = 100
		
		startTime := time.Now()
		
		// Create many files rapidly
		for i := 0; i < numFiles; i++ {
			filename := fmt.Sprintf("perf_test_%d.go", i)
			filePath := filepath.Join(tmpDir, filename)
			content := fmt.Sprintf(`package main

func function%d() {
    println("function %d")
}`, i, i)
			
			err := ioutil.WriteFile(filePath, []byte(content), 0644)
			require.NoError(t, err, "Failed to create file %d", i)
		}

		creationTime := time.Since(startTime)
		t.Logf("Created %d files in %v", numFiles, creationTime)

		// Process all events
		processedCount := 0
		eventProcessingStart := time.Now()
		
		timeout := time.After(10 * time.Second)
		for processedCount < numFiles {
			select {
			case event := <-eventChan:
				assert.Equal(t, watcher.EventCreate, event.Type, "Should detect creation")
				
				// Simulate indexing work
				err := mockIndexer.IndexFile(ctx, event.Path)
				assert.NoError(t, err, "Should index file")
				
				processedCount++
				
			case <-timeout:
				t.Fatalf("Timeout processing events. Processed %d/%d files", processedCount, numFiles)
			}
		}

		eventProcessingTime := time.Since(eventProcessingStart)
		t.Logf("Processed %d events in %v (avg: %v per event)", 
			numFiles, eventProcessingTime, eventProcessingTime/time.Duration(numFiles))

		// Verify performance is reasonable
		avgEventTime := eventProcessingTime / time.Duration(numFiles)
		assert.Less(t, avgEventTime, 50*time.Millisecond, 
			"Average event processing time should be reasonable")

		// Verify all files were indexed
		indexCount, _ := mockIndexer.GetStats()
		assert.Equal(t, numFiles, indexCount, "Should have indexed all files")
	})

	t.Run("Rapid modifications", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "rapid_modify.go")
		
		// Create initial file
		err := ioutil.WriteFile(testFile, []byte("package main\n"), 0644)
		require.NoError(t, err, "Failed to create initial file")

		// Wait for creation event
		select {
		case <-eventChan:
			mockIndexer.IndexFile(ctx, testFile)
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for creation event")
		}

		const numModifications = 20
		startTime := time.Now()

		// Rapidly modify the file
		for i := 0; i < numModifications; i++ {
			content := fmt.Sprintf("package main\n\nfunc version%d() {\n    println(\"version %d\")\n}\n", i, i)
			err := ioutil.WriteFile(testFile, []byte(content), 0644)
			require.NoError(t, err, "Failed to modify file iteration %d", i)
			
			// Small delay to avoid overwhelming the filesystem
			time.Sleep(10 * time.Millisecond)
		}

		modificationTime := time.Since(startTime)
		t.Logf("Made %d modifications in %v", numModifications, modificationTime)

		// Process modification events
		processedMods := 0
		timeout := time.After(5 * time.Second)
		
		for processedMods < numModifications {
			select {
			case event := <-eventChan:
				if event.Type == watcher.EventModify && event.Path == testFile {
					err := mockIndexer.IndexFile(ctx, event.Path)
					assert.NoError(t, err, "Should re-index modified file")
					processedMods++
				}
				
			case <-timeout:
				// Some filesystem implementations may coalesce rapid modifications
				t.Logf("Processed %d modification events (some may have been coalesced)", processedMods)
				break
			}
		}

		t.Logf("Processed %d modification events", processedMods)
		assert.Greater(t, processedMods, 0, "Should process at least some modification events")
	})
}

func TestFileWatcherFileTypeFiltering(t *testing.T) {
	tmpDir, mockIndexer, cleanup := setupWatcherTestEnvironment(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create watcher with file type filtering (if supported)
	w, err := watcher.NewWatcher(tmpDir)
	require.NoError(t, err, "Failed to create watcher")
	defer w.Close()

	eventChan := make(chan watcher.FileEvent, 100)
	err = w.Start(ctx, eventChan)
	require.NoError(t, err, "Failed to start watcher")

	t.Run("Different file types", func(t *testing.T) {
		testFiles := map[string]string{
			"code.go":       "package main\nfunc main() {}",
			"script.py":     "def main():\n    pass",
			"config.json":   `{"key": "value"}`,
			"readme.md":     "# Project\nDescription",
			"data.txt":      "plain text data",
			"image.png":     "fake png data", // Not a real PNG
			"binary.exe":    "fake binary",   // Not a real binary
			"temp.tmp":      "temporary file",
		}

		expectedCodeFiles := []string{"code.go", "script.py", "config.json", "readme.md"}

		for filename, content := range testFiles {
			filePath := filepath.Join(tmpDir, filename)
			err := ioutil.WriteFile(filePath, []byte(content), 0644)
			require.NoError(t, err, "Failed to create file %s", filename)
		}

		// Process all events
		processedFiles := make(map[string]bool)
		timeout := time.After(5 * time.Second)
		
		for len(processedFiles) < len(testFiles) {
			select {
			case event := <-eventChan:
				assert.Equal(t, watcher.EventCreate, event.Type, "Should detect creation")
				
				filename := filepath.Base(event.Path)
				processedFiles[filename] = true
				
				// Apply filtering logic (simulate what a real implementation might do)
				shouldIndex := false
				for _, expectedFile := range expectedCodeFiles {
					if filename == expectedFile {
						shouldIndex = true
						break
					}
				}

				if shouldIndex {
					err := mockIndexer.IndexFile(ctx, event.Path)
					assert.NoError(t, err, "Should index code file %s", filename)
					t.Logf("Indexed code file: %s", filename)
				} else {
					t.Logf("Skipped non-code file: %s", filename)
				}
				
			case <-timeout:
				t.Fatalf("Timeout waiting for file events. Processed: %v", processedFiles)
			}
		}

		// Verify that appropriate files were indexed
		indexCount, _ := mockIndexer.GetStats()
		assert.GreaterOrEqual(t, indexCount, len(expectedCodeFiles), 
			"Should have indexed at least the expected code files")
		
		t.Logf("Total files processed: %d, Files indexed: %d", len(testFiles), indexCount)
	})
}