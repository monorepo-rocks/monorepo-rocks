package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/config"
	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/embedder"
	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/indexer"
	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/mcp"
	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/query"
	"github.com/spf13/cobra"
)

var stdioCmd = &cobra.Command{
	Use:   "stdio",
	Short: "Run in MCP stdio mode",
	Long:  `Start the engine in MCP (Model Context Protocol) stdio mode for LLM agent integration.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load configuration
		cfg, err := config.Load(configFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Initialize components
		indexPath := filepath.Join(cfg.IndexRoot, "indexes")
		zoektIdx := indexer.NewZoektIndexer(indexPath)
		faissIdx := indexer.NewFAISSIndexer(indexPath, 768)
		emb := embedder.NewDefaultEmbedder()

		// Load indexes
		ctx := context.Background()
		if err := faissIdx.Load(ctx, filepath.Join(indexPath, "faiss.index")); err != nil {
			// Index might not exist yet, that's OK
			fmt.Fprintf(os.Stderr, "Warning: FAISS index not loaded: %v\n", err)
		}

		// Create query service
		querySvc := query.NewQueryService(zoektIdx, faissIdx, emb, cfg)

		// Create MCP server
		mcpServer := mcp.NewServer(querySvc)

		// Handle shutdown
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-sigChan
			cancel()
		}()

		// Run server
		return mcpServer.Run(ctx)
	},
}

func init() {
	rootCmd.AddCommand(stdioCmd)
}