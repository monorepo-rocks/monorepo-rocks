package watcher

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// FileOperation represents the type of file operation
type FileOperation int

const (
	OpCreate FileOperation = iota
	OpModify
	OpDelete
	OpRename
)

func (op FileOperation) String() string {
	switch op {
	case OpCreate:
		return "CREATE"
	case OpModify:
		return "MODIFY"
	case OpDelete:
		return "DELETE"
	case OpRename:
		return "RENAME"
	default:
		return "UNKNOWN"
	}
}

// FileEvent represents a file change event with enhanced information
type FileEvent struct {
	Path      string        `json:"path"`
	OldPath   string        `json:"old_path,omitempty"` // For rename operations
	Operation FileOperation `json:"operation"`
	Timestamp time.Time     `json:"timestamp"`
	Hash      string        `json:"hash,omitempty"`     // SHA-256 hash of file content
	Size      int64         `json:"size,omitempty"`     // File size in bytes
}

// Watcher monitors file system changes
type Watcher struct {
	fsWatcher      *fsnotify.Watcher
	eventQueue     chan FileEvent
	debounceTime   time.Duration
	ignorePatterns []string
	mu             sync.RWMutex
	watchedDirs    map[string]bool
	fileHashes     map[string]string    // Track file content hashes
	pendingRenames map[string]time.Time // Track potential rename operations
	batchSize      int                  // Maximum events to batch together
	batchTimeout   time.Duration        // Maximum time to wait for batching
}

// NewWatcher creates a new file system watcher
func NewWatcher(debounceMs int) (*Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create fs watcher: %w", err)
	}

	return &Watcher{
		fsWatcher:      fsWatcher,
		eventQueue:     make(chan FileEvent, 1000),
		debounceTime:   time.Duration(debounceMs) * time.Millisecond,
		watchedDirs:    make(map[string]bool),
		fileHashes:     make(map[string]string),
		pendingRenames: make(map[string]time.Time),
		batchSize:      10,                        // Process up to 10 events at once
		batchTimeout:   100 * time.Millisecond,   // Wait max 100ms for batching
		ignorePatterns: []string{
			".git",
			"node_modules",
			".cache",
			"bin",
			"dist",
			"*.log",
			"*.faiss",
			"*.zoekt",
			"*.index",
		},
	}, nil
}

// AddPath adds a path to watch (recursively for directories)
func (w *Watcher) AddPath(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to stat path %s: %w", path, err)
	}

	if info.IsDir() {
		return w.addDirectory(path)
	}

	dir := filepath.Dir(path)
	return w.addDirectory(dir)
}

// addDirectory recursively adds a directory and its subdirectories
func (w *Watcher) addDirectory(dir string) error {
	// Check if already watching
	w.mu.RLock()
	if w.watchedDirs[dir] {
		w.mu.RUnlock()
		return nil
	}
	w.mu.RUnlock()

	// Walk directory tree
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Skip ignored paths
		if w.shouldIgnore(path) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Add directories to watcher
		if info.IsDir() {
			if err := w.fsWatcher.Add(path); err != nil {
				return fmt.Errorf("failed to watch %s: %w", path, err)
			}
			
			w.mu.Lock()
			w.watchedDirs[path] = true
			w.mu.Unlock()
		}

		return nil
	})

	return err
}

// shouldIgnore checks if a path should be ignored
func (w *Watcher) shouldIgnore(path string) bool {
	base := filepath.Base(path)
	
	for _, pattern := range w.ignorePatterns {
		if strings.HasPrefix(pattern, "*") {
			if strings.HasSuffix(base, pattern[1:]) {
				return true
			}
		} else if base == pattern || strings.Contains(path, "/"+pattern+"/") {
			return true
		}
	}
	
	return false
}

// Start begins watching for file changes
func (w *Watcher) Start(ctx context.Context) error {
	// Debouncer to batch rapid file changes
	debouncer := make(map[string]*time.Timer)
	var debounceSync sync.Mutex

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-w.fsWatcher.Events:
				if !ok {
					return
				}

				// Skip ignored files
				if w.shouldIgnore(event.Name) {
					continue
				}

				// Handle event with debouncing
				debounceSync.Lock()
				if timer, exists := debouncer[event.Name]; exists {
					timer.Stop()
				}

				debouncer[event.Name] = time.AfterFunc(w.debounceTime, func() {
					debounceSync.Lock()
					delete(debouncer, event.Name)
					debounceSync.Unlock()

					// Process the file system event and create enhanced FileEvent
					if fileEvent := w.processFileSystemEvent(event); fileEvent != nil {
						select {
						case w.eventQueue <- *fileEvent:
						default:
							// Queue full, drop event
							fmt.Fprintf(os.Stderr, "Warning: Event queue full, dropping event for %s\n", event.Name)
						}
					}
				})
				debounceSync.Unlock()

			case err, ok := <-w.fsWatcher.Errors:
				if !ok {
					return
				}
				// Log error but continue watching
				fmt.Fprintf(os.Stderr, "Watcher error: %v\n", err)
			}
		}
	}()

	// Start cleanup routine for pending renames
	go w.cleanupPendingRenames(ctx)

	return nil
}

