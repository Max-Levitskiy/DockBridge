package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	manager := NewManager()
	assert.NotNil(t, manager)
	assert.NotNil(t, manager.viper)
	assert.NotNil(t, manager.config)
}

func TestLoadWithDefaults(t *testing.T) {
	// Create a temporary directory for test
	tempDir := t.TempDir()

	manager := NewManager()

	// Load with non-existent config file (should use defaults)
	err := manager.Load(filepath.Join(tempDir, "nonexistent.yaml"))
	require.NoError(t, err)

	config := manager.GetConfig()

	// Test defaults are applied
	assert.Equal(t, "/var/run/docker.sock", config.Docker.SocketPath)
	assert.Equal(t, 2376, config.Docker.ProxyPort)

	assert.Equal(t, 30*time.Second, config.KeepAlive.Interval)
	assert.Equal(t, 5*time.Minute, config.KeepAlive.Timeout)
	assert.Equal(t, 5*time.Second, config.KeepAlive.RetryInterval)
	assert.Equal(t, 3, config.KeepAlive.MaxRetries)

	assert.Equal(t, "info", config.Logging.Level)
	assert.Equal(t, "json", config.Logging.Format)
	assert.Equal(t, "stdout", config.Logging.Output)
}

func TestLoadWithConfigFile(t *testing.T) {
	// Create a temporary config file
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "test-config.yaml")

	configContent := `
docker:
  proxy_port: 3000

keepalive:
  interval: "60s"
  timeout: "10m"

logging:
  level: "debug"
  format: "text"
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	manager := NewManager()
	err = manager.Load(configFile)
	require.NoError(t, err)

	config := manager.GetConfig()

	// Test config file values override defaults
	assert.Equal(t, 3000, config.Docker.ProxyPort)
	assert.Equal(t, 60*time.Second, config.KeepAlive.Interval)
	assert.Equal(t, 10*time.Minute, config.KeepAlive.Timeout)
	assert.Equal(t, "debug", config.Logging.Level)
	assert.Equal(t, "text", config.Logging.Format)

	// Test defaults still apply for unspecified values
	assert.Equal(t, "/var/run/docker.sock", config.Docker.SocketPath)
	assert.Equal(t, "stdout", config.Logging.Output)
}

func TestLoadWithEnvironmentVariables(t *testing.T) {
	// Set environment variables
	os.Setenv("DOCKBRIDGE_DOCKER_PROXY_PORT", "4000")
	os.Setenv("LOG_LEVEL", "error")
	defer func() {
		os.Unsetenv("DOCKBRIDGE_DOCKER_PROXY_PORT")
		os.Unsetenv("LOG_LEVEL")
	}()

	manager := NewManager()
	err := manager.Load("")
	require.NoError(t, err)

	config := manager.GetConfig()

	// Test environment variables override defaults
	assert.Equal(t, 4000, config.Docker.ProxyPort)
	assert.Equal(t, "error", config.Logging.Level)
}

func TestValidateDocker(t *testing.T) {
	tests := []struct {
		name        string
		setupConfig func(*Manager)
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid configuration",
			setupConfig: func(m *Manager) {
				m.config.Docker.SocketPath = "/var/run/docker.sock"
				m.config.Docker.ProxyPort = 2376
			},
			expectError: false,
		},
		{
			name: "invalid proxy port - too low",
			setupConfig: func(m *Manager) {
				m.config.Docker.SocketPath = "/var/run/docker.sock"
				m.config.Docker.ProxyPort = 80
			},
			expectError: true,
			errorMsg:    "proxy_port must be between 1024 and 65535",
		},
		{
			name: "invalid proxy port - too high",
			setupConfig: func(m *Manager) {
				m.config.Docker.SocketPath = "/var/run/docker.sock"
				m.config.Docker.ProxyPort = 70000
			},
			expectError: true,
			errorMsg:    "proxy_port must be between 1024 and 65535",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewManager()
			tt.setupConfig(manager)

			err := manager.validateDocker()

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateKeepAlive(t *testing.T) {
	tests := []struct {
		name        string
		setupConfig func(*Manager)
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid configuration",
			setupConfig: func(m *Manager) {
				m.config.KeepAlive.Interval = 30 * time.Second
				m.config.KeepAlive.Timeout = 5 * time.Minute
				m.config.KeepAlive.RetryInterval = 5 * time.Second
				m.config.KeepAlive.MaxRetries = 3
			},
			expectError: false,
		},
		{
			name: "interval too short",
			setupConfig: func(m *Manager) {
				m.config.KeepAlive.Interval = 500 * time.Millisecond
				m.config.KeepAlive.Timeout = 5 * time.Minute
				m.config.KeepAlive.RetryInterval = 5 * time.Second
				m.config.KeepAlive.MaxRetries = 3
			},
			expectError: true,
			errorMsg:    "interval must be at least 1 second",
		},
		{
			name: "timeout less than interval",
			setupConfig: func(m *Manager) {
				m.config.KeepAlive.Interval = 30 * time.Second
				m.config.KeepAlive.Timeout = 10 * time.Second
				m.config.KeepAlive.RetryInterval = 5 * time.Second
				m.config.KeepAlive.MaxRetries = 3
			},
			expectError: true,
			errorMsg:    "timeout (10s) must be greater than interval (30s)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewManager()
			tt.setupConfig(manager)

			err := manager.validateKeepAlive()

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateLogging(t *testing.T) {
	tests := []struct {
		name        string
		setupConfig func(*Manager)
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid configuration",
			setupConfig: func(m *Manager) {
				m.config.Logging.Level = "info"
				m.config.Logging.Format = "json"
				m.config.Logging.Output = "stdout"
			},
			expectError: false,
		},
		{
			name: "invalid log level",
			setupConfig: func(m *Manager) {
				m.config.Logging.Level = "invalid"
				m.config.Logging.Format = "json"
				m.config.Logging.Output = "stdout"
			},
			expectError: true,
			errorMsg:    "invalid level",
		},
		{
			name: "invalid format",
			setupConfig: func(m *Manager) {
				m.config.Logging.Level = "info"
				m.config.Logging.Format = "invalid"
				m.config.Logging.Output = "stdout"
			},
			expectError: true,
			errorMsg:    "invalid format",
		},
		{
			name: "valid file output",
			setupConfig: func(m *Manager) {
				m.config.Logging.Level = "info"
				m.config.Logging.Format = "json"
				m.config.Logging.Output = "/var/log/dockbridge.log"
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewManager()
			tt.setupConfig(manager)

			err := manager.validateLogging()

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestFullValidation(t *testing.T) {
	// Test that full validation catches multiple errors
	manager := NewManager()

	// Set up invalid configuration
	manager.config.Docker.ProxyPort = 80                       // Invalid port
	manager.config.KeepAlive.Interval = 500 * time.Millisecond // Too short
	manager.config.Logging.Level = "invalid"

	err := manager.validate()
	require.Error(t, err)

	// Should contain multiple validation errors
	assert.Contains(t, err.Error(), "docker:")
	assert.Contains(t, err.Error(), "keepalive:")
	assert.Contains(t, err.Error(), "logging:")
}

func TestContainsHelper(t *testing.T) {
	slice := []string{"a", "b", "c"}

	assert.True(t, contains(slice, "a"))
	assert.True(t, contains(slice, "b"))
	assert.True(t, contains(slice, "c"))
	assert.False(t, contains(slice, "d"))
	assert.False(t, contains(slice, ""))
}
