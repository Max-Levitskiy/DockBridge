package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/dockbridge/dockbridge/internal/client/config"
	"github.com/dockbridge/dockbridge/internal/client/docker"
	"github.com/dockbridge/dockbridge/internal/client/hetzner"
	"github.com/dockbridge/dockbridge/pkg/logger"
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

	// Validate required configuration
	if cfg.Hetzner.APIToken == "" {
		return fmt.Errorf("Hetzner API token is required. Please set it in the configuration file or via HETZNER_API_TOKEN environment variable")
	}

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

	// Initialize logger
	log := logger.NewDefault()
	log.Info("Initializing DockBridge client")

	// Create Hetzner client
	hetznerConfig := &hetzner.Config{
		APIToken:   cfg.Hetzner.APIToken,
		ServerType: cfg.Hetzner.ServerType,
		Location:   cfg.Hetzner.Location,
		VolumeSize: cfg.Hetzner.VolumeSize,
	}

	hetznerClient, err := hetzner.NewClient(hetznerConfig)
	if err != nil {
		return fmt.Errorf("failed to create Hetzner client: %w", err)
	}

	// Create Docker proxy configuration
	proxyConfig := &docker.ProxyConfig{
		SocketPath:    cfg.Docker.SocketPath,
		ProxyPort:     cfg.Docker.ProxyPort,
		HetznerClient: hetznerClient,
		SSHConfig:     &cfg.SSH,
		HetznerConfig: &cfg.Hetzner,
		Logger:        log,
	}

	// Create and start Docker proxy
	proxy := docker.NewDockerProxy()

	fmt.Printf("Starting Docker proxy on socket: %s\n", cfg.Docker.SocketPath)
	fmt.Printf("Using Hetzner server type: %s in location: %s\n", cfg.Hetzner.ServerType, cfg.Hetzner.Location)
	fmt.Println("Servers will be provisioned automatically when Docker commands are executed.")

	// If using Unix socket, suggest adding user to docker group
	if strings.HasPrefix(cfg.Docker.SocketPath, "/") {
		fmt.Println("Note: If you encounter permission issues, ensure your user is in the 'docker' group:")
		fmt.Println("  sudo usermod -aG docker $USER")
		fmt.Println("  Then log out and back in for the changes to take effect.")
	}

	if err := proxy.Start(ctx, proxyConfig); err != nil {
		return fmt.Errorf("failed to start Docker proxy: %w", err)
	}

	// Start lock detector (placeholder for actual implementation)
	fmt.Println("Starting lock detector...")

	// Start keep-alive client (placeholder for actual implementation)
	fmt.Printf("Starting keep-alive client with interval: %s\n", cfg.KeepAlive.Interval)

	fmt.Println("DockBridge client started successfully!")
	fmt.Println("Docker commands will be proxied to remote Hetzner servers (provisioned on-demand)")
	fmt.Println("Press Ctrl+C to stop")

	// Wait for context cancellation
	<-ctx.Done()
	fmt.Println("Shutting down DockBridge client...")

	// Stop the proxy
	if err := proxy.Stop(); err != nil {
		log.WithFields(map[string]interface{}{
			"error": err.Error(),
		}).Error("Error stopping Docker proxy")
	}

	fmt.Println("DockBridge client stopped")
	return nil
}
