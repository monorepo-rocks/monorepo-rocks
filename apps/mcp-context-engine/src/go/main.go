package main

import (
	"fmt"
	"os"

	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}