package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// InitializeDefaultConfig creates the default configuration directory and files in the user's home directory
func InitializeDefaultConfig() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".dockbridge")

	// Create the config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Create client config if it doesn't exist
	clientConfigPath := filepath.Join(configDir, "client.yaml")
	if _, err := os.Stat(clientConfigPath); os.IsNotExist(err) {
		if err := createDefaultClientConfig(clientConfigPath); err != nil {
			return fmt.Errorf("failed to create default client config: %w", err)
		}
	}

	// Create server config if it doesn't exist
	serverConfigPath := filepath.Join(configDir, "server.yaml")
	if _, err := os.Stat(serverConfigPath); os.IsNotExist(err) {
		if err := createDefaultServerConfig(serverConfigPath); err != nil {
			return fmt.Errorf("failed to create default server config: %w", err)
		}
	}

	return nil
}

// createDefaultClientConfig creates the default client configuration file
func createDefaultClientConfig(path string) error {
	content := `# DockBridge Client Configuration
# This file contains default configuration for the DockBridge client
# Configuration files are loaded from ~/.dockbridge/ by default

# Hetzner Cloud configuration
hetzner:
  # API token for Hetzner Cloud (can also be set via HETZNER_API_TOKEN env var)
  api_token: ""
  
  # Server type to provision (see Hetzner Cloud documentation for available types)
  server_type: "cpx21"
  
  # Location/datacenter for the server
  location: "fsn1"
  
  # Volume size in GB (minimum 10, maximum 10000)
  volume_size: 10

# Docker configuration
docker:
  # Path to Docker socket
  socket_path: "/var/run/docker.sock"
  
  # Port for Docker proxy to listen on
  proxy_port: 2376

# Keep-alive configuration
keepalive:
  # Interval between heartbeat messages
  interval: "30s"
  
  # Timeout before server self-destructs without heartbeat
  timeout: "5m"
  
  # Interval between retry attempts
  retry_interval: "5s"
  
  # Maximum number of retry attempts
  max_retries: 3

# SSH configuration
ssh:
  # Path to SSH private key (will be generated if doesn't exist)
  key_path: "~/.dockbridge/ssh/id_rsa"
  
  # SSH port
  port: 22
  
  # SSH connection timeout
  timeout: "30s"
  
  # SSH keep-alive interval
  keep_alive: "30s"

# Logging configuration
logging:
  # Log level: debug, info, warn, error, fatal
  level: "info"
  
  # Log format: json, text
  format: "json"
  
  # Log output: stdout, stderr, or file path
  output: "stdout"
`

	return os.WriteFile(path, []byte(content), 0644)
}

// createDefaultServerConfig creates the default server configuration file
func createDefaultServerConfig(path string) error {
	content := `# DockBridge Server Configuration
# This file contains default configuration for the DockBridge server
# Configuration files are loaded from ~/.dockbridge/ by default

# Docker configuration
docker:
  # Path to Docker socket
  socket_path: "/var/run/docker.sock"
  
  # Port for Docker proxy to listen on
  proxy_port: 2376

# Keep-alive configuration
keepalive:
  # Interval between heartbeat messages
  interval: "30s"
  
  # Timeout before server self-destructs without heartbeat
  timeout: "5m"
  
  # Interval between retry attempts
  retry_interval: "5s"
  
  # Maximum number of retry attempts
  max_retries: 3

# Logging configuration
logging:
  # Log level: debug, info, warn, error, fatal
  level: "info"
  
  # Log format: json, text
  format: "json"
  
  # Log output: stdout, stderr, or file path
  output: "stdout"
`

	return os.WriteFile(path, []byte(content), 0644)
}

// GetDefaultConfigPath returns the default configuration file path for the given config type
func GetDefaultConfigPath(configType string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	return filepath.Join(homeDir, ".dockbridge", configType+".yaml"), nil
}
