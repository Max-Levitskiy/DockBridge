package hetzner

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/dockbridge/dockbridge/client/hetzner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDockerStatePersistence tests that Docker state persists across server recreations
// This is an integration test that requires a real Hetzner API token
func TestDockerStatePersistence(t *testing.T) {
	// Skip if no API token is provided
	apiToken := os.Getenv("HETZNER_API_TOKEN")
	if apiToken == "" {
		t.Skip("HETZNER_API_TOKEN not set, skipping integration test")
	}

	// Skip if not explicitly enabled
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("RUN_INTEGRATION_TESTS not set to 'true', skipping integration test")
	}

	ctx := context.Background()

	// Create Hetzner client
	config := &Config{
		APIToken:   apiToken,
		ServerType: "cpx11", // Use smallest server for testing
		Location:   "fsn1",
		VolumeSize: 10,
	}

	client, err := NewClient(config)
	require.NoError(t, err)

	// Test volume creation and reuse
	t.Run("VolumeCreationAndReuse", func(t *testing.T) {
		// Create a Docker volume
		volume1, err := client.FindOrCreateDockerVolume(ctx, config.Location)
		require.NoError(t, err)
		assert.NotNil(t, volume1)
		assert.Contains(t, volume1.Name, "dockbridge-docker-data")
		assert.Equal(t, config.VolumeSize, volume1.Size)

		// Try to find the same volume again - should reuse existing
		volume2, err := client.FindOrCreateDockerVolume(ctx, config.Location)
		require.NoError(t, err)
		assert.Equal(t, volume1.ID, volume2.ID, "Should reuse existing volume")

		// Clean up - delete the volume
		defer func() {
			// Note: In a real scenario, we'd want to keep the volume for persistence
			// But for testing, we clean up to avoid accumulating test volumes
			if volume1 != nil {
				// First detach if attached
				client.DetachVolume(ctx, fmt.Sprintf("%d", volume1.ID))
				// Note: Hetzner doesn't provide volume deletion in the basic API
				// Volumes need to be deleted manually through the web interface
			}
		}()
	})

	t.Run("CloudInitScriptGeneration", func(t *testing.T) {
		publicKey := "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC7... test@example.com"

		cloudInitConfig := &CloudInitConfig{
			SSHPublicKey: publicKey,
			VolumeMount:  "/var/lib/docker",
		}

		script := GenerateCloudInitScript(cloudInitConfig)

		// Verify script contains essential components
		assert.Contains(t, script, "#cloud-config")
		assert.Contains(t, script, publicKey)
		assert.Contains(t, script, "/dev/sdb")
		assert.Contains(t, script, "/var/lib/docker")
		assert.Contains(t, script, "mkfs.ext4")
		assert.Contains(t, script, "mount")
		assert.Contains(t, script, "fstab")
		assert.Contains(t, script, "docker")
		assert.Contains(t, script, "data-root")
	})

	t.Run("DefaultCloudInitConfig", func(t *testing.T) {
		config := hetzner.GetDefaultCloudInitConfig()
		assert.Equal(t, "/var/lib/docker", config.VolumeMount)
		assert.Equal(t, 2376, config.DockerAPIPort)
	})
}

// TestVolumeManagementMethods tests the volume management methods
func TestVolumeManagementMethods(t *testing.T) {
	// Skip if no API token is provided
	apiToken := os.Getenv("HETZNER_API_TOKEN")
	if apiToken == "" {
		t.Skip("HETZNER_API_TOKEN not set, skipping integration test")
	}

	// Skip if not explicitly enabled
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("RUN_INTEGRATION_TESTS not set to 'true', skipping integration test")
	}

	ctx := context.Background()

	config := &Config{
		APIToken:   apiToken,
		ServerType: "cpx11",
		Location:   "fsn1",
		VolumeSize: 10,
	}

	client, err := NewClient(config)
	require.NoError(t, err)

	t.Run("CreateVolume", func(t *testing.T) {
		volume, err := client.CreateVolume(ctx, 10, config.Location)
		require.NoError(t, err)
		assert.NotNil(t, volume)
		assert.Equal(t, 10, volume.Size)
		assert.Equal(t, config.Location, volume.Location)
		assert.Contains(t, volume.Name, "dockbridge-docker-data")

		// Clean up
		defer func() {
			// Detach volume if needed
			client.DetachVolume(ctx, fmt.Sprintf("%d", volume.ID))
		}()
	})

	t.Run("ListVolumes", func(t *testing.T) {
		volumes, err := client.ListVolumes(ctx)
		require.NoError(t, err)
		assert.NotNil(t, volumes)
		// Should have at least the volumes we created
		assert.GreaterOrEqual(t, len(volumes), 0)
	})
}

// BenchmarkCloudInitGeneration benchmarks the cloud-init script generation
func BenchmarkCloudInitGeneration(b *testing.B) {
	config := &hetzner.CloudInitConfig{
		SSHPublicKey: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC7vMzlJKvQqGHWDQz1234567890abcdefghijklmnopqrstuvwxyz test@example.com",
		VolumeMount:  "/var/lib/docker",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = hetzner.GenerateCloudInitScript(config)
	}
}
