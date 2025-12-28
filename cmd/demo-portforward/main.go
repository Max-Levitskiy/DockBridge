package main

import (
	"fmt"
	"os"

	"github.com/Max-Levitskiy/DockBridge/client/portforward"
)

func main() {
	if err := portforward.DemoPortForwarding(); err != nil {
		fmt.Fprintf(os.Stderr, "Demo failed: %v\n", err)
		os.Exit(1)
	}
}