// Events returns the event channel
func (w *Watcher) Events() <-chan FileEvent {
	return w.eventQueue
}

// Close stops the watcher
func (w *Watcher) Close() error {
	close(w.eventQueue)
	return w.fsWatcher.Close()
}

// SetIgnorePatterns updates the ignore patterns
func (w *Watcher) SetIgnorePatterns(patterns []string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.ignorePatterns = patterns
}

// processFileSystemEvent converts an fsnotify.Event to an enhanced FileEvent
func (w *Watcher) processFileSystemEvent(event fsnotify.Event) *FileEvent {
	w.mu.Lock()
	defer w.mu.Unlock()

	now := time.Now()
	var operation FileOperation
	var hash string
	var size int64
	var oldPath string

	// Determine operation type
	switch {
	case event.Op&fsnotify.Create != 0:
		operation = OpCreate
		// Check if this is actually a rename (CREATE after REMOVE)
		if pendingTime, exists := w.pendingRenames[event.Name]; exists {
			if now.Sub(pendingTime) < 100*time.Millisecond {
				operation = OpRename
				// Find the original path that was removed
				for path, timestamp := range w.pendingRenames {
					if path != event.Name && now.Sub(timestamp) < 100*time.Millisecond {
						oldPath = path
						delete(w.pendingRenames, path)
						break
					}
				}
			}
			delete(w.pendingRenames, event.Name)
		}
	case event.Op&fsnotify.Write != 0:
		operation = OpModify
	case event.Op&fsnotify.Remove != 0:
		operation = OpDelete
		// Mark as potential rename
		w.pendingRenames[event.Name] = now
		// Clean up old hash
		delete(w.fileHashes, event.Name)
		// Don't process immediately - wait to see if it's a rename
		return nil
	case event.Op&fsnotify.Rename != 0:
		operation = OpRename
		// fsnotify.Rename is platform-specific and may not always fire
		delete(w.fileHashes, event.Name)
	default:
		return nil // Unknown operation
	}

	// Skip if file doesn't exist (for delete operations, we already handled above)
	if operation != OpDelete {
		if stat, err := os.Stat(event.Name); err == nil {
			size = stat.Size()
			
			// Calculate hash for content change detection
			if currentHash := w.calculateFileHash(event.Name); currentHash != "" {
				previousHash, existed := w.fileHashes[event.Name]
				
				// If it's a modify operation but hash hasn't changed, skip
				if operation == OpModify && existed && currentHash == previousHash {
					return nil
				}
				
				hash = currentHash
				w.fileHashes[event.Name] = currentHash
			}
		} else if operation != OpDelete {
			// File doesn't exist and it's not a delete - skip
			return nil
		}
	}

	return &FileEvent{
		Path:      event.Name,
		OldPath:   oldPath,
		Operation: operation,
		Timestamp: now,
		Hash:      hash,
		Size:      size,
	}
}

// calculateFileHash computes SHA-256 hash of file content
func (w *Watcher) calculateFileHash(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()

	// Don't hash large files to avoid performance issues
	if stat, err := file.Stat(); err == nil && stat.Size() > 10*1024*1024 { // 10MB limit
		return fmt.Sprintf("large-file-%d-%d", stat.Size(), stat.ModTime().Unix())
	}

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return ""
	}

	return fmt.Sprintf("%x", hash.Sum(nil))
}

// cleanupPendingRenames removes old pending rename entries
func (w *Watcher) cleanupPendingRenames(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.mu.Lock()
			now := time.Now()
			for path, timestamp := range w.pendingRenames {
				if now.Sub(timestamp) > 5*time.Second {
					// This was actually a delete, not a rename
					fileEvent := &FileEvent{
						Path:      path,
						Operation: OpDelete,
						Timestamp: timestamp,
					}
					
					select {
					case w.eventQueue <- *fileEvent:
					default:
						// Queue full, skip
					}
					
					delete(w.pendingRenames, path)
				}
			}
			w.mu.Unlock()
		}
	}
}

// GetBatchedEvents returns multiple events for efficient processing
func (w *Watcher) GetBatchedEvents(timeout time.Duration) []FileEvent {
	var events []FileEvent
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	// Get first event (blocking)
	select {
	case event := <-w.eventQueue:
		events = append(events, event)
	case <-timer.C:
		return events
	}

	// Get additional events (non-blocking) up to batch size
	for len(events) < w.batchSize {
		select {
		case event := <-w.eventQueue:
			events = append(events, event)
		case <-time.After(w.batchTimeout):
			return events
		default:
			return events
		}
	}

	return events
}

// InitializeFileHashes populates the initial file hash cache
func (w *Watcher) InitializeFileHashes(rootPath string, isCodeFile func(string) bool) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	return filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		if w.shouldIgnore(path) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if !info.IsDir() && isCodeFile(path) {
			if hash := w.calculateFileHash(path); hash != "" {
				w.fileHashes[path] = hash
			}
		}

		return nil
	})
}