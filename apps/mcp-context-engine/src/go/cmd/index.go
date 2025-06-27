package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var indexCmd = &cobra.Command{
	Use:   "index [path]",
	Short: "Index a repository for search",
	Long:  `Build or update search indexes for the specified repository path.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repoPath := args[0]
		fmt.Printf("Indexing repository: %s\n", repoPath)
		// TODO: Implement indexing logic
		return nil
	},
}

func init() {
	rootCmd.AddCommand(indexCmd)
}