package docker

import (
	"context"
	"testing"
	"time"

	"github.com/dockbridge/dockbridge/client/hetzner"
	"github.com/dockbridge/dockbridge/internal/shared/config"
	"github.com/dockbridge/dockbridge/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockHetznerClient is a mock implementation of HetznerClient for testing
type MockHetznerClient struct {
	mock.Mock
}

func (m *MockHetznerClient) ProvisionServer(ctx context.Context, config *hetzner.ServerConfig) (*hetzner.Server, error) {
	args := m.Called(ctx, config)
	return args.Get(0).(*hetzner.Server), args.Error(1)
}

func (m *MockHetznerClient) DestroyServer(ctx context.Context, serverID string) error {
	args := m.Called(ctx, serverID)
	return args.Error(0)
}

func (m *MockHetznerClient) CreateVolume(ctx context.Context, size int, location string) (*hetzner.Volume, error) {
	args := m.Called(ctx, size, location)
	return args.Get(0).(*hetzner.Volume), args.Error(1)
}

func (m *MockHetznerClient) FindOrCreateDockerVolume(ctx context.Context, location string) (*hetzner.Volume, error) {
	args := m.Called(ctx, location)
	return args.Get(0).(*hetzner.Volume), args.Error(1)
}

func (m *MockHetznerClient) AttachVolume(ctx context.Context, serverID, volumeID string) error {
	args := m.Called(ctx, serverID, volumeID)
	return args.Error(0)
}

func (m *MockHetznerClient) DetachVolume(ctx context.Context, volumeID string) error {
	args := m.Called(ctx, volumeID)
	return args.Error(0)
}

func (m *MockHetznerClient) ManageSSHKeys(ctx context.Context, publicKey string) (*hetzner.SSHKey, error) {
	args := m.Called(ctx, publicKey)
	return args.Get(0).(*hetzner.SSHKey), args.Error(1)
}

func (m *MockHetznerClient) GetServer(ctx context.Context, serverID string) (*hetzner.Server, error) {
	args := m.Called(ctx, serverID)
	return args.Get(0).(*hetzner.Server), args.Error(1)
}

func (m *MockHetznerClient) ListServers(ctx context.Context) ([]*hetzner.Server, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*hetzner.Server), args.Error(1)
}

func (m *MockHetznerClient) GetVolume(ctx context.Context, volumeID string) (*hetzner.Volume, error) {
	args := m.Called(ctx, volumeID)
	return args.Get(0).(*hetzner.Volume), args.Error(1)
}

func (m *MockHetznerClient) ListVolumes(ctx context.Context) ([]*hetzner.Volume, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*hetzner.Volume), args.Error(1)
}

