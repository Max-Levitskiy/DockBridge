package config

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the proxy configuration
type Config struct {
	LocalSocket  string        `yaml:"local_socket"`  // Local Unix socket path (e.g., /tmp/docker.sock)
	SSHUser      string        `yaml:"ssh_user"`      // SSH username
	SSHHost      string        `yaml:"ssh_host"`      // SSH hostname with optional port
	SSHKeyPath   string        `yaml:"ssh_key_path"`  // Path to SSH private key file
	RemoteSocket string        `yaml:"remote_socket"` // Remote Docker socket path (default: /var/run/docker.sock)
	Timeout      time.Duration `yaml:"timeout"`       // SSH connection timeout
}

// Validate ensures configuration is complete and valid
func (c *Config) Validate() error {
	if c.SSHUser == "" {
		return &ProxyError{
			Category: ErrorCategoryConfig,
			Message:  "SSH user is required",
		}
	}

	if c.SSHHost == "" {
		return &ProxyError{
			Category: ErrorCategoryConfig,
			Message:  "SSH host is required",
		}
	}

	if c.SSHKeyPath == "" {
		return &ProxyError{
			Category: ErrorCategoryConfig,
			Message:  "SSH key path is required",
		}
	}

	// Expand and check if SSH key file exists
	expandedKeyPath, err := expandPath(c.SSHKeyPath)
	if err != nil {
		return &ProxyError{
			Category: ErrorCategoryConfig,
			Message:  fmt.Sprintf("failed to expand SSH key path: %s", c.SSHKeyPath),
			Cause:    err,
		}
	}

	if _, err := os.Stat(expandedKeyPath); os.IsNotExist(err) {
		return &ProxyError{
			Category: ErrorCategoryConfig,
			Message:  fmt.Sprintf("SSH key file does not exist: %s", expandedKeyPath),
			Cause:    err,
		}
	}

	if c.LocalSocket == "" {
		return &ProxyError{
			Category: ErrorCategoryConfig,
			Message:  "Local socket path is required",
		}
	}

	if c.RemoteSocket == "" {
		c.RemoteSocket = "/var/run/docker.sock"
	}

	if c.Timeout <= 0 {
		c.Timeout = 10 * time.Second
	}

	return nil
}

// ProxyError represents categorized proxy errors
type ProxyError struct {
	Category string
	Message  string
	Cause    error
}

func (e *ProxyError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Category, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Category, e.Message)
}

// Error categories
const (
	ErrorCategoryConfig  = "CONFIG"
	ErrorCategorySSH     = "SSH"
	ErrorCategoryDocker  = "DOCKER"
	ErrorCategoryRuntime = "RUNTIME"
)

// DefaultConfig returns a configuration with default values
func DefaultConfig() *Config {
	return &Config{
		RemoteSocket: "/var/run/docker.sock",
		Timeout:      10 * time.Second,
	}
}

// LoadFromFile loads configuration from a YAML file
func LoadFromFile(filename string) (*Config, error) {
	config := DefaultConfig()

	// Check if file exists
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return nil, &ProxyError{
			Category: ErrorCategoryConfig,
			Message:  fmt.Sprintf("Configuration file does not exist: %s", filename),
			Cause:    err,
		}
	}

	// Read file
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, &ProxyError{
			Category: ErrorCategoryConfig,
			Message:  fmt.Sprintf("Failed to read configuration file: %s", filename),
			Cause:    err,
		}
	}

	// Parse YAML
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, &ProxyError{
			Category: ErrorCategoryConfig,
			Message:  fmt.Sprintf("Failed to parse YAML configuration: %s", filename),
			Cause:    err,
		}
	}

	return config, nil
}

