package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/api"
	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/config"
	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/embedder"
	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/indexer"
	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/query"
	"github.com/spf13/cobra"
)

var (
	port string
	host string
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start HTTP API server",
	Long:  `Start the HTTP API server for REST-based search queries.`,
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

		// Create API server
		portInt := 8080
		if port != "" {
			fmt.Sscanf(port, "%d", &portInt)
		}
		apiServer := api.NewServer(querySvc, portInt)

		// Handle shutdown
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-sigChan
			fmt.Println("\nShutting down server...")
			cancel()
		}()

		// Start server
		fmt.Printf("Starting API server on %s:%s\n", host, port)
		return apiServer.Start()
	},
}

func init() {
	serveCmd.Flags().StringVarP(&port, "port", "p", "8080", "Port to listen on")
	serveCmd.Flags().StringVar(&host, "host", "0.0.0.0", "Host to bind to")

	rootCmd.AddCommand(serveCmd)
}