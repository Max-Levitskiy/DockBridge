package server

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/dockbridge/dockbridge/internal/client/hetzner"
	"github.com/dockbridge/dockbridge/internal/shared/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDockerStatePersistenceIntegration tests that Docker state persists across server recreations
// This is a comprehensive integration test that requires a real Hetzner API token
func TestDockerStatePersistenceIntegration(t *testing.T) {
	// Skip if no API token is provided
	apiToken := os.Getenv("HETZNER_API_TOKEN")
	if apiToken == "" {
		t.Skip("Skipping integration test: HETZNER_API_TOKEN not set")
	}

	// Use smallest server type for cost efficiency
	hetznerConfig := &hetzner.Config{
		APIToken:   apiToken,
		ServerType: "cpx11", // Smallest server for testing
		Location:   "fsn1",
		VolumeSize: 10, // Minimum volume size
	}

	clientConfig := &config.HetznerConfig{
		APIToken:   apiToken,
		ServerType: "cpx11",
		Location:   "fsn1",
		VolumeSize: 10,
	}

	hetznerClient, err := hetzner.NewClient(hetznerConfig)
	require.NoError(t, err)

	manager := NewManager(hetznerClient, clientConfig)
	ctx := context.Background()

	// Cleanup function to ensure resources are cleaned up
	var serverID string
	var volumeID string
	defer func() {
		if serverID != "" {
			t.Logf("Cleaning up server: %s", serverID)
			_ = manager.DestroyServer(ctx, serverID)
		}
		// Note: We intentionally don't clean up the volume to test persistence
		// In a real scenario, you might want to clean it up to avoid costs
		if volumeID != "" {
			t.Logf("Volume %s left for manual cleanup (to avoid data loss)", volumeID)
		}
	}()

	t.Run("CreateServerWithVolume", func(t *testing.T) {
		// Step 1: Ensure volume exists
		volume, err := manager.EnsureVolume(ctx)
		require.NoError(t, err)
		assert.NotNil(t, volume)
		assert.Equal(t, "/var/lib/docker", volume.MountPath)
		volumeID = volume.ID
		t.Logf("Created/found volume: %s (ID: %s)", volume.Name, volume.ID)

		// Step 2: Provision server with volume
		server, err := manager.EnsureServer(ctx)
		require.NoError(t, err)
		assert.NotNil(t, server)
		assert.Equal(t, volume.ID, server.VolumeID)
		assert.Equal(t, StatusRunning, server.Status)
		serverID = server.ID
		t.Logf("Provisioned server: %s (ID: %s)", server.Name, server.ID)

		// Verify volume information in metadata
		assert.Equal(t, "/var/lib/docker", server.Metadata["docker_data_dir"])
		assert.NotEmpty(t, server.Metadata["volume_name"])
		assert.Equal(t, "10", server.Metadata["volume_size"])
	})

	t.Run("VerifyVolumeMount", func(t *testing.T) {
		// Wait for server to be fully ready
		time.Sleep(2 * time.Minute)

		// In a real test, you would SSH to the server and verify:
		// 1. Volume is mounted at /var/lib/docker
		// 2. Docker daemon is using the volume
		// 3. Docker commands work properly

		// For now, we'll just verify the server is running
		status, err := manager.GetServerStatus(ctx)
		require.NoError(t, err)
		assert.Equal(t, StatusRunning, *status)

		t.Log("Server is running and should have volume mounted at /var/lib/docker")
	})

	t.Run("DestroyServerPreserveVolume", func(t *testing.T) {
		// Destroy server while preserving volume
		err := manager.DestroyServer(ctx, serverID)
		require.NoError(t, err)
		t.Logf("Destroyed server %s, volume should be preserved", serverID)

		// Reset serverID since it's destroyed
		serverID = ""

		// Wait for server to be fully destroyed
		time.Sleep(30 * time.Second)

		// Verify server is gone
		status, err := manager.GetServerStatus(ctx)
		require.NoError(t, err)
		assert.Equal(t, StatusTerminated, *status)
	})

	t.Run("RecreateServerWithSameVolume", func(t *testing.T) {
		// Create a new server that should reuse the existing volume
		server, err := manager.EnsureServer(ctx)
		require.NoError(t, err)
		assert.NotNil(t, server)
		assert.Equal(t, volumeID, server.VolumeID) // Should reuse the same volume
		assert.Equal(t, StatusRunning, server.Status)
		serverID = server.ID
		t.Logf("Recreated server: %s (ID: %s) with same volume: %s", server.Name, server.ID, server.VolumeID)

		// Verify volume information is preserved
		assert.Equal(t, "/var/lib/docker", server.Metadata["docker_data_dir"])
	})

	t.Run("VerifyDockerStatePersistence", func(t *testing.T) {
		// Wait for server to be fully ready
		time.Sleep(2 * time.Minute)

		// In a real test, you would SSH to the server and verify:
		// 1. Previous Docker containers/images are still available
		// 2. Docker daemon starts successfully with existing data
		// 3. All Docker state is preserved

		// For now, we'll just verify the server is running with the correct volume
		servers, err := manager.ListServers(ctx)
		require.NoError(t, err)
		require.Len(t, servers, 1)

		server := servers[0]
		assert.Equal(t, volumeID, server.VolumeID)
		assert.Equal(t, "/var/lib/docker", server.Metadata["docker_data_dir"])

		t.Log("Server recreated successfully with persistent volume")
		t.Log("In a full test, Docker state persistence would be verified via SSH")
	})
}

