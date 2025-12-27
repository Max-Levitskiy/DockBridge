package config

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/dockbridge/dockbridge/shared/config"
	"github.com/spf13/viper"
)

// Manager handles configuration loading and validation for the server
type Manager struct {
	viper  *viper.Viper
	config *config.ServerConfig
}

// NewManager creates a new configuration manager
func NewManager() *Manager {
	v := viper.New()
	return &Manager{
		viper:  v,
		config: &config.ServerConfig{},
	}
}

// Load loads configuration from file, environment variables, and defaults
func (m *Manager) Load(configPath string) error {
	// Set up Viper configuration
	m.setupViper(configPath)

	// Set defaults
	m.setDefaults()

	// Read configuration
	if err := m.viper.ReadInConfig(); err != nil {
		// Check if it's a config file not found error
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found is OK, we'll use defaults and env vars
		} else {
			// Check if it's a path error (file doesn't exist)
			if os.IsNotExist(err) {
				// File doesn't exist is OK, we'll use defaults and env vars
			} else {
				return fmt.Errorf("failed to read config file: %w", err)
			}
		}
	}

	// Unmarshal into struct
	if err := m.viper.Unmarshal(m.config); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate configuration
	if err := m.validate(); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	return nil
}

// GetConfig returns the loaded configuration
func (m *Manager) GetConfig() *config.ServerConfig {
	return m.config
}

// setupViper configures Viper for configuration loading
func (m *Manager) setupViper(configPath string) {
	// Set config file path
	if configPath != "" {
		m.viper.SetConfigFile(configPath)
	} else {
		// Look for config in standard locations (home directory first)
		m.viper.SetConfigName("server")
		m.viper.SetConfigType("yaml")

		// Get user's home directory
		homeDir, err := os.UserHomeDir()
		if err == nil {
			// Add ~/.dockbridge/configs as primary config location
			m.viper.AddConfigPath(filepath.Join(homeDir, ".dockbridge", "configs"))
			// Also check ~/.dockbridge for backward compatibility
			m.viper.AddConfigPath(filepath.Join(homeDir, ".dockbridge"))
		}

		// Fallback locations
		m.viper.AddConfigPath("./configs")
		m.viper.AddConfigPath("/etc/dockbridge")
		m.viper.AddConfigPath(".")
	}

	// Environment variable configuration
	m.viper.SetEnvPrefix("DOCKBRIDGE")
	m.viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	m.viper.AutomaticEnv()

	// Bind specific environment variables
	m.viper.BindEnv("docker.socket_path", "DOCKER_SOCKET_PATH")
	m.viper.BindEnv("logging.level", "LOG_LEVEL")
}

// setDefaults sets default configuration values
func (m *Manager) setDefaults() {
	// Docker defaults
	m.viper.SetDefault("docker.socket_path", "/var/run/docker.sock")
	m.viper.SetDefault("docker.proxy_port", 2376)

	// Keep-alive defaults
	m.viper.SetDefault("keepalive.interval", "30s")
	m.viper.SetDefault("keepalive.timeout", "5m")
	m.viper.SetDefault("keepalive.retry_interval", "5s")
	m.viper.SetDefault("keepalive.max_retries", 3)

	// Logging defaults
	m.viper.SetDefault("logging.level", "info")
	m.viper.SetDefault("logging.format", "json")
	m.viper.SetDefault("logging.output", "stdout")
}

// validate performs comprehensive configuration validation
func (m *Manager) validate() error {
	var errors []string

	// Validate Docker configuration
	if err := m.validateDocker(); err != nil {
		errors = append(errors, fmt.Sprintf("docker: %v", err))
	}

	// Validate Keep-alive configuration
	if err := m.validateKeepAlive(); err != nil {
		errors = append(errors, fmt.Sprintf("keepalive: %v", err))
	}

	// Validate Logging configuration
	if err := m.validateLogging(); err != nil {
		errors = append(errors, fmt.Sprintf("logging: %v", err))
	}

	if len(errors) > 0 {
		return fmt.Errorf("validation errors:\n  - %s", strings.Join(errors, "\n  - "))
	}

	return nil
}

// validateDocker validates Docker-specific configuration
func (m *Manager) validateDocker() error {
	docker := &m.config.Docker

	// Validate socket path exists (if not default)
	if docker.SocketPath != "/var/run/docker.sock" {
		if _, err := os.Stat(docker.SocketPath); err != nil {
			return fmt.Errorf("docker socket path '%s' does not exist or is not accessible", docker.SocketPath)
		}
	}

	// Validate proxy port
	if docker.ProxyPort < 1024 || docker.ProxyPort > 65535 {
		return fmt.Errorf("proxy_port must be between 1024 and 65535, got %d", docker.ProxyPort)
	}

	return nil
}

// validateKeepAlive validates keep-alive configuration
func (m *Manager) validateKeepAlive() error {
	keepAlive := &m.config.KeepAlive

	// Validate intervals
	if keepAlive.Interval < time.Second {
		return fmt.Errorf("interval must be at least 1 second, got %v", keepAlive.Interval)
	}

	if keepAlive.Timeout < keepAlive.Interval {
		return fmt.Errorf("timeout (%v) must be greater than interval (%v)", keepAlive.Timeout, keepAlive.Interval)
	}

	if keepAlive.RetryInterval < time.Second {
		return fmt.Errorf("retry_interval must be at least 1 second, got %v", keepAlive.RetryInterval)
	}

	// Validate max retries
	if keepAlive.MaxRetries < 0 || keepAlive.MaxRetries > 10 {
		return fmt.Errorf("max_retries must be between 0 and 10, got %d", keepAlive.MaxRetries)
	}

	return nil
}

// validateLogging validates logging configuration
func (m *Manager) validateLogging() error {
	logging := &m.config.Logging

	// Validate log level
	validLevels := []string{"debug", "info", "warn", "error", "fatal"}
	if !slices.Contains(validLevels, strings.ToLower(logging.Level)) {
		return fmt.Errorf("invalid level '%s', must be one of: %s", logging.Level, strings.Join(validLevels, ", "))
	}

	// Validate format
	validFormats := []string{"json", "text"}
	if !slices.Contains(validFormats, strings.ToLower(logging.Format)) {
		return fmt.Errorf("invalid format '%s', must be one of: %s", logging.Format, strings.Join(validFormats, ", "))
	}

	// Validate output
	validOutputs := []string{"stdout", "stderr"}
	if !slices.Contains(validOutputs, strings.ToLower(logging.Output)) && !strings.HasPrefix(logging.Output, "/") {
		return fmt.Errorf("invalid output '%s', must be 'stdout', 'stderr', or a file path", logging.Output)
	}

	return nil
}
