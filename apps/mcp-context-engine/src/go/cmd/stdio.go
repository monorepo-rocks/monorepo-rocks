package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var stdioCmd = &cobra.Command{
	Use:   "stdio",
	Short: "Run in MCP stdio mode",
	Long:  `Start the engine in MCP (Model Context Protocol) stdio mode for LLM agent integration.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Starting MCP stdio server...")
		// TODO: Implement MCP stdio protocol handler
		return nil
	},
}

func init() {
	rootCmd.AddCommand(stdioCmd)
}