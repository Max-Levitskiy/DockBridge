package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"ssh-docker-proxy/internal/cli"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nReceived shutdown signal, stopping proxy...")
		cancel()
	}()

	// Create and run CLI
	cli := cli.NewCLI()
	if err := cli.Execute(ctx, os.Args[1:]); err != nil {
		log.Fatalf("Error: %v", err)
	}
}
