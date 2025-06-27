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