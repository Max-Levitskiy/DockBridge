package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dockbridge/dockbridge/internal/shared/config"
	"github.com/spf13/viper"
)

// Manager handles configuration loading and validation for the client
type Manager struct {
	viper  *viper.Viper
	config *config.ClientConfig
}

// NewManager creates a new configuration manager
func NewManager() *Manager {
	v := viper.New()
	return &Manager{
		viper:  v,
		config: &config.ClientConfig{},
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
func (m *Manager) GetConfig() *config.ClientConfig {
	return m.config
}

// setupViper configures Viper for configuration loading
func (m *Manager) setupViper(configPath string) {
	// Set config file path
	if configPath != "" {
		m.viper.SetConfigFile(configPath)
	} else {
		// Look for config in standard locations (home directory first)
		m.viper.SetConfigName("client")
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
	m.viper.BindEnv("hetzner.api_token", "HETZNER_API_TOKEN")
	m.viper.BindEnv("docker.socket_path", "DOCKER_SOCKET_PATH")
	m.viper.BindEnv("logging.level", "LOG_LEVEL")
}

// setDefaults sets default configuration values
func (m *Manager) setDefaults() {
	// Hetzner defaults
	m.viper.SetDefault("hetzner.server_type", "cpx21")
	m.viper.SetDefault("hetzner.location", "fsn1")
	m.viper.SetDefault("hetzner.volume_size", 10)

	// Docker defaults
	m.viper.SetDefault("docker.socket_path", "/var/run/docker.sock")
	m.viper.SetDefault("docker.proxy_port", 2376)

	// Activity defaults - Reasonable production values
	m.viper.SetDefault("activity.idle_timeout", "5m")
	m.viper.SetDefault("activity.connection_timeout", "30m")
	m.viper.SetDefault("activity.grace_period", "30s")

	// Keep-alive defaults
	m.viper.SetDefault("keepalive.interval", "30s")
	m.viper.SetDefault("keepalive.timeout", "5m")
	m.viper.SetDefault("keepalive.retry_interval", "5s")
	m.viper.SetDefault("keepalive.max_retries", 3)

	// SSH defaults
	homeDir, _ := os.UserHomeDir()
	defaultKeyPath := filepath.Join(homeDir, ".dockbridge", "ssh", "id_rsa")
	m.viper.SetDefault("ssh.key_path", defaultKeyPath)
	m.viper.SetDefault("ssh.port", 22)
	m.viper.SetDefault("ssh.timeout", "30s")
	m.viper.SetDefault("ssh.keep_alive", "30s")

	// Logging defaults
	m.viper.SetDefault("logging.level", "info")
	m.viper.SetDefault("logging.format", "json")
	m.viper.SetDefault("logging.output", "stdout")
}

// validate performs comprehensive configuration validation
func (m *Manager) validate() error {
	var errors []string

	// Validate Hetzner configuration
	if err := m.validateHetzner(); err != nil {
		errors = append(errors, fmt.Sprintf("hetzner: %v", err))
	}

	// Validate Docker configuration
	if err := m.validateDocker(); err != nil {
		errors = append(errors, fmt.Sprintf("docker: %v", err))
	}

	// Validate Activity configuration
	if err := m.validateActivity(); err != nil {
		errors = append(errors, fmt.Sprintf("activity: %v", err))
	}

	// Validate Keep-alive configuration
	if err := m.validateKeepAlive(); err != nil {
		errors = append(errors, fmt.Sprintf("keepalive: %v", err))
	}

	// Validate SSH configuration
	if err := m.validateSSH(); err != nil {
		errors = append(errors, fmt.Sprintf("ssh: %v", err))
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

// validateHetzner validates Hetzner-specific configuration
func (m *Manager) validateHetzner() error {
	hetzner := &m.config.Hetzner

	// API token is required
	if hetzner.APIToken == "" {
		return fmt.Errorf("api_token is required (set via HETZNER_API_TOKEN environment variable or config file)")
	}

	// Validate server type
	validServerTypes := []string{"cx11", "cpx11", "cx21", "cpx21", "cx31", "cpx31", "cx41", "cpx41", "cx51", "cpx51"}
	if !contains(validServerTypes, hetzner.ServerType) {
		return fmt.Errorf("invalid server_type '%s', must be one of: %s", hetzner.ServerType, strings.Join(validServerTypes, ", "))
	}

	// Validate location
	validLocations := []string{"fsn1", "nbg1", "hel1", "ash", "hil"}
	if !contains(validLocations, hetzner.Location) {
		return fmt.Errorf("invalid location '%s', must be one of: %s", hetzner.Location, strings.Join(validLocations, ", "))
	}

	// Validate volume size
	if hetzner.VolumeSize < 10 || hetzner.VolumeSize > 10000 {
		return fmt.Errorf("volume_size must be between 10 and 10000 GB, got %d", hetzner.VolumeSize)
	}

	return nil
}

// validateDocker validates Docker-specific configuration
func (m *Manager) validateDocker() error {
	docker := &m.config.Docker

	// Validate socket path directory exists and is writable (DockBridge will create the socket)
	if docker.SocketPath != "/var/run/docker.sock" && docker.SocketPath != "tcp" && !strings.HasPrefix(docker.SocketPath, ":") {
		// Check if the directory exists and is writable
		dir := filepath.Dir(docker.SocketPath)
		if _, err := os.Stat(dir); err != nil {
			return fmt.Errorf("directory for docker socket path '%s' does not exist: %s", docker.SocketPath, dir)
		}

		// Test if we can write to the directory by creating a temporary file
		tempFile := filepath.Join(dir, ".dockbridge-test")
		if file, err := os.Create(tempFile); err != nil {
			return fmt.Errorf("cannot write to directory for docker socket path '%s': %s", docker.SocketPath, dir)
		} else {
			file.Close()
			os.Remove(tempFile)
		}
	}

	// Validate proxy port
	if docker.ProxyPort < 1024 || docker.ProxyPort > 65535 {
		return fmt.Errorf("proxy_port must be between 1024 and 65535, got %d", docker.ProxyPort)
	}

	return nil
}

// validateActivity validates activity tracking configuration
func (m *Manager) validateActivity() error {
	activity := &m.config.Activity

	// Validate idle timeout - Allow short timeouts for testing but reasonable minimums
	if activity.IdleTimeout < 30*time.Second {
		return fmt.Errorf("idle_timeout must be at least 30 seconds, got %v", activity.IdleTimeout)
	}

	// Validate connection timeout - Allow short timeouts for testing but reasonable minimums
	if activity.ConnectionTimeout < time.Minute {
		return fmt.Errorf("connection_timeout must be at least 1 minute, got %v", activity.ConnectionTimeout)
	}

	// Validate grace period
	if activity.GracePeriod < time.Second {
		return fmt.Errorf("grace_period must be at least 1 second, got %v", activity.GracePeriod)
	}

	// Ensure connection timeout is longer than idle timeout
	if activity.ConnectionTimeout <= activity.IdleTimeout {
		return fmt.Errorf("connection_timeout (%v) must be greater than idle_timeout (%v)", activity.ConnectionTimeout, activity.IdleTimeout)
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

// validateSSH validates SSH configuration
func (m *Manager) validateSSH() error {
	ssh := &m.config.SSH

	// Validate key path directory exists
	keyDir := filepath.Dir(ssh.KeyPath)
	if err := os.MkdirAll(keyDir, 0700); err != nil {
		return fmt.Errorf("cannot create SSH key directory '%s': %v", keyDir, err)
	}

	// Validate port
	if ssh.Port < 1 || ssh.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535, got %d", ssh.Port)
	}

	// Validate timeout
	if ssh.Timeout < time.Second {
		return fmt.Errorf("timeout must be at least 1 second, got %v", ssh.Timeout)
	}

	if ssh.KeepAlive < time.Second {
		return fmt.Errorf("keep_alive must be at least 1 second, got %v", ssh.KeepAlive)
	}

	return nil
}

// validateLogging validates logging configuration
func (m *Manager) validateLogging() error {
	logging := &m.config.Logging

	// Validate log level
	validLevels := []string{"debug", "info", "warn", "error", "fatal"}
	if !contains(validLevels, strings.ToLower(logging.Level)) {
		return fmt.Errorf("invalid level '%s', must be one of: %s", logging.Level, strings.Join(validLevels, ", "))
	}

	// Validate format
	validFormats := []string{"json", "text"}
	if !contains(validFormats, strings.ToLower(logging.Format)) {
		return fmt.Errorf("invalid format '%s', must be one of: %s", logging.Format, strings.Join(validFormats, ", "))
	}

	// Validate output
	validOutputs := []string{"stdout", "stderr"}
	if !contains(validOutputs, strings.ToLower(logging.Output)) && !strings.HasPrefix(logging.Output, "/") {
		return fmt.Errorf("invalid output '%s', must be 'stdout', 'stderr', or a file path", logging.Output)
	}

	return nil
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
