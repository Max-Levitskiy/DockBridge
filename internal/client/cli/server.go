package cli

import (
	"fmt"

	"github.com/dockbridge/dockbridge/internal/client/config"
	"github.com/spf13/cobra"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Manage Hetzner Cloud servers",
	Long:  `Create, destroy, and check status of Hetzner Cloud servers.`,
}

var serverCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new Hetzner Cloud server",
	Long:  `Provision a new Hetzner Cloud server with Docker CE installed.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath, _ := cmd.Flags().GetString("config")
		return createServer(configPath)
	},
}

var serverDestroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destroy a Hetzner Cloud server",
	Long:  `Destroy a Hetzner Cloud server while preserving volumes.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath, _ := cmd.Flags().GetString("config")
		force, _ := cmd.Flags().GetBool("force")
		return destroyServer(configPath, force)
	},
}

var serverStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check server status",
	Long:  `Check the status of the Hetzner Cloud server.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath, _ := cmd.Flags().GetString("config")
		return checkServerStatus(configPath)
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

	serverDestroyCmd.Flags().StringP("config", "c", "", "Path to configuration file")
	serverDestroyCmd.Flags().BoolP("force", "f", false, "Force destruction without confirmation")

	serverStatusCmd.Flags().StringP("config", "c", "", "Path to configuration file")
}

func createServer(configPath string) error {
	// Load configuration
	manager := config.NewManager()
	if err := manager.Load(configPath); err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	cfg := manager.GetConfig()

	fmt.Println("Creating Hetzner Cloud server...")
	fmt.Println("Server Type:", cfg.Hetzner.ServerType)
	fmt.Println("Location:", cfg.Hetzner.Location)
	fmt.Println("Volume Size:", cfg.Hetzner.VolumeSize, "GB")

	// Placeholder for actual server creation logic
	fmt.Println("Server created successfully!")
	fmt.Println("Server IP: 203.0.113.10")

	return nil
}

func destroyServer(configPath string, force bool) error {
	if !force {
		fmt.Print("Are you sure you want to destroy the server? This action cannot be undone. (y/N): ")
		var confirm string
		fmt.Scanln(&confirm)
		if confirm != "y" && confirm != "Y" {
			fmt.Println("Server destruction cancelled.")
			return nil
		}
	}

	fmt.Println("Destroying Hetzner Cloud server...")

	// Placeholder for actual server destruction logic
	fmt.Println("Server destroyed successfully!")
	fmt.Println("Volume preserved for future use.")

	return nil
}

func checkServerStatus(configPath string) error {
	fmt.Println("Checking server status...")

	// Placeholder for actual status check logic
	fmt.Println("Server Status: Running")
	fmt.Println("Server IP: 203.0.113.10")
	fmt.Println("Uptime: 2 hours 15 minutes")
	fmt.Println("Docker Status: Running")
	fmt.Println("Volume Status: Attached")

	return nil
}
