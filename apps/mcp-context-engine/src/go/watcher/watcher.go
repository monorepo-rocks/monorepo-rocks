package watcher

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// FileEvent represents a file change event
type FileEvent struct {
	Path      string
	Operation string
	Timestamp time.Time
}

// Watcher monitors file system changes
type Watcher struct {
	fsWatcher    *fsnotify.Watcher
	eventQueue   chan FileEvent
	debounceTime time.Duration
	ignorePatterns []string
	mu           sync.RWMutex
	watchedDirs  map[string]bool
}

// NewWatcher creates a new file system watcher
func NewWatcher(debounceMs int) (*Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create fs watcher: %w", err)
	}

	return &Watcher{
		fsWatcher:    fsWatcher,
		eventQueue:   make(chan FileEvent, 1000),
		debounceTime: time.Duration(debounceMs) * time.Millisecond,
		watchedDirs:  make(map[string]bool),
		ignorePatterns: []string{
			".git",
			"node_modules",
			".cache",
			"bin",
			"dist",
			"*.log",
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

					// Send event
					select {
					case w.eventQueue <- FileEvent{
						Path:      event.Name,
						Operation: event.Op.String(),
						Timestamp: time.Now(),
					}:
					default:
						// Queue full, drop event
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