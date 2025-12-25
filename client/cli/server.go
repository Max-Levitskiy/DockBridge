package cli

import (
	"context"
	"fmt"

	clientconfig "github.com/dockbridge/dockbridge/client/config"
	"github.com/dockbridge/dockbridge/client/hetzner"

	"github.com/dockbridge/dockbridge/pkg/errors"
	"github.com/dockbridge/dockbridge/pkg/logger"
	"github.com/spf13/cobra"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Manage Hetzner Cloud servers",
	Long:  `Create, destroy, and check status of Hetzner Cloud servers.`,
}

var serverCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Initialize and validate server configuration",
	Long:  `Initialize and validate server configuration. Servers are automatically provisioned when Docker commands are executed.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath, _ := cmd.Flags().GetString("config")

		// Initialize logger
		log := logger.NewDefault()
		_ = log // Use logger if needed

		return createServer(cmd.Context(), configPath)
	},
}

var serverDestroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destroy DockBridge servers",
	Long:  `Destroy all DockBridge servers while preserving volumes for future use.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath, _ := cmd.Flags().GetString("config")
		force, _ := cmd.Flags().GetBool("force")

		// Initialize logger
		log := logger.NewDefault()
		_ = log // Use logger if needed

		return destroyServer(cmd.Context(), configPath, force)
	},
}

var serverStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check DockBridge server status",
	Long:  `Check the status of all DockBridge servers and volumes.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath, _ := cmd.Flags().GetString("config")

		// Initialize logger
		log := logger.NewDefault()
		_ = log // Use logger if needed

		return checkServerStatus(cmd.Context(), configPath)
	},
}

func init() {
	rootCmd.AddCommand(serverCmd)

	// Add subcommands
	serverCmd.AddCommand(serverCreateCmd)
	serverCmd.AddCommand(serverDestroyCmd)
	serverCmd.AddCommand(serverStatusCmd)

	// Add flags
	serverCreateCmd.Flags().StringP("config", "c", "", "Path to configuration file")
	serverCreateCmd.Flags().String("log-config", "", "Path to logger configuration file")

	serverDestroyCmd.Flags().StringP("config", "c", "", "Path to configuration file")
	serverDestroyCmd.Flags().String("log-config", "", "Path to logger configuration file")
	serverDestroyCmd.Flags().BoolP("force", "f", false, "Force destruction without confirmation")

	serverStatusCmd.Flags().StringP("config", "c", "", "Path to configuration file")
	serverStatusCmd.Flags().String("log-config", "", "Path to logger configuration file")
}

func createServer(ctx context.Context, configPath string) error {
	log := logger.GlobalWithField("operation", "server_config_init")
	log.Info("Initializing server configuration")

	// Load configuration
	manager := clientconfig.NewManager()
	if err := manager.Load(configPath); err != nil {
		errors.LogError(err, "Failed to load configuration")
		return errors.NewConfigError(errors.ErrCodeInvalidConfig, "Failed to load configuration", err)
	}

	cfg := manager.GetConfig()

	fmt.Println("DockBridge Server Configuration:")
	fmt.Println("================================")
	fmt.Printf("Server Type: %s\n", cfg.Hetzner.ServerType)
	fmt.Printf("Location: %s\n", cfg.Hetzner.Location)
	fmt.Printf("Volume Size: %d GB\n", cfg.Hetzner.VolumeSize)
	fmt.Printf("Docker API Port: %d\n", cfg.Docker.ProxyPort)
	fmt.Printf("SSH Key Path: %s\n", cfg.SSH.KeyPath)
	fmt.Println()

	// Validate Hetzner API token
	if cfg.Hetzner.APIToken == "" {
		fmt.Println("⚠️  Warning: Hetzner API token not configured.")
		fmt.Println("   Set HETZNER_API_TOKEN environment variable or add to config file.")
		return nil
	}

	// Test Hetzner API connection
	fmt.Println("Testing Hetzner Cloud API connection...")
	hetznerConfig := &hetzner.Config{
		APIToken:   cfg.Hetzner.APIToken,
		ServerType: cfg.Hetzner.ServerType,
		Location:   cfg.Hetzner.Location,
		VolumeSize: cfg.Hetzner.VolumeSize,
	}

	client, err := hetzner.NewClient(hetznerConfig)
	if err != nil {
		fmt.Printf("❌ Failed to create Hetzner client: %v\n", err)
		return errors.NewConfigError(errors.ErrCodeInvalidConfig, "Failed to create Hetzner client", err)
	}

	// Test API by listing servers
	_, err = client.ListServers(ctx)
	if err != nil {
		fmt.Printf("❌ Failed to connect to Hetzner API: %v\n", err)
		return errors.NewNetworkError("API_ERROR", "Failed to connect to Hetzner API", err, true)
	}

	fmt.Println("✅ Hetzner Cloud API connection successful!")

	log.Info("Server configuration validated successfully")
	fmt.Println()
	fmt.Println("Configuration is valid!")
	fmt.Println("Servers will be automatically created when Docker commands are executed.")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Start the DockBridge client: dockbridge-client start")
	fmt.Println("  2. Run Docker commands normally - servers will auto-provision")
	fmt.Println("  3. Check server status: dockbridge-client server status")

	return nil
}

func destroyServer(ctx context.Context, configPath string, force bool) error {
	log := logger.GlobalWithFields(map[string]any{
		"operation": "server_destroy",
		"force":     force,
	})

	log.Info("Starting server destruction process")

	// Load configuration
	manager := clientconfig.NewManager()
	if err := manager.Load(configPath); err != nil {
		errors.LogError(err, "Failed to load configuration")
		return errors.NewConfigError(errors.ErrCodeInvalidConfig, "Failed to load configuration", err)
	}

	cfg := manager.GetConfig()

	// Validate Hetzner API token
	if cfg.Hetzner.APIToken == "" {
		return errors.NewConfigError(errors.ErrCodeInvalidConfig, "Hetzner API token is required", nil)
	}

	// Create Hetzner client
	hetznerConfig := &hetzner.Config{
		APIToken:   cfg.Hetzner.APIToken,
		ServerType: cfg.Hetzner.ServerType,
		Location:   cfg.Hetzner.Location,
		VolumeSize: cfg.Hetzner.VolumeSize,
	}

	client, err := hetzner.NewClient(hetznerConfig)
	if err != nil {
		errors.LogError(err, "Failed to create Hetzner client")
		return errors.NewConfigError(errors.ErrCodeInvalidConfig, "Failed to create Hetzner client", err)
	}

	// List servers to find DockBridge servers
	servers, err := client.ListServers(ctx)
	if err != nil {
		errors.LogError(err, "Failed to list servers")
		return errors.NewNetworkError("API_ERROR", "Failed to list servers", err, true)
	}

	// Filter for DockBridge servers
	var dockbridgeServers []*hetzner.Server
	for _, server := range servers {
		if len(server.Name) >= 10 && server.Name[:10] == "dockbridge" {
			dockbridgeServers = append(dockbridgeServers, server)
		}
	}

	if len(dockbridgeServers) == 0 {
		fmt.Println("No DockBridge servers found to destroy.")
		return nil
	}

	// Show servers that will be destroyed
	fmt.Println("DockBridge servers found:")
	for _, server := range dockbridgeServers {
		fmt.Printf("  - %s (ID: %d, IP: %s)\n", server.Name, server.ID, server.IPAddress)
	}

	if !force {
		fmt.Print("\nAre you sure you want to destroy these servers? Volumes will be preserved. (y/N): ")
		var confirm string
		fmt.Scanln(&confirm)
		if confirm != "y" && confirm != "Y" {
			log.Info("Server destruction cancelled by user")
			fmt.Println("Server destruction cancelled.")
			return nil
		}
	}

	// Create lifecycle manager
	lifecycleManager := hetzner.NewLifecycleManager(client)

	log.Info("Destroying Hetzner Cloud servers")
	fmt.Println("Destroying DockBridge servers...")

	// Destroy each server
	for _, server := range dockbridgeServers {
		fmt.Printf("Destroying server %s...\n", server.Name)

		err := lifecycleManager.DestroyServerWithCleanup(ctx, fmt.Sprintf("%d", server.ID), true)
		if err != nil {
			errors.LogError(err, fmt.Sprintf("Failed to destroy server %s", server.Name))
			fmt.Printf("Failed to destroy server %s: %v\n", server.Name, err)
			continue
		}

		log.WithField("server_id", server.ID).Info("Server destroyed successfully")
		fmt.Printf("Server %s destroyed successfully.\n", server.Name)
	}

	log.Info("Server destruction process completed, volumes preserved")
	fmt.Println("All servers destroyed successfully!")
	fmt.Println("Volumes preserved for future use.")

	return nil
}

func checkServerStatus(ctx context.Context, configPath string) error {
	log := logger.GlobalWithField("operation", "server_status")
	log.Info("Checking server status")

	// Load configuration
	manager := clientconfig.NewManager()
	if err := manager.Load(configPath); err != nil {
		errors.LogError(err, "Failed to load configuration")
		return errors.NewConfigError(errors.ErrCodeInvalidConfig, "Failed to load configuration", err)
	}

	cfg := manager.GetConfig()

	// Validate Hetzner API token
	if cfg.Hetzner.APIToken == "" {
		return errors.NewConfigError(errors.ErrCodeInvalidConfig, "Hetzner API token is required", nil)
	}

	fmt.Println("Checking server status...")

	// Create Hetzner client
	hetznerConfig := &hetzner.Config{
		APIToken:   cfg.Hetzner.APIToken,
		ServerType: cfg.Hetzner.ServerType,
		Location:   cfg.Hetzner.Location,
		VolumeSize: cfg.Hetzner.VolumeSize,
	}

	client, err := hetzner.NewClient(hetznerConfig)
	if err != nil {
		errors.LogError(err, "Failed to create Hetzner client")
		return errors.NewConfigError(errors.ErrCodeInvalidConfig, "Failed to create Hetzner client", err)
	}

	// List all servers to find DockBridge servers
	servers, err := client.ListServers(ctx)
	if err != nil {
		errors.LogError(err, "Failed to list servers")
		return errors.NewNetworkError("API_ERROR", "Failed to list servers", err, true)
	}

	// Filter for DockBridge servers
	var dockbridgeServers []*hetzner.Server
	for _, server := range servers {
		if len(server.Name) >= 10 && server.Name[:10] == "dockbridge" { // Check if server name starts with "dockbridge"
			dockbridgeServers = append(dockbridgeServers, server)
		}
	}

	if len(dockbridgeServers) == 0 {
		fmt.Println("No DockBridge servers found.")
		fmt.Println("Servers are automatically created when Docker commands are executed.")
		return nil
	}

	// Display status for each DockBridge server
	for i, server := range dockbridgeServers {
		if i > 0 {
			fmt.Println() // Add spacing between servers
		}

		fmt.Printf("Server: %s\n", server.Name)
		fmt.Printf("  ID: %d\n", server.ID)
		fmt.Printf("  Status: %s\n", server.Status)
		fmt.Printf("  IP Address: %s\n", server.IPAddress)
		fmt.Printf("  Created: %s\n", server.CreatedAt.Format("2006-01-02 15:04:05"))

		if server.VolumeID != "" {
			// Get volume information
			volume, err := client.GetVolume(ctx, server.VolumeID)
			if err != nil {
				fmt.Printf("  Volume: %s (failed to get details)\n", server.VolumeID)
			} else {
				fmt.Printf("  Volume: %s (%d GB, %s)\n", volume.Name, volume.Size, volume.Status)
			}
		}

		log.WithFields(map[string]any{
			"server_id":     server.ID,
			"server_status": server.Status,
			"server_ip":     server.IPAddress,
		}).Info("Server status retrieved")
	}

	// List volumes
	volumes, err := client.ListVolumes(ctx)
	if err != nil {
		log.WithFields(map[string]any{"error": err.Error()}).Warn("Failed to list volumes")
	} else {
		var dockbridgeVolumes []*hetzner.Volume
		for _, volume := range volumes {
			if len(volume.Name) >= 10 && volume.Name[:10] == "dockbridge" {
				dockbridgeVolumes = append(dockbridgeVolumes, volume)
			}
		}

		if len(dockbridgeVolumes) > 0 {
			fmt.Println("\nDockBridge Volumes:")
			for _, volume := range dockbridgeVolumes {
				fmt.Printf("  %s: %d GB (%s)\n", volume.Name, volume.Size, volume.Status)
			}
		}
	}

	return nil
}
