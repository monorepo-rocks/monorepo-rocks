package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/config"
	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/embedder"
	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/indexer"
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
		faissIdx := indexer.NewFAISSIndex(indexPath)
		emb := embedder.NewEmbedder(embedder.DefaultConfig())

		ctx := context.Background()

		// Initial indexing
		if err := performIndexing(ctx, repoPath, zoektIdx, faissIdx, emb); err != nil {
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

func performIndexing(ctx context.Context, repoPath string, zoekt *indexer.ZoektIndexer, faiss *indexer.FAISSIndex, emb embedder.Embedder) error {
	// Walk the repository
	var files []string
	err := filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if !info.IsDir() && isCodeFile(path) {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return err
	}

	fmt.Printf("Found %d code files\n", len(files))

	// Index with Zoekt
	fmt.Println("Building lexical index...")
	for _, file := range files {
		if err := zoekt.IndexFile(ctx, file); err != nil {
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

		// Generate chunks
		chunks := embedder.ChunkCode(string(content), detectLanguage(file))
		for _, chunk := range chunks {
			// Generate embedding
			vec, err := emb.Embed(ctx, chunk.Text)
			if err != nil {
				continue
			}

			// Add to FAISS
			if err := faiss.AddVector(ctx, chunk.Hash, vec); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to add vector: %v\n", err)
			}
		}
	}

	// Save indexes
	if err := faiss.Save(ctx); err != nil {
		return fmt.Errorf("failed to save FAISS index: %w", err)
	}

	fmt.Println("Indexing complete!")
	return nil
}

func runWatcher(ctx context.Context, repoPath string, cfg *config.Config, zoekt *indexer.ZoektIndexer, faiss *indexer.FAISSIndex, emb embedder.Embedder) error {
	w, err := watcher.NewWatcher(cfg.Watcher.DebounceMs)
	if err != nil {
		return err
	}
	defer w.Close()

	if err := w.AddPath(repoPath); err != nil {
		return err
	}

	if err := w.Start(ctx); err != nil {
		return err
	}

	// Process file events
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event := <-w.Events():
			fmt.Printf("File changed: %s\n", event.Path)
			// Re-index the file
			if isCodeFile(event.Path) {
				if err := zoekt.IndexFile(ctx, event.Path); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to re-index %s: %v\n", event.Path, err)
				}
			}
		}
	}
}

func isCodeFile(path string) bool {
	ext := filepath.Ext(path)
	codeExts := map[string]bool{
		".go": true, ".js": true, ".ts": true, ".tsx": true,
		".py": true, ".java": true, ".c": true, ".cpp": true,
		".h": true, ".hpp": true, ".rs": true, ".rb": true,
	}
	return codeExts[ext]
}

func detectLanguage(path string) string {
	ext := filepath.Ext(path)
	langMap := map[string]string{
		".go": "go", ".js": "javascript", ".ts": "typescript",
		".py": "python", ".java": "java", ".c": "c",
		".cpp": "cpp", ".rs": "rust", ".rb": "ruby",
	}
	if lang, ok := langMap[ext]; ok {
		return lang
	}
	return "unknown"
}

func init() {
	indexCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch for file changes")
	indexCmd.Flags().BoolVarP(&forceFlag, "force", "f", false, "Force full re-indexing")
	rootCmd.AddCommand(indexCmd)
}