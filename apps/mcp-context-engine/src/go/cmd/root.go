package cmd

import (
	"github.com/spf13/cobra"
)

var (
	configFile string
	rootCmd    = &cobra.Command{
		Use:   "mcpce",
		Short: "MCP Context Engine - Fast, offline code search",
		Long: `MCP Context Engine combines lexical (Zoekt) and semantic (FAISS) search
to provide sub-100ms code retrieval for LLM agents. All indexing and search
runs entirely offline with zero network dependencies.`,
	}
)

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "config file (default is $HOME/.config/mcpce/config.yaml)")
}