package watcher

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatcher(t *testing.T) {
	// Create temp directory for testing
	tmpDir, err := os.MkdirTemp("", "watcher-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create watcher
	w, err := NewWatcher(100) // 100ms debounce
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	defer w.Close()

	// Start watching
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := w.Start(ctx); err != nil {
		t.Fatalf("Failed to start watcher: %v", err)
	}

	// Add directory to watch
	if err := w.AddPath(tmpDir); err != nil {
		t.Fatalf("Failed to add path: %v", err)
	}

	// Create a test file
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Wait for event
	select {
	case event := <-w.Events():
		if event.Path != testFile {
			t.Errorf("Expected path %s, got %s", testFile, event.Path)
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("Timeout waiting for file event")
	}
}

func TestIgnorePatterns(t *testing.T) {
	w, err := NewWatcher(100)
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	defer w.Close()

	tests := []struct {
		path     string
		expected bool
	}{
		{"/path/to/.git/config", true},
		{"/path/to/node_modules/pkg", true},
		{"/path/to/file.log", true},
		{"/path/to/source.go", false},
		{"/path/to/dist/bundle.js", true},
	}

	for _, tt := range tests {
		got := w.shouldIgnore(tt.path)
		if got != tt.expected {
			t.Errorf("shouldIgnore(%s) = %v, want %v", tt.path, got, tt.expected)
		}
	}
}

func TestWatcherEnhancedEvents(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "watcher_enhanced_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test watcher
	w, err := NewWatcher(100) // 100ms debounce
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	defer w.Close()

	// Add path to watch
	if err := w.AddPath(tmpDir); err != nil {
		t.Fatalf("Failed to add path to watcher: %v", err)
	}

	// Start watcher
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := w.Start(ctx); err != nil {
		t.Fatalf("Failed to start watcher: %v", err)
	}

	// Test file creation
	testFile := filepath.Join(tmpDir, "test.go")
	if err := os.WriteFile(testFile, []byte("package main\n"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Wait for event
	select {
	case event := <-w.Events():
		if event.Operation != OpCreate {
			t.Errorf("Expected CREATE operation, got %v", event.Operation)
		}
		if event.Path != testFile {
			t.Errorf("Expected path %s, got %s", testFile, event.Path)
		}
		if event.Hash == "" {
			t.Error("Expected hash to be set for created file")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for create event")
	}

	// Test file modification
	if err := os.WriteFile(testFile, []byte("package main\n\nfunc main() {}\n"), 0644); err != nil {
		t.Fatalf("Failed to modify test file: %v", err)
	}

	// Wait for modify event
	select {
	case event := <-w.Events():
		if event.Operation != OpModify {
			t.Errorf("Expected MODIFY operation, got %v", event.Operation)
		}
		if event.Hash == "" {
			t.Error("Expected hash to be set for modified file")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for modify event")
	}

	// Test file deletion
	if err := os.Remove(testFile); err != nil {
		t.Fatalf("Failed to delete test file: %v", err)
	}

	// Wait for delete event (may take longer due to pending rename detection)
	select {
	case event := <-w.Events():
		if event.Operation != OpDelete {
			t.Errorf("Expected DELETE operation, got %v", event.Operation)
		}
	case <-time.After(6 * time.Second): // Allow time for pending rename cleanup
		t.Fatal("Timeout waiting for delete event")
	}
}

func TestWatcherHashTracking(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "watcher_hash_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test watcher
	w, err := NewWatcher(100)
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	defer w.Close()

	// Test calculateFileHash
	testFile := filepath.Join(tmpDir, "hash_test.go")
	content1 := "package main\n"
	content2 := "package main\n\nfunc main() {}\n"

	// Create file and calculate hash
	if err := os.WriteFile(testFile, []byte(content1), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	hash1 := w.calculateFileHash(testFile)
	if hash1 == "" {
		t.Error("Expected non-empty hash for file")
	}

	// Modify file and check hash changes
	if err := os.WriteFile(testFile, []byte(content2), 0644); err != nil {
		t.Fatalf("Failed to modify test file: %v", err)
	}

	hash2 := w.calculateFileHash(testFile)
	if hash2 == "" {
		t.Error("Expected non-empty hash for modified file")
	}

	if hash1 == hash2 {
		t.Error("Expected hash to change when file content changes")
	}

	// Write same content and verify hash is same
	if err := os.WriteFile(testFile, []byte(content2), 0644); err != nil {
		t.Fatalf("Failed to write same content: %v", err)
	}

	hash3 := w.calculateFileHash(testFile)
	if hash2 != hash3 {
		t.Error("Expected same hash for same content")
	}
}

func TestWatcherInitializeFileHashes(t *testing.T) {
	// Create a temporary directory with some test files
	tmpDir, err := os.MkdirTemp("", "watcher_init_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test files
	testFiles := []string{
		"test1.go",
		"test2.js",
		"test3.txt",
		"ignored.log", // Should be ignored
	}

	for _, file := range testFiles {
		path := filepath.Join(tmpDir, file)
		content := "test content for " + file
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", file, err)
		}
	}

	// Create subdirectory with more files
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	subFile := filepath.Join(subDir, "sub.py")
	if err := os.WriteFile(subFile, []byte("print('hello')"), 0644); err != nil {
		t.Fatalf("Failed to create sub file: %v", err)
	}

	// Create watcher and initialize hashes
	w, err := NewWatcher(100)
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	defer w.Close()

	// Define code file checker
	isCodeFile := func(path string) bool {
		ext := filepath.Ext(path)
		return ext == ".go" || ext == ".js" || ext == ".py" || ext == ".txt"
	}

	// Initialize file hashes
	if err := w.InitializeFileHashes(tmpDir, isCodeFile); err != nil {
		t.Fatalf("Failed to initialize file hashes: %v", err)
	}

	// Check that hashes were created for code files
	expectedFiles := []string{
		filepath.Join(tmpDir, "test1.go"),
		filepath.Join(tmpDir, "test2.js"),
		filepath.Join(tmpDir, "test3.txt"),
		filepath.Join(subDir, "sub.py"),
	}

	w.mu.RLock()
	for _, file := range expectedFiles {
		if hash, exists := w.fileHashes[file]; !exists || hash == "" {
			t.Errorf("Expected hash for file %s, got exists=%v, hash=%s", file, exists, hash)
		}
	}

	// Check that ignored files don't have hashes
	ignoredFile := filepath.Join(tmpDir, "ignored.log")
	if _, exists := w.fileHashes[ignoredFile]; exists {
		t.Errorf("Expected ignored file %s to not have hash", ignoredFile)
	}
	w.mu.RUnlock()
}

func TestWatcherBatchedEvents(t *testing.T) {
	// Create test watcher
	w, err := NewWatcher(50)
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	defer w.Close()

	// Test timeout behavior with no events
	events := w.GetBatchedEvents(100 * time.Millisecond)
	if len(events) != 0 {
		t.Errorf("Expected 0 events, got %d", len(events))
	}

	// Add some mock events to the queue
	mockEvents := []FileEvent{
		{Path: "/test1.go", Operation: OpCreate, Timestamp: time.Now()},
		{Path: "/test2.go", Operation: OpModify, Timestamp: time.Now()},
		{Path: "/test3.go", Operation: OpDelete, Timestamp: time.Now()},
	}

	// Add events to queue
	for _, event := range mockEvents {
		select {
		case w.eventQueue <- event:
		default:
			t.Fatalf("Failed to add event to queue")
		}
	}

	// Get batched events
	events = w.GetBatchedEvents(100 * time.Millisecond)
	if len(events) != len(mockEvents) {
		t.Errorf("Expected %d events, got %d", len(mockEvents), len(events))
	}

	// Verify events match
	for i, event := range events {
		if event.Path != mockEvents[i].Path || event.Operation != mockEvents[i].Operation {
			t.Errorf("Event %d mismatch: expected %v, got %v", i, mockEvents[i], event)
		}
	}
}

func TestFileOperationString(t *testing.T) {
	tests := []struct {
		op       FileOperation
		expected string
	}{
		{OpCreate, "CREATE"},
		{OpModify, "MODIFY"},
		{OpDelete, "DELETE"},
		{OpRename, "RENAME"},
		{FileOperation(999), "UNKNOWN"},
	}

	for _, tt := range tests {
		got := tt.op.String()
		if got != tt.expected {
			t.Errorf("FileOperation(%d).String() = %s, want %s", int(tt.op), got, tt.expected)
		}
	}
}