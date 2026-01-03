package config

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	manager := NewManager()
	assert.NotNil(t, manager)
	assert.NotNil(t, manager.config)
}

func TestDefaultConfiguration(t *testing.T) {
	manager := NewManager()
	// Must load to apply defaults
	// Provide non-existent file to ensure we test defaults and not user config
	_ = manager.Load("nonexistent-test-config.yaml")
	cfg := manager.GetConfig()

	// Check default values
	assert.Equal(t, "cpx21", cfg.Hetzner.ServerType)
	assert.Equal(t, "fsn1", cfg.Hetzner.Location)
	assert.Equal(t, 10, cfg.Hetzner.VolumeSize)
	// PreferredImages usually empty by default in config.go
	// assert.Contains(t, cfg.Hetzner.PreferredImages, "docker-ce")

	assert.Equal(t, "/var/run/docker.sock", cfg.Docker.SocketPath)
	assert.Equal(t, 2376, cfg.Docker.ProxyPort)

	assert.Equal(t, 30*time.Second, cfg.KeepAlive.Interval)
	assert.Equal(t, 5*time.Minute, cfg.KeepAlive.Timeout)

	assert.Equal(t, 22, cfg.SSH.Port)
	assert.Contains(t, cfg.SSH.KeyPath, ".dockbridge/ssh")
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
				m.config.Hetzner.APIToken = "test-token"
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
			},
			expectError: true,
			errorMsg:    "api_token is required",
		},
		{
			name: "missing server type",
			setupConfig: func(m *Manager) {
				m.config.Hetzner.APIToken = "test-token"
				m.config.Hetzner.ServerType = ""
			},
			expectError: true,
			errorMsg:    "server_type is required",
		},
		{
			name: "invalid volume size",
			setupConfig: func(m *Manager) {
				m.config.Hetzner.APIToken = "test-token"
				m.config.Hetzner.ServerType = "cpx21"
				m.config.Hetzner.Location = "fsn1"
				m.config.Hetzner.VolumeSize = 5 // Minimum is usually 10
			},
			expectError: true,
			errorMsg:    "volume_size must be between 10 and 10000 GB",
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
