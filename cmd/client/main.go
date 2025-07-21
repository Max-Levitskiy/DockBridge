package main

import (
	"fmt"
	"os"

	"github.com/dockbridge/dockbridge/internal/client/cli"
)

func main() {
	// Execute the root command
	if err := cli.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
