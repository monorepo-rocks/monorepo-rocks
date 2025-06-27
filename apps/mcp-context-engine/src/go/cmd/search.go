package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	searchQuery string
	topK        int
	language    string
)

var searchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search indexed repositories",
	Long:  `Perform hybrid lexical and semantic search across indexed code.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Searching for: %s\n", searchQuery)
		// TODO: Implement search logic
		return nil
	},
}

func init() {
	searchCmd.Flags().StringVarP(&searchQuery, "query", "q", "", "Search query (required)")
	searchCmd.Flags().IntVarP(&topK, "top", "k", 20, "Number of results to return")
	searchCmd.Flags().StringVarP(&language, "lang", "l", "", "Filter by language")
	searchCmd.MarkFlagRequired("query")

	rootCmd.AddCommand(searchCmd)
}