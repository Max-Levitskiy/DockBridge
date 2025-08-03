package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	ssh_docker_proxy "ssh-docker-proxy"
)

// Example logger implementation
type ExampleLogger struct{}

func (l *ExampleLogger) Printf(format string, v ...interface{}) {
	log.Printf("[EXAMPLE] "+format, v...)
}

func main() {
	// Example configuration
	config := &ssh_docker_proxy.ProxyConfig{
		LocalSocket:  "/tmp/docker-example.sock",
		SSHUser:      "ubuntu",
		SSHHost:      "your-server.example.com",
		SSHKeyPath:   "~/.ssh/id_rsa",
		RemoteSocket: "/var/run/docker.sock",
		Timeout:      "10s",
	}

	// Create proxy with custom logger
	proxy, err := ssh_docker_proxy.NewProxy(config, &ExampleLogger{})
	if err != nil {
		log.Fatalf("Failed to create proxy: %v", err)
	}

	// Set up context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nReceived shutdown signal, stopping proxy...")
		cancel()
	}()

	// Start the proxy
	fmt.Printf("Starting SSH Docker proxy on %s\n", config.LocalSocket)
	fmt.Printf("Connecting to %s@%s:%s\n", config.SSHUser, config.SSHHost, config.RemoteSocket)
	fmt.Println("Press Ctrl+C to stop")

	if err := proxy.Start(ctx); err != nil {
		log.Fatalf("Proxy failed: %v", err)
	}

	fmt.Println("Proxy stopped")
}
