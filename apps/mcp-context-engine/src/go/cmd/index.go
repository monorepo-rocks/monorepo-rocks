package cmd

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/config"
	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/embedder"
	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/indexer"
	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/types"
	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/watcher"
	"github.com/spf13/cobra"
)

var (
	watchFlag bool
	forceFlag bool
)

var indexCmd = &cobra.Command{
	Use:   "index [path]",
	Short: "Index a repository for search",
	Long:  `Build or update search indexes for the specified repository path.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repoPath, err := filepath.Abs(args[0])
		if err != nil {
			return fmt.Errorf("invalid path: %w", err)
		}

		// Load configuration
		cfg, err := config.Load(configFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Create index directory
		indexPath := filepath.Join(cfg.IndexRoot, "indexes")
		if err := os.MkdirAll(indexPath, 0755); err != nil {
			return fmt.Errorf("failed to create index directory: %w", err)
		}

		fmt.Printf("Indexing repository: %s\n", repoPath)

		// Initialize components
		zoektIdx := indexer.NewZoektIndexer(indexPath)
		faissIdx := indexer.NewFAISSIndexer(indexPath, 768) // 768-dim for CodeBERT
		emb := embedder.NewDefaultEmbedder()

		ctx := context.Background()

		// Initial indexing
		if err := performIndexing(ctx, repoPath, zoektIdx, faissIdx, emb, indexPath); err != nil {
			return fmt.Errorf("indexing failed: %w", err)
		}

		// Watch mode
		if watchFlag {
			fmt.Println("Watching for changes...")
			return runWatcher(ctx, repoPath, cfg, zoektIdx, faissIdx, emb)
		}

		return nil
	},
}

func performIndexing(ctx context.Context, repoPath string, zoekt indexer.ZoektIndexer, faiss indexer.FAISSIndexer, emb embedder.Embedder, indexPath string) error {
	// Walk the repository respecting .gitignore
	files, err := WalkCodeFiles(repoPath, isCodeFile)
	if err != nil {
		return fmt.Errorf("failed to walk repository: %w", err)
	}

	fmt.Printf("Found %d code files\n", len(files))

	// Index with Zoekt
	fmt.Println("Building lexical index...")
	for _, file := range files {
		if err := zoekt.Index(ctx, []string{file}); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to index %s: %v\n", file, err)
		}
	}

	// Build FAISS index
	fmt.Println("Building semantic index...")
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		// Generate chunks using structure-aware chunking
		// Use larger chunk size for structured files (JSON/YAML) to reduce fragmentation
		language := detectLanguage(file)
		chunkSize := 300 // Default chunk size
		if language == "json" || language == "yaml" || language == "yml" {
			chunkSize = 1000 // Larger chunk size for structured files
		}
		chunks := embedder.ChunkCode(file, file, string(content), language, chunkSize)
		embeddings := make([]types.Embedding, 0, len(chunks))
		for _, chunk := range chunks {
			// Generate embedding
			emb, err := emb.EmbedSingle(ctx, chunk)
			if err != nil {
				continue
			}
			embeddings = append(embeddings, emb)
		}
		// Add embeddings to FAISS after collecting them
		if len(embeddings) > 0 {
			if err := faiss.AddVectors(ctx, embeddings); err != nil {
				return fmt.Errorf("failed to add vectors: %w", err)
			}
		}
	}

	// Save indexes to the configured index directory
	if err := faiss.Save(ctx, filepath.Join(indexPath, "faiss.index")); err != nil {
		return fmt.Errorf("failed to save FAISS index: %w", err)
	}
	
	if err := zoekt.Save(ctx, filepath.Join(indexPath, "zoekt.index")); err != nil {
		return fmt.Errorf("failed to save Zoekt index: %w", err)
	}

	fmt.Println("Indexing complete!")
	return nil
}

func runWatcher(ctx context.Context, repoPath string, cfg *config.Config, zoekt indexer.ZoektIndexer, faiss indexer.FAISSIndexer, emb embedder.Embedder) error {
	w, err := watcher.NewWatcher(cfg.Watcher.DebounceMs)
	if err != nil {
		return err
	}
	defer w.Close()

	if err := w.AddPath(repoPath); err != nil {
		return err
	}

	// Initialize file hash cache for change detection
	fmt.Println("Initializing file hash cache...")
	if err := w.InitializeFileHashes(repoPath, isCodeFile); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to initialize file hashes: %v\n", err)
	}

	if err := w.Start(ctx); err != nil {
		return err
	}

	fmt.Println("Watcher started. Processing file events...")
	
	// Enhanced event processing with batching and proper incremental updates
	var consecutiveErrors int
	maxConsecutiveErrors := 5
	errorBackoffDuration := time.Second
	
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Get batched events for efficient processing
			events := w.GetBatchedEvents(1 * time.Second)
			if len(events) == 0 {
				continue
			}

			// Process events in batch with error handling and recovery
			if err := processBatchedEvents(ctx, events, zoekt, faiss, emb); err != nil {
				consecutiveErrors++
				fmt.Fprintf(os.Stderr, "Error processing events (attempt %d/%d): %v\n", consecutiveErrors, maxConsecutiveErrors, err)
				
				// Implement exponential backoff for consecutive errors
				if consecutiveErrors >= maxConsecutiveErrors {
					fmt.Fprintf(os.Stderr, "Too many consecutive errors. Backing off for %v\n", errorBackoffDuration)
					time.Sleep(errorBackoffDuration)
					errorBackoffDuration *= 2
					if errorBackoffDuration > 30*time.Second {
						errorBackoffDuration = 30 * time.Second
					}
					consecutiveErrors = 0
				}
			} else {
				// Reset error tracking on successful processing
				consecutiveErrors = 0
				errorBackoffDuration = time.Second
			}
		}
	}
}

// processBatchedEvents handles a batch of file events efficiently
func processBatchedEvents(ctx context.Context, events []watcher.FileEvent, zoekt indexer.ZoektIndexer, faiss indexer.FAISSIndexer, emb embedder.Embedder) error {
	var filesToIndex []string
	var filesToDelete []string
	var chunkIDsToDelete []string
	var processingErrors []string

	fmt.Printf("Processing batch of %d events\n", len(events))

	// Pre-process events and validate file accessibility
	for _, event := range events {
		fmt.Printf("Event: %s %s", event.Operation, event.Path)
		if event.OldPath != "" {
			fmt.Printf(" (renamed from %s)", event.OldPath)
		}
		fmt.Println()

		// Skip non-code files
		if !isCodeFile(event.Path) {
			continue
		}

		// Validate file for operations that require file access
		if event.Operation == watcher.OpCreate || event.Operation == watcher.OpModify {
			if _, err := os.Stat(event.Path); err != nil {
				processingErrors = append(processingErrors, fmt.Sprintf("Cannot access file %s: %v", event.Path, err))
				continue
			}
		}

		switch event.Operation {
		case watcher.OpCreate, watcher.OpModify:
			filesToIndex = append(filesToIndex, event.Path)

		case watcher.OpDelete:
			filesToDelete = append(filesToDelete, event.Path)
			// Generate chunk IDs that would have been created for this file
			chunkIDsToDelete = append(chunkIDsToDelete, generateChunkIDsForFile(event.Path)...)

		case watcher.OpRename:
			// Handle rename as delete old + create new
			if event.OldPath != "" && isCodeFile(event.OldPath) {
				filesToDelete = append(filesToDelete, event.OldPath)
				chunkIDsToDelete = append(chunkIDsToDelete, generateChunkIDsForFile(event.OldPath)...)
			}
			filesToIndex = append(filesToIndex, event.Path)
		}
	}

	var hasErrors bool

	// Process deletions first with error handling
	if len(filesToDelete) > 0 {
		fmt.Printf("Deleting %d files from indexes\n", len(filesToDelete))
		
		// Delete from Zoekt with retry logic
		if err := deleteFromZoektWithRetry(ctx, zoekt, filesToDelete, 3); err != nil {
			processingErrors = append(processingErrors, fmt.Sprintf("Failed to delete files from Zoekt after retries: %v", err))
			hasErrors = true
		}

		// Delete from FAISS with retry logic
		if len(chunkIDsToDelete) > 0 {
			if err := deleteFromFAISSWithRetry(ctx, faiss, chunkIDsToDelete, 3); err != nil {
				processingErrors = append(processingErrors, fmt.Sprintf("Failed to delete chunks from FAISS after retries: %v", err))
				hasErrors = true
			}
		}
	}

	// Process additions/modifications with error handling
	if len(filesToIndex) > 0 {
		fmt.Printf("Indexing %d files\n", len(filesToIndex))
		
		// Re-index in Zoekt with retry logic
		if err := indexInZoektWithRetry(ctx, zoekt, filesToIndex, 3); err != nil {
			processingErrors = append(processingErrors, fmt.Sprintf("Failed to incrementally index files in Zoekt after retries: %v", err))
			hasErrors = true
		}

		// Re-index in FAISS with retry logic
		if err := indexFilesInFAISSWithRetry(ctx, filesToIndex, faiss, emb, 3); err != nil {
			processingErrors = append(processingErrors, fmt.Sprintf("Failed to index files in FAISS after retries: %v", err))
			hasErrors = true
		}
	}

	// Report any processing errors
	if len(processingErrors) > 0 {
		for _, errMsg := range processingErrors {
			fmt.Fprintf(os.Stderr, "Processing error: %s\n", errMsg)
		}
	}

	// Return an error only if critical operations failed
	if hasErrors {
		return fmt.Errorf("batch processing completed with %d errors", len(processingErrors))
	}

	return nil
}

// indexFilesInFAISS handles indexing files in the FAISS vector database
func indexFilesInFAISS(ctx context.Context, files []string, faiss indexer.FAISSIndexer, emb embedder.Embedder) error {
	for _, file := range files {
		// First, delete any existing chunks for this file
		existingChunkIDs := generateChunkIDsForFile(file)
		if len(existingChunkIDs) > 0 {
			if err := faiss.Delete(ctx, existingChunkIDs); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Failed to delete existing chunks for %s: %v\n", file, err)
			}
		}

		// Read file content
		content, err := os.ReadFile(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to read %s: %v\n", file, err)
			continue
		}

		// Generate chunks using structure-aware chunking
		language := detectLanguage(file)
		chunkSize := 300 // Default chunk size
		if language == "json" || language == "yaml" || language == "yml" {
			chunkSize = 1000 // Larger chunk size for structured files
		}
		
		chunks := embedder.ChunkCode(file, file, string(content), language, chunkSize)
		if len(chunks) == 0 {
			continue
		}

		// Generate embeddings
		embeddings := make([]types.Embedding, 0, len(chunks))
		for _, chunk := range chunks {
			emb_result, err := emb.EmbedSingle(ctx, chunk)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to generate embedding for chunk in %s: %v\n", file, err)
				continue
			}
			embeddings = append(embeddings, emb_result)
		}

		// Add embeddings to FAISS
		if len(embeddings) > 0 {
			if err := faiss.AddVectors(ctx, embeddings); err != nil {
				return fmt.Errorf("failed to add vectors for %s: %w", file, err)
			}
		}
	}

	return nil
}

// generateChunkIDsForFile generates the chunk IDs that would be created for a file
// This is used for deletion - we need to predict what chunk IDs were created
func generateChunkIDsForFile(filePath string) []string {
	// This is a simplified approach. In a real implementation, you might:
	// 1. Query the FAISS index for chunks with this file path
	// 2. Maintain a separate mapping of file -> chunk IDs
	// 3. Use a deterministic chunk ID generation scheme
	
	// For now, we'll use a simple approach based on file path hash
	// This assumes chunk IDs are generated deterministically
	var chunkIDs []string
	
	// Generate some potential chunk IDs based on file path
	// This is a simplified approach - in practice you'd want to maintain
	// a proper mapping or query the index
	baseID := fmt.Sprintf("%x", sha256.Sum256([]byte(filePath)))
	
	// Estimate chunks based on typical file size (conservative estimate)
	// Assume up to 20 chunks per file on average
	for i := 0; i < 20; i++ {
		chunkID := fmt.Sprintf("%s_%d", baseID, i)
		chunkIDs = append(chunkIDs, chunkID)
	}
	
	return chunkIDs
}

// Retry helper functions for error handling and recovery

// deleteFromZoektWithRetry attempts to delete files from Zoekt with retry logic
func deleteFromZoektWithRetry(ctx context.Context, zoekt indexer.ZoektIndexer, files []string, maxRetries int) error {
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if err := zoekt.Delete(ctx, files); err != nil {
			lastErr = err
			if attempt < maxRetries-1 {
				waitTime := time.Duration(attempt+1) * time.Second
				fmt.Fprintf(os.Stderr, "Zoekt delete attempt %d failed, retrying in %v: %v\n", attempt+1, waitTime, err)
				time.Sleep(waitTime)
				continue
			}
		} else {
			return nil
		}
	}
	return fmt.Errorf("failed after %d attempts: %w", maxRetries, lastErr)
}

// deleteFromFAISSWithRetry attempts to delete chunks from FAISS with retry logic
func deleteFromFAISSWithRetry(ctx context.Context, faiss indexer.FAISSIndexer, chunkIDs []string, maxRetries int) error {
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if err := faiss.Delete(ctx, chunkIDs); err != nil {
			lastErr = err
			if attempt < maxRetries-1 {
				waitTime := time.Duration(attempt+1) * time.Second
				fmt.Fprintf(os.Stderr, "FAISS delete attempt %d failed, retrying in %v: %v\n", attempt+1, waitTime, err)
				time.Sleep(waitTime)
				continue
			}
		} else {
			return nil
		}
	}
	return fmt.Errorf("failed after %d attempts: %w", maxRetries, lastErr)
}

// indexInZoektWithRetry attempts to index files in Zoekt with retry logic
func indexInZoektWithRetry(ctx context.Context, zoekt indexer.ZoektIndexer, files []string, maxRetries int) error {
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if err := zoekt.IncrementalIndex(ctx, files); err != nil {
			lastErr = err
			if attempt < maxRetries-1 {
				waitTime := time.Duration(attempt+1) * time.Second
				fmt.Fprintf(os.Stderr, "Zoekt index attempt %d failed, retrying in %v: %v\n", attempt+1, waitTime, err)
				time.Sleep(waitTime)
				continue
			}
		} else {
			return nil
		}
	}
	return fmt.Errorf("failed after %d attempts: %w", maxRetries, lastErr)
}

// indexFilesInFAISSWithRetry attempts to index files in FAISS with retry logic
func indexFilesInFAISSWithRetry(ctx context.Context, files []string, faiss indexer.FAISSIndexer, emb embedder.Embedder, maxRetries int) error {
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if err := indexFilesInFAISS(ctx, files, faiss, emb); err != nil {
			lastErr = err
			if attempt < maxRetries-1 {
				waitTime := time.Duration(attempt+1) * time.Second
				fmt.Fprintf(os.Stderr, "FAISS index attempt %d failed, retrying in %v: %v\n", attempt+1, waitTime, err)
				time.Sleep(waitTime)
				continue
			}
		} else {
			return nil
		}
	}
	return fmt.Errorf("failed after %d attempts: %w", maxRetries, lastErr)
}

func isCodeFile(path string) bool {
	ext := filepath.Ext(path)
	codeExts := map[string]bool{
		// Programming languages
		".go": true, ".js": true, ".ts": true, ".tsx": true,
		".py": true, ".java": true, ".c": true, ".cpp": true,
		".h": true, ".hpp": true, ".rs": true, ".rb": true,
		".jsx": true, ".php": true, ".cs": true, ".kt": true,
		".swift": true, ".scala": true, ".clj": true, ".hs": true,
		// Configuration and data files
		".json": true, ".yaml": true, ".yml": true,
		".xml": true, ".toml": true, ".ini": true,
		// Documentation
		".md": true, ".rst": true, ".txt": true,
		// Build and project files
		".dockerfile": true, ".gitignore": true, ".gitattributes": true,
		".makefile": true, ".cmake": true,
		// Web technologies
		".html": true, ".css": true, ".scss": true, ".sass": true,
		".less": true, ".vue": true, ".svelte": true,
	}
	// Special case for files without extensions that are commonly config files
	basename := filepath.Base(path)
	specialFiles := map[string]bool{
		"Dockerfile": true, "Makefile": true, "CMakeLists.txt": true,
		"package.json": true, "tsconfig.json": true, "composer.json": true,
		"Cargo.toml": true, "pyproject.toml": true, "go.mod": true,
		"go.sum": true, "requirements.txt": true, "Pipfile": true,
	}
	return codeExts[ext] || specialFiles[basename]
}

func detectLanguage(path string) string {
	ext := filepath.Ext(path)
	basename := filepath.Base(path)
	
	langMap := map[string]string{
		// Programming languages
		".go": "go", ".js": "javascript", ".ts": "typescript",
		".tsx": "typescript", ".jsx": "javascript",
		".py": "python", ".java": "java", ".c": "c",
		".cpp": "cpp", ".h": "c", ".hpp": "cpp",
		".rs": "rust", ".rb": "ruby", ".php": "php",
		".cs": "csharp", ".kt": "kotlin", ".swift": "swift",
		".scala": "scala", ".clj": "clojure", ".hs": "haskell",
		// Configuration and data formats
		".json": "json", ".yaml": "yaml", ".yml": "yaml",
		".xml": "xml", ".toml": "toml", ".ini": "ini",
		// Documentation
		".md": "markdown", ".rst": "restructuredtext", ".txt": "text",
		// Web technologies
		".html": "html", ".css": "css", ".scss": "scss",
		".sass": "sass", ".less": "less", ".vue": "vue",
		".svelte": "svelte",
		// Build files
		".dockerfile": "dockerfile", ".makefile": "makefile",
		".cmake": "cmake", ".gitignore": "gitignore",
	}
	
	// Check special files without extensions
	specialFiles := map[string]string{
		"Dockerfile": "dockerfile", "Makefile": "makefile",
		"CMakeLists.txt": "cmake", "go.mod": "go-mod",
		"go.sum": "go-sum", "Cargo.toml": "toml",
		"pyproject.toml": "toml", "requirements.txt": "text",
		"Pipfile": "toml",
	}
	
	if lang, ok := specialFiles[basename]; ok {
		return lang
	}
	if lang, ok := langMap[ext]; ok {
		return lang
	}
	return "text"
}

func init() {
	indexCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch for file changes")
	indexCmd.Flags().BoolVarP(&forceFlag, "force", "f", false, "Force full re-indexing")
	rootCmd.AddCommand(indexCmd)
}