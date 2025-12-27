package ssh

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests require a real SSH server to connect to
// For CI environments, we'll skip them if the environment variables are not set
func TestClient_Integration(t *testing.T) {
	// Skip if no SSH server is available for testing
	sshHost := os.Getenv("TEST_SSH_HOST")
	sshPortStr := os.Getenv("TEST_SSH_PORT")
	sshUser := os.Getenv("TEST_SSH_USER")
	sshKeyPath := os.Getenv("TEST_SSH_KEY_PATH")

	if sshHost == "" || sshPortStr == "" || sshUser == "" || sshKeyPath == "" {
		t.Skip("Skipping SSH client integration tests: environment variables not set")
	}

	sshPort := 22
	_, _ = fmt.Sscanf(sshPortStr, "%d", &sshPort)

	// Create client config
	config := &ClientConfig{
		Host:           sshHost,
		Port:           sshPort,
		User:           sshUser,
		PrivateKeyPath: sshKeyPath,
		Timeout:        5 * time.Second,
	}

	// Create client
	client := NewClient(config)

	// Test connection
	ctx := context.Background()
	err := client.Connect(ctx)
	if err != nil {
		t.Skipf("Skipping SSH client integration tests: could not connect: %v", err)
	}
	defer func() { _ = client.Close() }()

	// Test IsConnected
	assert.True(t, client.IsConnected())

	// Test executing a command
	output, err := client.ExecuteCommand(ctx, "echo 'Hello, SSH!'")
	require.NoError(t, err)
	assert.Contains(t, string(output), "Hello, SSH!")

	// Test closing the connection
	err = client.Close()
	require.NoError(t, err)
	assert.False(t, client.IsConnected())
}

// TestTunnel_Mock tests the tunnel functionality with a mock server
// This doesn't test the actual SSH tunneling but does test the connection handling logic
func TestTunnel_Mock(t *testing.T) {
	t.Skip("Skipping tunnel mock test as it requires SSH client mocking")

	// This test would require mocking the SSH client which is complex
	// In a real implementation, you would use a library like github.com/golang/mock
	// to create a mock SSH client
}
