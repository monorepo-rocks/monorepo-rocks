package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/config"
	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/embedder"
	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/indexer"
	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/query"
	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/types"
	"github.com/spf13/cobra"
)

var (
	searchQuery string
	topK        int
	language    string
	jsonOutput  bool
)

var searchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search indexed repositories",
	Long:  `Perform hybrid lexical and semantic search across indexed code.`,
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
			// Index might not exist yet
			fmt.Fprintf(os.Stderr, "Warning: FAISS index not loaded: %v\n", err)
		}

		// Create query service
		querySvc := query.NewQueryService(zoektIdx, faissIdx, emb, cfg)

		// Prepare search request
		searchReq := &types.SearchRequest{
			Query:    searchQuery,
			TopK:     topK,
			Language: language,
		}

		// Execute search
		start := time.Now()
		resp, err := querySvc.Search(ctx, searchReq)
		if err != nil {
			return fmt.Errorf("search failed: %w", err)
		}
		resp.QueryTime = time.Since(start)

		// Output results
		if jsonOutput {
			return outputJSON(resp)
		}
		return outputText(resp)
	},
}

func outputJSON(resp *types.SearchResponse) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(resp)
}

func outputText(resp *types.SearchResponse) error {
	fmt.Printf("Found %d results in %dms\n\n", resp.TotalHits, resp.QueryTime.Milliseconds())

	for i, hit := range resp.Hits {
		fmt.Printf("%d. %s:%d (score: %.3f, source: %s)\n", 
			i+1, hit.File, hit.LineNumber, hit.Score, hit.Source)
		fmt.Printf("   %s\n\n", hit.Text)
	}

	if resp.TotalHits == 0 {
		fmt.Println("No results found.")
	}

	return nil
}

func init() {
	searchCmd.Flags().StringVarP(&searchQuery, "query", "q", "", "Search query (required)")
	searchCmd.Flags().IntVarP(&topK, "top", "k", 20, "Number of results to return")
	searchCmd.Flags().StringVarP(&language, "lang", "l", "", "Filter by language")
	searchCmd.Flags().BoolVar(&jsonOutput, "json", false, "Output results as JSON")
	searchCmd.MarkFlagRequired("query")

	rootCmd.AddCommand(searchCmd)
}