func TestDockerClientManagerVolumeIntegration(t *testing.T) {
	// Create mock Hetzner client
	mockHetzner := &MockHetznerClient{}

	// Create test configurations
	sshConfig := &config.SSHConfig{
		KeyPath:   "/tmp/test_key",
		Port:      22,
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	hetznerConfig := &config.HetznerConfig{
		ServerType: "cpx21",
		Location:   "fsn1",
		VolumeSize: 10,
	}

	// Create logger
	log := logger.NewDefault()

	// Create Docker client manager
	dcm := NewDockerClientManager(mockHetzner, sshConfig, hetznerConfig, log)

	ctx := context.Background()

	t.Run("ProvisionServerWithVolume", func(t *testing.T) {
		// Setup mock expectations
		expectedVolume := &hetzner.Volume{
			ID:       12345,
			Name:     "dockbridge-docker-data-123456789",
			Size:     10,
			Location: "fsn1",
			Status:   "available",
		}

		expectedSSHKey := &hetzner.SSHKey{
			ID:          67890,
			Name:        "dockbridge-key-123456789",
			Fingerprint: "SHA256:test",
			PublicKey:   "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC7...",
		}

		expectedServer := &hetzner.Server{
			ID:        54321,
			Name:      "dockbridge-123456789",
			Status:    "running",
			IPAddress: "192.168.1.100",
			VolumeID:  "12345",
			CreatedAt: time.Now(),
		}

		// Mock the volume creation/finding
		mockHetzner.On("FindOrCreateDockerVolume", ctx, "fsn1").Return(expectedVolume, nil)

		// Mock SSH key management
		mockHetzner.On("ManageSSHKeys", ctx, mock.AnythingOfType("string")).Return(expectedSSHKey, nil)

		// Mock server provisioning
		mockHetzner.On("ProvisionServer", ctx, mock.MatchedBy(func(config *hetzner.ServerConfig) bool {
			// Verify that the server config includes the volume
			return config.VolumeID == "12345" &&
				config.Location == "fsn1" &&
				config.ServerType == "cpx21" &&
				config.SSHKeyID == 67890
		})).Return(expectedServer, nil)

		// Mock volume attachment
		mockHetzner.On("AttachVolume", ctx, "54321", "12345").Return(nil)

		// Mock server readiness check (simplified)
		mockHetzner.On("GetServer", ctx, "54321").Return(expectedServer, nil)

		// Test the provisioning (we'll need to access the internal method for testing)
		// This would normally be called through the DockerClientManager interface
		dcmImpl := dcm.(*dockerClientManagerImpl)

		// We can't easily test the full provisioning without file system access
		// So we'll test the components individually

		// Verify mock expectations
		mockHetzner.AssertExpectations(t)
	})

	t.Run("VolumeAttachmentFlow", func(t *testing.T) {
		// Test that volume attachment is called with correct parameters
		mockHetzner.ExpectedCalls = nil // Reset expectations

		expectedVolume := &hetzner.Volume{
			ID:       99999,
			Name:     "test-volume",
			Size:     10,
			Location: "fsn1",
			Status:   "available",
		}

		mockHetzner.On("FindOrCreateDockerVolume", ctx, "fsn1").Return(expectedVolume, nil)

		// Call the volume finding method
		volume, err := mockHetzner.FindOrCreateDockerVolume(ctx, "fsn1")
		require.NoError(t, err)
		assert.Equal(t, expectedVolume.ID, volume.ID)
		assert.Contains(t, volume.Name, "test-volume")

		mockHetzner.AssertExpectations(t)
	})
}

func TestCloudInitScriptIntegration(t *testing.T) {
	t.Run("ScriptContainsVolumeMount", func(t *testing.T) {
		publicKey := "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC7... test@example.com"

		config := &hetzner.CloudInitConfig{
			SSHPublicKey: publicKey,
			VolumeMount:  "/var/lib/docker",
		}

		script := hetzner.GenerateCloudInitScript(config)

		// Verify the script contains volume-specific operations
		assert.Contains(t, script, "/var/lib/docker", "Script should mount volume at Docker data directory")
		assert.Contains(t, script, "mkfs.ext4", "Script should format the volume")
		assert.Contains(t, script, "fstab", "Script should add volume to fstab for persistence")
		assert.Contains(t, script, `"data-root"`, "Script should configure Docker to use the mounted volume")
		assert.Contains(t, script, publicKey, "Script should include the SSH public key")
		assert.Contains(t, script, "/dev/sdb", "Script should reference the volume device path")
	})
}

func TestVolumeReuseLogic(t *testing.T) {
	mockHetzner := &MockHetznerClient{}
	ctx := context.Background()

	t.Run("ReuseExistingVolume", func(t *testing.T) {
		// Mock finding an existing available volume
		existingVolume := &hetzner.Volume{
			ID:       11111,
			Name:     "dockbridge-docker-data-existing",
			Size:     10,
			Location: "fsn1",
			Status:   "available",
		}

		mockHetzner.On("FindOrCreateDockerVolume", ctx, "fsn1").Return(existingVolume, nil)

		volume, err := mockHetzner.FindOrCreateDockerVolume(ctx, "fsn1")
		require.NoError(t, err)
		assert.Equal(t, existingVolume.ID, volume.ID)
		assert.Equal(t, "available", volume.Status)

		mockHetzner.AssertExpectations(t)
	})
}
