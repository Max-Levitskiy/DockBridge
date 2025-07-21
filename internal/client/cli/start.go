package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/dockbridge/dockbridge/internal/client/config"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the DockBridge client",
	Long: `Start the DockBridge client which proxies Docker commands to a remote Hetzner server.
The client will automatically provision a server if none exists.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath, _ := cmd.Flags().GetString("config")
		return startClient(configPath)
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
	startCmd.Flags().StringP("config", "c", "", "Path to configuration file")
}

func startClient(configPath string) error {
	fmt.Println("Starting DockBridge client...")

	// Load configuration
	manager := config.NewManager()
	if err := manager.Load(configPath); err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	cfg := manager.GetConfig()

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		fmt.Printf("Received signal %v, shutting down...\n", sig)
		cancel()
	}()

	// Start Docker proxy (placeholder for actual implementation)
	fmt.Println("Starting Docker proxy on port:", cfg.Docker.ProxyPort)
	fmt.Println("Using Docker socket:", cfg.Docker.SocketPath)
	fmt.Println("Using Hetzner server type:", cfg.Hetzner.ServerType)
	fmt.Println("Using Hetzner location:", cfg.Hetzner.Location)

	// Start lock detector (placeholder for actual implementation)
	fmt.Println("Starting lock detector...")

	// Start keep-alive client (placeholder for actual implementation)
	fmt.Println("Starting keep-alive client with interval:", cfg.KeepAlive.Interval)

	fmt.Println("DockBridge client started successfully!")
	fmt.Println("Press Ctrl+C to stop")

	// Wait for context cancellation
	<-ctx.Done()
	fmt.Println("Shutting down DockBridge client...")

	return nil
}
