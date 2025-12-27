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

	// Set environment variable for API token
	os.Setenv("HETZNER_API_TOKEN", "test-token")
	defer os.Unsetenv("HETZNER_API_TOKEN")

	manager := NewManager()

	// Load with non-existent config file (should use defaults)
	err := manager.Load(filepath.Join(tempDir, "nonexistent.yaml"))
	require.NoError(t, err)

	config := manager.GetConfig()

	// Test defaults are applied
	assert.Equal(t, "test-token", config.Hetzner.APIToken)
	assert.Equal(t, "cpx21", config.Hetzner.ServerType)
	assert.Equal(t, "fsn1", config.Hetzner.Location)
	assert.Equal(t, 10, config.Hetzner.VolumeSize)

	assert.Equal(t, "/var/run/docker.sock", config.Docker.SocketPath)
	assert.Equal(t, 2376, config.Docker.ProxyPort)

	assert.Equal(t, 30*time.Second, config.KeepAlive.Interval)
	assert.Equal(t, 5*time.Minute, config.KeepAlive.Timeout)
	assert.Equal(t, 5*time.Second, config.KeepAlive.RetryInterval)
	assert.Equal(t, 3, config.KeepAlive.MaxRetries)

	assert.Equal(t, 22, config.SSH.Port)
	assert.Equal(t, 30*time.Second, config.SSH.Timeout)
	assert.Equal(t, 30*time.Second, config.SSH.KeepAlive)

	assert.Equal(t, "info", config.Logging.Level)
	assert.Equal(t, "json", config.Logging.Format)
	assert.Equal(t, "stdout", config.Logging.Output)
}

