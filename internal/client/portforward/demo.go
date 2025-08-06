package portforward

import (
	"context"
	"fmt"
	"time"

	"github.com/dockbridge/dockbridge/internal/client/monitor"
	"github.com/dockbridge/dockbridge/internal/shared/config"
	"github.com/dockbridge/dockbridge/pkg/logger"
)

// DemoPortForwarding demonstrates the port forwarding infrastructure
func DemoPortForwarding() error {
	fmt.Println("=== Port Forwarding Infrastructure Demo ===")

	// Create logger
	logger, err := logger.New(&logger.Config{
		Level:     "info",
		UseColors: true,
	})
	if err != nil {
		return fmt.Errorf("failed to create logger: %w", err)
	}

	// Demo 1: Port Conflict Resolver
	fmt.Println("\n1. Port Conflict Resolution Demo")
	resolver := NewPortConflictResolver()

	// Test common ports with increment strategy
	testPorts := []int{80, 443, 3000, 5432}
	for _, port := range testPorts {
		if resolver.IsPortAvailable(port) {
			fmt.Printf("   Port %d is available\n", port)
		} else {
			fmt.Printf("   Port %d is occupied, resolving...\n", port)
			resolvedPort, err := resolver.ResolvePortConflict(port, config.ConflictStrategyIncrement)
			if err != nil {
				fmt.Printf("   Failed to resolve port %d: %v\n", port, err)
			} else {
				fmt.Printf("   Resolved port %d -> %d\n", port, resolvedPort)
			}
		}
	}

	// Demo fail strategy
	fmt.Println("\n   Testing fail strategy:")
	if !resolver.IsPortAvailable(80) {
		_, err := resolver.ResolvePortConflict(80, config.ConflictStrategyFail)
		if err != nil {
			fmt.Printf("   Port 80 conflict with fail strategy: %v\n", err)
		}
	}

	// Demo 2: Port Forward Manager
	fmt.Println("\n2. Port Forward Manager Demo")
	config := &config.PortForwardConfig{
		Enabled:          true,
		ConflictStrategy: config.ConflictStrategyIncrement,
		MonitorInterval:  30 * time.Second,
	}

	manager := NewPortForwardManager(config, logger)

	// Start manager
	ctx := context.Background()
	if err := manager.Start(ctx); err != nil {
		return fmt.Errorf("failed to start port forward manager: %w", err)
	}
	defer manager.Stop()

	fmt.Println("   Port forward manager started successfully")

	// Simulate container creation
	container := &monitor.ContainerInfo{
		ID:     "demo-container-123",
		Name:   "demo-nginx",
		Image:  "nginx:latest",
		Status: "running",
		Ports: []monitor.PortMapping{
			{
				ContainerPort: 80,
				HostPort:      8080,
				Protocol:      "tcp",
				HostIP:        "0.0.0.0",
			},
			{
				ContainerPort: 443,
				HostPort:      8443,
				Protocol:      "tcp",
				HostIP:        "0.0.0.0",
			},
		},
		Labels:  map[string]string{"app": "demo"},
		Created: time.Now(),
	}

	if err := manager.OnContainerCreated(container); err != nil {
		return fmt.Errorf("failed to handle container creation: %w", err)
	}

	// List port forwards
	forwards, err := manager.ListPortForwards()
	if err != nil {
		return fmt.Errorf("failed to list port forwards: %w", err)
	}

	fmt.Printf("   Created %d port forwards:\n", len(forwards))
	for _, forward := range forwards {
		fmt.Printf("     %s: localhost:%d -> container:%d (status: %s)\n",
			forward.ContainerName, forward.LocalPort, forward.RemotePort, forward.Status)
	}

	// Simulate container stop
	if err := manager.OnContainerStopped(container.ID); err != nil {
		return fmt.Errorf("failed to handle container stop: %w", err)
	}

	// Verify cleanup
	forwards, err = manager.ListPortForwards()
	if err != nil {
		return fmt.Errorf("failed to list port forwards after cleanup: %w", err)
	}

	fmt.Printf("   After container stop: %d port forwards remaining\n", len(forwards))

	fmt.Println("\n=== Demo completed successfully ===")
	fmt.Println("\nNote: This demo shows the port forwarding infrastructure.")
	fmt.Println("Actual TCP proxy functionality requires SSH tunnel integration.")
	fmt.Println("Run 'go test -v ./internal/client/portforward' to see all tests.")

	return nil
}