// LoadFromFlags loads configuration from command-line flags
func LoadFromFlags(args []string) (*Config, error) {
	config := DefaultConfig()

	// Create a new flag set to avoid conflicts with global flags
	fs := flag.NewFlagSet("ssh-docker-proxy", flag.ContinueOnError)

	// Define flags (including config flag which we'll ignore here)
	_ = fs.String("config", "", "Path to configuration file (optional)")
	localSocket := fs.String("local-socket", "", "Local Unix socket path (required)")
	sshUser := fs.String("ssh-user", "", "SSH username (required)")
	sshHost := fs.String("ssh-host", "", "SSH hostname with optional port (required)")
	sshKeyPath := fs.String("ssh-key", "", "Path to SSH private key file (required)")
	remoteSocket := fs.String("remote-socket", config.RemoteSocket, "Remote Docker socket path")
	timeout := fs.Duration("timeout", config.Timeout, "SSH connection timeout")

	// Parse flags
	if err := fs.Parse(args); err != nil {
		return nil, &ProxyError{
			Category: ErrorCategoryConfig,
			Message:  "Failed to parse command-line flags",
			Cause:    err,
		}
	}

	// Set values from flags
	if *localSocket != "" {
		config.LocalSocket = *localSocket
	}
	if *sshUser != "" {
		config.SSHUser = *sshUser
	}
	if *sshHost != "" {
		config.SSHHost = *sshHost
	}
	if *sshKeyPath != "" {
		config.SSHKeyPath = *sshKeyPath
	}
	if *remoteSocket != config.RemoteSocket {
		config.RemoteSocket = *remoteSocket
	}
	if *timeout != config.Timeout {
		config.Timeout = *timeout
	}

	return config, nil
}

// LoadConfig loads configuration with precedence: flags > file > defaults
func LoadConfig(configFile string, args []string) (*Config, error) {
	var config *Config
	var err error

	// Start with defaults
	config = DefaultConfig()

	// Load from file if specified and exists
	if configFile != "" {
		if _, statErr := os.Stat(configFile); statErr == nil {
			fileConfig, loadErr := LoadFromFile(configFile)
			if loadErr != nil {
				return nil, loadErr
			}
			config = fileConfig
		}
	}

	// Override with flags
	flagConfig, err := LoadFromFlags(args)
	if err != nil {
		return nil, err
	}

	// Apply flag overrides (only if flag was explicitly set)
	if err := applyFlagOverrides(config, flagConfig, args); err != nil {
		return nil, err
	}

	return config, nil
}

// applyFlagOverrides applies flag values to config only if they were explicitly set
func applyFlagOverrides(config, flagConfig *Config, args []string) error {
	// Create a flag set to check which flags were actually set
	fs := flag.NewFlagSet("ssh-docker-proxy", flag.ContinueOnError)

	_ = fs.String("config", "", "")
	localSocket := fs.String("local-socket", "", "")
	sshUser := fs.String("ssh-user", "", "")
	sshHost := fs.String("ssh-host", "", "")
	sshKeyPath := fs.String("ssh-key", "", "")
	remoteSocket := fs.String("remote-socket", config.RemoteSocket, "")
	timeout := fs.Duration("timeout", config.Timeout, "")

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Check which flags were visited (explicitly set)
	flagsSet := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) {
		flagsSet[f.Name] = true
	})

	// Apply overrides only for explicitly set flags
	if flagsSet["local-socket"] {
		config.LocalSocket = *localSocket
	}
	if flagsSet["ssh-user"] {
		config.SSHUser = *sshUser
	}
	if flagsSet["ssh-host"] {
		config.SSHHost = *sshHost
	}
	if flagsSet["ssh-key"] {
		config.SSHKeyPath = *sshKeyPath
	}
	if flagsSet["remote-socket"] {
		config.RemoteSocket = *remoteSocket
	}
	if flagsSet["timeout"] {
		config.Timeout = *timeout
	}

	return nil
}

// FindConfigFile searches for configuration file in standard locations
func FindConfigFile() string {
	// Check common config file locations
	locations := []string{
		"ssh-docker-proxy.yaml",
		"ssh-docker-proxy.yml",
		"config.yaml",
		"config.yml",
	}

	// Add home directory locations
	if homeDir, err := os.UserHomeDir(); err == nil {
		homeLocations := []string{
			filepath.Join(homeDir, ".ssh-docker-proxy.yaml"),
			filepath.Join(homeDir, ".ssh-docker-proxy.yml"),
			filepath.Join(homeDir, ".config", "ssh-docker-proxy", "config.yaml"),
			filepath.Join(homeDir, ".config", "ssh-docker-proxy", "config.yml"),
		}
		locations = append(locations, homeLocations...)
	}

	// Return first existing file
	for _, location := range locations {
		if _, err := os.Stat(location); err == nil {
			return location
		}
	}

	return ""
}

// expandPath expands ~ to the user's home directory
func expandPath(path string) (string, error) {
	if !strings.HasPrefix(path, "~") {
		return path, nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	if path == "~" {
		return homeDir, nil
	}

	if strings.HasPrefix(path, "~/") {
		return filepath.Join(homeDir, path[2:]), nil
	}

	// Handle ~user/path format (not commonly used, but for completeness)
	return path, nil
}
