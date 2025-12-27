package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dockbridge/dockbridge/client/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize DockBridge client configuration",
	Long: `Initialize DockBridge client configuration with guided setup.
This command creates the necessary configuration files and directories.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		force, _ := cmd.Flags().GetBool("force")
		guided, _ := cmd.Flags().GetBool("guided")

		if guided {
			return runGuidedSetup(force)
		}

		return initializeConfig(force)
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().BoolP("force", "f", false, "Force reinitialization of configuration")
	initCmd.Flags().BoolP("guided", "g", false, "Run guided setup for configuration")
}

func initializeConfig(force bool) error {
	fmt.Println("Initializing DockBridge client configuration...")

	// Get default config path
	clientConfigPath, err := config.GetDefaultConfigPath("client")
	if err != nil {
		return fmt.Errorf("failed to get default config path: %w", err)
	}

	// Check if config already exists
	if _, err := os.Stat(clientConfigPath); err == nil && !force {
		fmt.Println("Configuration already exists. Use --force to reinitialize.")
		return nil
	}

	// Create config directory
	configDir := filepath.Dir(clientConfigPath)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Create SSH directory
	sshDir := filepath.Join(configDir, "ssh")
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		return fmt.Errorf("failed to create SSH directory: %w", err)
	}

	// Initialize default configuration
	if err := config.InitializeDefaultConfig(); err != nil {
		return fmt.Errorf("failed to initialize default configuration: %w", err)
	}

	fmt.Println("Configuration initialized successfully at:", clientConfigPath)
	fmt.Println("You may need to set your Hetzner API token in the configuration file or via the HETZNER_API_TOKEN environment variable.")
	return nil
}

func runGuidedSetup(force bool) error {
	fmt.Println("Running guided setup for DockBridge client...")

	// Initialize default config first
	if err := initializeConfig(force); err != nil {
		return err
	}

	// Get default config path
	clientConfigPath, err := config.GetDefaultConfigPath("client")
	if err != nil {
		return fmt.Errorf("failed to get default config path: %w", err)
	}

	// Create a new configuration manager
	manager := config.NewManager()
	if err := manager.Load(clientConfigPath); err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	cfg := manager.GetConfig()

	// Get user input
	reader := bufio.NewReader(os.Stdin)

	// Hetzner API Token
	fmt.Print("Enter Hetzner API Token (leave empty to set via environment variable): ")
	apiToken, _ := reader.ReadString('\n')
	apiToken = strings.TrimSpace(apiToken)
	if apiToken != "" {
		cfg.Hetzner.APIToken = apiToken
	}

	// Server Type
	fmt.Printf("Enter Hetzner Server Type (default: %s): ", cfg.Hetzner.ServerType)
	serverType, _ := reader.ReadString('\n')
	serverType = strings.TrimSpace(serverType)
	if serverType != "" {
		cfg.Hetzner.ServerType = serverType
	}

	// Location
	fmt.Printf("Enter Hetzner Location (default: %s): ", cfg.Hetzner.Location)
	location, _ := reader.ReadString('\n')
	location = strings.TrimSpace(location)
	if location != "" {
		cfg.Hetzner.Location = location
	}

	// Volume Size
	fmt.Printf("Enter Volume Size in GB (default: %d): ", cfg.Hetzner.VolumeSize)
	volumeSizeStr, _ := reader.ReadString('\n')
	volumeSizeStr = strings.TrimSpace(volumeSizeStr)
	if volumeSizeStr != "" {
		var volumeSize int
		if _, err := fmt.Sscanf(volumeSizeStr, "%d", &volumeSize); err == nil && volumeSize > 0 {
			cfg.Hetzner.VolumeSize = volumeSize
		}
	}

	// Docker Socket Path
	fmt.Printf("Enter Docker Socket Path (default: %s): ", cfg.Docker.SocketPath)
	socketPath, _ := reader.ReadString('\n')
	socketPath = strings.TrimSpace(socketPath)
	if socketPath != "" {
		cfg.Docker.SocketPath = socketPath
	}

	// Save the updated configuration
	if err := saveClientConfig(clientConfigPath, cfg); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	fmt.Println("Configuration saved successfully!")
	return nil
}

func saveClientConfig(path string, cfg any) error {
	// Convert config to YAML
	yamlData, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration to YAML: %w", err)
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Write to file
	if err := os.WriteFile(path, yamlData, 0600); err != nil {
		return fmt.Errorf("failed to write configuration to %s: %w", path, err)
	}

	fmt.Println("Configuration saved successfully to:", path)
	return nil
}
