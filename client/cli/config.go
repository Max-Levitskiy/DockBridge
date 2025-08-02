package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dockbridge/dockbridge/client/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage DockBridge configuration",
	Long:  `View and modify DockBridge configuration settings.`,
}

var configViewCmd = &cobra.Command{
	Use:   "view",
	Short: "View current configuration",
	Long:  `Display the current DockBridge configuration.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath, _ := cmd.Flags().GetString("config")
		return viewConfig(configPath)
	},
}

var configValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate configuration",
	Long:  `Validate the DockBridge configuration for errors.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath, _ := cmd.Flags().GetString("config")
		return validateConfig(configPath)
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set [key] [value]",
	Short: "Set configuration value",
	Long:  `Set a specific configuration value.`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath, _ := cmd.Flags().GetString("config")
		return setConfigValue(configPath, args[0], args[1])
	},
}

func init() {
	rootCmd.AddCommand(configCmd)

	// Add subcommands
	configCmd.AddCommand(configViewCmd)
	configCmd.AddCommand(configValidateCmd)
	configCmd.AddCommand(configSetCmd)

	// Add flags
	configViewCmd.Flags().StringP("config", "c", "", "Path to configuration file")
	configValidateCmd.Flags().StringP("config", "c", "", "Path to configuration file")
	configSetCmd.Flags().StringP("config", "c", "", "Path to configuration file")
}

func viewConfig(configPath string) error {
	// Load configuration
	manager := config.NewManager()
	if err := manager.Load(configPath); err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	cfg := manager.GetConfig()

	// Display configuration (simplified for now)
	fmt.Println("Current DockBridge Configuration:")
	fmt.Println("=================================")

	fmt.Println("\nHetzner Configuration:")
	fmt.Println("  API Token:", maskToken(cfg.Hetzner.APIToken))
	fmt.Println("  Server Type:", cfg.Hetzner.ServerType)
	fmt.Println("  Location:", cfg.Hetzner.Location)
	fmt.Println("  Volume Size:", cfg.Hetzner.VolumeSize, "GB")

	fmt.Println("\nDocker Configuration:")
	fmt.Println("  Socket Path:", cfg.Docker.SocketPath)
	fmt.Println("  Proxy Port:", cfg.Docker.ProxyPort)

	fmt.Println("\nKeep-Alive Configuration:")
	fmt.Println("  Interval:", cfg.KeepAlive.Interval)
	fmt.Println("  Timeout:", cfg.KeepAlive.Timeout)
	fmt.Println("  Retry Interval:", cfg.KeepAlive.RetryInterval)
	fmt.Println("  Max Retries:", cfg.KeepAlive.MaxRetries)

	fmt.Println("\nSSH Configuration:")
	fmt.Println("  Key Path:", cfg.SSH.KeyPath)
	fmt.Println("  Port:", cfg.SSH.Port)
	fmt.Println("  Timeout:", cfg.SSH.Timeout)
	fmt.Println("  Keep-Alive:", cfg.SSH.KeepAlive)

	fmt.Println("\nLogging Configuration:")
	fmt.Println("  Level:", cfg.Logging.Level)
	fmt.Println("  Format:", cfg.Logging.Format)
	fmt.Println("  Output:", cfg.Logging.Output)

	return nil
}

// maskToken masks a token for display
func maskToken(token string) string {
	if token == "" {
		return "[not set]"
	}
	if len(token) <= 8 {
		return "****"
	}
	return token[:4] + "..." + token[len(token)-4:]
}

func validateConfig(configPath string) error {
	// Load configuration
	manager := config.NewManager()
	if err := manager.Load(configPath); err != nil {
		return fmt.Errorf("failed to validate configuration: %w", err)
	}

	fmt.Println("Configuration is valid!")
	return nil
}

// parseDuration parses a duration string
func parseDuration(value string) time.Duration {
	duration, err := time.ParseDuration(value)
	if err != nil {
		fmt.Printf("Warning: Invalid duration format '%s', using default\n", value)
		return 0
	}
	return duration
}

func setConfigValue(configPath, key, value string) error {
	// Get default config path if not provided
	if configPath == "" {
		var err error
		configPath, err = config.GetDefaultConfigPath("client")
		if err != nil {
			return fmt.Errorf("failed to get default config path: %w", err)
		}
	}

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Errorf("configuration file does not exist: %s", configPath)
	}

	displayValue := value
	if key == "hetzner.api_token" {
		displayValue = "****"
	}
	fmt.Printf("Setting configuration %s to %s in %s\n", key, displayValue, configPath)

	// Load the configuration without validation
	manager := config.NewManager()
	if err := manager.LoadWithoutValidation(configPath); err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Get the configuration
	cfg := manager.GetConfig()

	// Update the configuration based on the key
	switch key {
	case "hetzner.api_token":
		cfg.Hetzner.APIToken = value
	case "hetzner.server_type":
		cfg.Hetzner.ServerType = value
	case "hetzner.location":
		cfg.Hetzner.Location = value
	case "hetzner.volume_size":
		var volumeSize int
		if _, err := fmt.Sscanf(value, "%d", &volumeSize); err != nil {
			return fmt.Errorf("invalid volume size: %s", value)
		}
		cfg.Hetzner.VolumeSize = volumeSize
	case "docker.socket_path":
		cfg.Docker.SocketPath = value
	case "docker.proxy_port":
		var proxyPort int
		if _, err := fmt.Sscanf(value, "%d", &proxyPort); err != nil {
			return fmt.Errorf("invalid proxy port: %s", value)
		}
		cfg.Docker.ProxyPort = proxyPort
	case "keepalive.interval":
		cfg.KeepAlive.Interval = parseDuration(value)
	case "keepalive.timeout":
		cfg.KeepAlive.Timeout = parseDuration(value)
	case "keepalive.retry_interval":
		cfg.KeepAlive.RetryInterval = parseDuration(value)
	case "keepalive.max_retries":
		var maxRetries int
		if _, err := fmt.Sscanf(value, "%d", &maxRetries); err != nil {
			return fmt.Errorf("invalid max retries: %s", value)
		}
		cfg.KeepAlive.MaxRetries = maxRetries
	case "ssh.key_path":
		cfg.SSH.KeyPath = value
	case "ssh.port":
		var port int
		if _, err := fmt.Sscanf(value, "%d", &port); err != nil {
			return fmt.Errorf("invalid port: %s", value)
		}
		cfg.SSH.Port = port
	case "ssh.timeout":
		cfg.SSH.Timeout = parseDuration(value)
	case "ssh.keep_alive":
		cfg.SSH.KeepAlive = parseDuration(value)
	case "logging.level":
		cfg.Logging.Level = value
	case "logging.format":
		cfg.Logging.Format = value
	case "logging.output":
		cfg.Logging.Output = value
	default:
		return fmt.Errorf("unknown configuration key: %s", key)
	}

	// Save the updated configuration
	if err := manager.SaveConfig(configPath); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	fmt.Println("Configuration updated successfully!")
	return nil
}

// saveConfig saves the configuration to a file
func saveConfig(path string, cfg interface{}) error {
	// Convert config to YAML
	yamlData, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration to YAML: %w", err)
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Write to file
	if err := os.WriteFile(path, yamlData, 0644); err != nil {
		return fmt.Errorf("failed to write configuration to %s: %w", path, err)
	}

	return nil
}