func TestLoadWithConfigFile(t *testing.T) {
	// Create a temporary config file
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "test-config.yaml")

	configContent := `
hetzner:
  api_token: "file-token"
  server_type: "cx31"
  location: "nbg1"
  volume_size: 20

docker:
  proxy_port: 3000

logging:
  level: "debug"
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	manager := NewManager()
	err = manager.Load(configFile)
	require.NoError(t, err)

	config := manager.GetConfig()

	// Test config file values override defaults
	assert.Equal(t, "file-token", config.Hetzner.APIToken)
	assert.Equal(t, "cx31", config.Hetzner.ServerType)
	assert.Equal(t, "nbg1", config.Hetzner.Location)
	assert.Equal(t, 20, config.Hetzner.VolumeSize)
	assert.Equal(t, 3000, config.Docker.ProxyPort)
	assert.Equal(t, "debug", config.Logging.Level)

	// Test defaults still apply for unspecified values
	assert.Equal(t, "/var/run/docker.sock", config.Docker.SocketPath)
}

func TestLoadWithEnvironmentVariables(t *testing.T) {
	// Set environment variables
	os.Setenv("HETZNER_API_TOKEN", "env-token")
	os.Setenv("DOCKBRIDGE_DOCKER_PROXY_PORT", "4000")
	os.Setenv("DOCKER_SOCKET_PATH", "/var/run/docker.sock")
	os.Setenv("LOG_LEVEL", "error")
	defer func() {
		os.Unsetenv("HETZNER_API_TOKEN")
		os.Unsetenv("DOCKBRIDGE_DOCKER_PROXY_PORT")
		os.Unsetenv("DOCKER_SOCKET_PATH")
		os.Unsetenv("LOG_LEVEL")
	}()

	manager := NewManager()
	err := manager.Load("")
	require.NoError(t, err)

	config := manager.GetConfig()

	// Test environment variables override defaults
	assert.Equal(t, "env-token", config.Hetzner.APIToken)
	assert.Equal(t, 4000, config.Docker.ProxyPort)
	assert.Equal(t, "error", config.Logging.Level)
}

func TestValidateHetzner(t *testing.T) {
	tests := []struct {
		name        string
		setupConfig func(*Manager)
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid configuration",
			setupConfig: func(m *Manager) {
				m.config.Hetzner.APIToken = "valid-token"
				m.config.Hetzner.ServerType = "cpx21"
				m.config.Hetzner.Location = "fsn1"
				m.config.Hetzner.VolumeSize = 10
			},
			expectError: false,
		},
		{
			name: "missing API token",
			setupConfig: func(m *Manager) {
				m.config.Hetzner.APIToken = ""
				m.config.Hetzner.ServerType = "cpx21"
				m.config.Hetzner.Location = "fsn1"
				m.config.Hetzner.VolumeSize = 10
			},
			expectError: true,
			errorMsg:    "api_token is required",
		},
		{
			name: "invalid server type",
			setupConfig: func(m *Manager) {
				m.config.Hetzner.APIToken = "valid-token"
				m.config.Hetzner.ServerType = "invalid-type"
				m.config.Hetzner.Location = "fsn1"
				m.config.Hetzner.VolumeSize = 10
			},
			expectError: true,
			errorMsg:    "invalid server_type",
		},
		{
			name: "invalid location",
			setupConfig: func(m *Manager) {
				m.config.Hetzner.APIToken = "valid-token"
				m.config.Hetzner.ServerType = "cpx21"
				m.config.Hetzner.Location = "invalid-location"
				m.config.Hetzner.VolumeSize = 10
			},
			expectError: true,
			errorMsg:    "invalid location",
		},
		{
			name: "volume size too small",
			setupConfig: func(m *Manager) {
				m.config.Hetzner.APIToken = "valid-token"
				m.config.Hetzner.ServerType = "cpx21"
				m.config.Hetzner.Location = "fsn1"
				m.config.Hetzner.VolumeSize = 5
			},
			expectError: true,
			errorMsg:    "volume_size must be between 10 and 10000",
		},
		{
			name: "volume size too large",
			setupConfig: func(m *Manager) {
				m.config.Hetzner.APIToken = "valid-token"
				m.config.Hetzner.ServerType = "cpx21"
				m.config.Hetzner.Location = "fsn1"
				m.config.Hetzner.VolumeSize = 15000
			},
			expectError: true,
			errorMsg:    "volume_size must be between 10 and 10000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewManager()
			tt.setupConfig(manager)

			err := manager.validateHetzner()

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
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
		{
			name: "retry interval too short",
			setupConfig: func(m *Manager) {
				m.config.KeepAlive.Interval = 30 * time.Second
				m.config.KeepAlive.Timeout = 5 * time.Minute
				m.config.KeepAlive.RetryInterval = 500 * time.Millisecond
				m.config.KeepAlive.MaxRetries = 3
			},
			expectError: true,
			errorMsg:    "retry_interval must be at least 1 second",
		},
		{
			name: "max retries too high",
			setupConfig: func(m *Manager) {
				m.config.KeepAlive.Interval = 30 * time.Second
				m.config.KeepAlive.Timeout = 5 * time.Minute
				m.config.KeepAlive.RetryInterval = 5 * time.Second
				m.config.KeepAlive.MaxRetries = 15
			},
			expectError: true,
			errorMsg:    "max_retries must be between 0 and 10",
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

func TestValidateSSH(t *testing.T) {
	tests := []struct {
		name        string
		setupConfig func(*Manager, string)
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid configuration",
			setupConfig: func(m *Manager, tempDir string) {
				m.config.SSH.KeyPath = filepath.Join(tempDir, "ssh", "id_rsa")
				m.config.SSH.Port = 22
				m.config.SSH.Timeout = 30 * time.Second
				m.config.SSH.KeepAlive = 30 * time.Second
			},
			expectError: false,
		},
		{
			name: "invalid port - too low",
			setupConfig: func(m *Manager, tempDir string) {
				m.config.SSH.KeyPath = filepath.Join(tempDir, "ssh", "id_rsa")
				m.config.SSH.Port = 0
				m.config.SSH.Timeout = 30 * time.Second
				m.config.SSH.KeepAlive = 30 * time.Second
			},
			expectError: true,
			errorMsg:    "port must be between 1 and 65535",
		},
		{
			name: "invalid port - too high",
			setupConfig: func(m *Manager, tempDir string) {
				m.config.SSH.KeyPath = filepath.Join(tempDir, "ssh", "id_rsa")
				m.config.SSH.Port = 70000
				m.config.SSH.Timeout = 30 * time.Second
				m.config.SSH.KeepAlive = 30 * time.Second
			},
			expectError: true,
			errorMsg:    "port must be between 1 and 65535",
		},
		{
			name: "timeout too short",
			setupConfig: func(m *Manager, tempDir string) {
				m.config.SSH.KeyPath = filepath.Join(tempDir, "ssh", "id_rsa")
				m.config.SSH.Port = 22
				m.config.SSH.Timeout = 500 * time.Millisecond
				m.config.SSH.KeepAlive = 30 * time.Second
			},
			expectError: true,
			errorMsg:    "timeout must be at least 1 second",
		},
		{
			name: "keep_alive too short",
			setupConfig: func(m *Manager, tempDir string) {
				m.config.SSH.KeyPath = filepath.Join(tempDir, "ssh", "id_rsa")
				m.config.SSH.Port = 22
				m.config.SSH.Timeout = 30 * time.Second
				m.config.SSH.KeepAlive = 500 * time.Millisecond
			},
			expectError: true,
			errorMsg:    "keep_alive must be at least 1 second",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			manager := NewManager()
			tt.setupConfig(manager, tempDir)

			err := manager.validateSSH()

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
			name: "invalid output",
			setupConfig: func(m *Manager) {
				m.config.Logging.Level = "info"
				m.config.Logging.Format = "json"
				m.config.Logging.Output = "invalid"
			},
			expectError: true,
			errorMsg:    "invalid output",
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
	manager.config.Hetzner.APIToken = "" // Missing token
	manager.config.Hetzner.ServerType = "invalid"
	manager.config.Docker.ProxyPort = 80 // Invalid port
	manager.config.Logging.Level = "invalid"

	err := manager.validate()
	require.Error(t, err)

	// Should contain multiple validation errors
	assert.Contains(t, err.Error(), "hetzner:")
	assert.Contains(t, err.Error(), "docker:")
	assert.Contains(t, err.Error(), "logging:")
}
