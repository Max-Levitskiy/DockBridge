package main

import (
	"fmt"
	"os"

	"github.com/Max-Levitskiy/DockBridge/client/cli"
)

func main() {
	// Execute the root command
	if err := cli.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