// TestCloudInitScriptGeneration tests the enhanced cloud-init script generation
func TestCloudInitScriptGeneration(t *testing.T) {
	tests := []struct {
		name   string
		config *hetzner.CloudInitConfig
	}{
		{
			name: "default configuration",
			config: &hetzner.CloudInitConfig{
				DockerVersion: "latest",
				VolumeMount:   "/var/lib/docker",
				KeepAlivePort: 8080,
				DockerAPIPort: 2376,
			},
		},
		{
			name: "with SSH key",
			config: &hetzner.CloudInitConfig{
				DockerVersion: "latest",
				SSHPublicKey:  "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC7... test@example.com",
				VolumeMount:   "/var/lib/docker",
				KeepAlivePort: 8080,
				DockerAPIPort: 2376,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			script := hetzner.GenerateCloudInitScript(tt.config)

			// Verify script contains essential components
			assert.Contains(t, script, "#cloud-config")
			assert.Contains(t, script, "docker-ce")
			assert.Contains(t, script, "/var/lib/docker")
			assert.Contains(t, script, "data-root")
			assert.Contains(t, script, "mkfs.ext4")
			assert.Contains(t, script, "UUID=")
			assert.Contains(t, script, "mountpoint")
			assert.Contains(t, script, "docker-volume-health-check")

			// Verify enhanced volume management features
			assert.Contains(t, script, "VOLUME_DEVICE")
			assert.Contains(t, script, "docker-backup")
			assert.Contains(t, script, "Volume mount verification")
			assert.Contains(t, script, "health check")

			if tt.config.SSHPublicKey != "" {
				assert.Contains(t, script, "ssh_authorized_keys")
				assert.Contains(t, script, tt.config.SSHPublicKey)
			}
		})
	}
}

// TestDefaultCloudInitConfig tests the default configuration
func TestDefaultCloudInitConfig(t *testing.T) {
	config := hetzner.GetDefaultCloudInitConfig()

	assert.Equal(t, "latest", config.DockerVersion)
	assert.Equal(t, "/var/lib/docker", config.VolumeMount)
	assert.Equal(t, 8080, config.KeepAlivePort)
	assert.Equal(t, 2376, config.DockerAPIPort)
	assert.Contains(t, config.Packages, "e2fsprogs")
	assert.Contains(t, config.Packages, "parted")
}

// BenchmarkCloudInitGeneration benchmarks the enhanced cloud-init script generation
func BenchmarkCloudInitGeneration(b *testing.B) {
	config := &hetzner.CloudInitConfig{
		DockerVersion: "latest",
		SSHPublicKey:  "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC7vMzlJKvQqGHWDQz1234567890abcdefghijklmnopqrstuvwxyz test@example.com",
		VolumeMount:   "/var/lib/docker",
		KeepAlivePort: 8080,
		DockerAPIPort: 2376,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = hetzner.GenerateCloudInitScript(config)
	}
}
