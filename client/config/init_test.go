package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitializeDefaultConfig(t *testing.T) {
	// Set a temporary home directory for testing
	originalHome := os.Getenv("HOME")
	tempDir := t.TempDir()
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	// Initialize default config
	err := InitializeDefaultConfig()
	require.NoError(t, err)

	// Check that the config directory was created
	configDir := filepath.Join(tempDir, ".dockbridge")
	_, err = os.Stat(configDir)
	assert.NoError(t, err)

	// Check that the client config file was created
	clientConfigPath := filepath.Join(configDir, "configs", "client.yaml")
	_, err = os.Stat(clientConfigPath)
	assert.NoError(t, err)

	// Check that the server config file was created
	serverConfigPath := filepath.Join(configDir, "configs", "server.yaml")
	_, err = os.Stat(serverConfigPath)
	assert.NoError(t, err)

	// Check the content of the client config file
	clientContent, err := os.ReadFile(clientConfigPath)
	require.NoError(t, err)
	assert.Contains(t, string(clientContent), "# DockBridge Client Configuration")
	assert.Contains(t, string(clientContent), "hetzner:")
	assert.Contains(t, string(clientContent), "docker:")
	assert.Contains(t, string(clientContent), "keepalive:")
	assert.Contains(t, string(clientContent), "ssh:")
	assert.Contains(t, string(clientContent), "logging:")

	// Check the content of the server config file
	serverContent, err := os.ReadFile(serverConfigPath)
	require.NoError(t, err)
	assert.Contains(t, string(serverContent), "# DockBridge Server Configuration")
	assert.Contains(t, string(serverContent), "docker:")
	assert.Contains(t, string(serverContent), "keepalive:")
	assert.Contains(t, string(serverContent), "logging:")
}

func TestGetDefaultConfigPath(t *testing.T) {
	// Set a temporary home directory for testing
	originalHome := os.Getenv("HOME")
	tempDir := t.TempDir()
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	// Test client config path
	clientPath, err := GetDefaultConfigPath("client")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(tempDir, ".dockbridge", "client.yaml"), clientPath)

	// Test server config path
	serverPath, err := GetDefaultConfigPath("server")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(tempDir, ".dockbridge", "server.yaml"), serverPath)
}
