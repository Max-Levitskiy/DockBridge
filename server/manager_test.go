package server

import (
	"context"
	"testing"
	"time"

	"github.com/dockbridge/dockbridge/client/hetzner"
	"github.com/dockbridge/dockbridge/shared/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockHetznerClient is a mock implementation of the HetznerClient interface
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

func TestManager_EnsureServer(t *testing.T) {
	tests := []struct {
		name            string
		existingServer  *hetzner.Server
		expectNewServer bool
		expectError     bool
	}{
		{
			name: "existing server found",
			existingServer: &hetzner.Server{
				ID:        123,
				Name:      "dockbridge-existing",
				Status:    "running",
				IPAddress: "1.2.3.4",
				VolumeID:  "456",
				CreatedAt: time.Now(),
			},
			expectNewServer: false,
			expectError:     false,
		},
		{
			name:            "no existing server, provision new one",
			existingServer:  nil,
			expectNewServer: true,
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockHetznerClient{}
			config := &config.HetznerConfig{
				ServerType: "cpx21",
				Location:   "fsn1",
				VolumeSize: 10,
			}

			manager := NewManager(mockClient, config)

			// Setup mock expectations
			if tt.existingServer != nil {
				mockClient.On("ListServers", mock.Anything).Return([]*hetzner.Server{tt.existingServer}, nil)
				mockClient.On("GetVolume", mock.Anything, "456").Return(&hetzner.Volume{
					ID:       456,
					Name:     "test-volume",
					Size:     10,
					Location: "fsn1",
					Status:   "attached",
				}, nil)
			} else {
				mockClient.On("ListServers", mock.Anything).Return([]*hetzner.Server{}, nil)
				mockClient.On("FindOrCreateDockerVolume", mock.Anything, "fsn1").Return(&hetzner.Volume{
					ID:       789,
					Name:     "dockbridge-docker-data-123",
					Size:     10,
					Location: "fsn1",
					Status:   "available",
				}, nil)
				mockClient.On("ProvisionServer", mock.Anything, mock.AnythingOfType("*hetzner.ServerConfig")).Return(&hetzner.Server{
					ID:        999,
					Name:      "dockbridge-new",
					Status:    "running",
					IPAddress: "5.6.7.8",
					VolumeID:  "789",
					CreatedAt: time.Now(),
				}, nil)
				mockClient.On("GetServer", mock.Anything, "999").Return(&hetzner.Server{
					ID:        999,
					Name:      "dockbridge-new",
					Status:    "running",
					IPAddress: "5.6.7.8",
					VolumeID:  "789",
					CreatedAt: time.Now(),
				}, nil)
			}

			// Execute
			server, err := manager.EnsureServer(context.Background())

			// Assert
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, server)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, server)
				assert.NotEmpty(t, server.VolumeID)
				assert.Equal(t, StatusRunning, server.Status)

				// Verify Docker data directory is set in metadata
				assert.Equal(t, "/var/lib/docker", server.Metadata["docker_data_dir"])
			}

			mockClient.AssertExpectations(t)
		})
	}
}

func TestManager_EnsureVolume(t *testing.T) {
	mockClient := &MockHetznerClient{}
	config := &config.HetznerConfig{
		Location:   "fsn1",
		VolumeSize: 10,
	}

	manager := NewManager(mockClient, config)

	// Setup mock expectations
	expectedVolume := &hetzner.Volume{
		ID:       123,
		Name:     "dockbridge-docker-data-456",
		Size:     10,
		Location: "fsn1",
		Status:   "available",
	}

	mockClient.On("FindOrCreateDockerVolume", mock.Anything, "fsn1").Return(expectedVolume, nil)

	// Execute
	volume, err := manager.EnsureVolume(context.Background())

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, volume)
	assert.Equal(t, "123", volume.ID)
	assert.Equal(t, "dockbridge-docker-data-456", volume.Name)
	assert.Equal(t, 10, volume.Size)
	assert.Equal(t, "/var/lib/docker", volume.MountPath)
	assert.Equal(t, VolumeStatusAvailable, volume.Status)

	mockClient.AssertExpectations(t)
}

func TestManager_DestroyServer(t *testing.T) {
	mockClient := &MockHetznerClient{}
	config := &config.HetznerConfig{}

	manager := NewManager(mockClient, config)

	// Setup mock expectations
	serverID := "123"
	volumeID := "456"

	mockClient.On("GetServer", mock.Anything, serverID).Return(&hetzner.Server{
		ID:        123,
		Name:      "dockbridge-test",
		Status:    "running",
		IPAddress: "1.2.3.4",
		VolumeID:  volumeID,
		CreatedAt: time.Now(),
	}, nil)

	mockClient.On("DetachVolume", mock.Anything, volumeID).Return(nil)
	mockClient.On("DestroyServer", mock.Anything, serverID).Return(nil)

	// Execute
	err := manager.DestroyServer(context.Background(), serverID)

	// Assert
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestManager_ListServers(t *testing.T) {
	mockClient := &MockHetznerClient{}
	config := &config.HetznerConfig{}

	manager := NewManager(mockClient, config)

	// Setup mock expectations
	hetznerServers := []*hetzner.Server{
		{
			ID:        123,
			Name:      "dockbridge-server1",
			Status:    "running",
			IPAddress: "1.2.3.4",
			VolumeID:  "456",
			CreatedAt: time.Now(),
		},
		{
			ID:        789,
			Name:      "other-server", // Should be filtered out
			Status:    "running",
			IPAddress: "5.6.7.8",
			VolumeID:  "999",
			CreatedAt: time.Now(),
		},
	}

	mockClient.On("ListServers", mock.Anything).Return(hetznerServers, nil)
	mockClient.On("GetVolume", mock.Anything, "456").Return(&hetzner.Volume{
		ID:       456,
		Name:     "test-volume",
		Size:     10,
		Location: "fsn1",
		Status:   "attached",
	}, nil)

	// Execute
	servers, err := manager.ListServers(context.Background())

	// Assert
	require.NoError(t, err)
	assert.Len(t, servers, 1) // Only DockBridge servers should be returned
	assert.Equal(t, "123", servers[0].ID)
	assert.Equal(t, "dockbridge-server1", servers[0].Name)
	assert.Equal(t, "/var/lib/docker", servers[0].Metadata["docker_data_dir"])

	mockClient.AssertExpectations(t)
}

func TestConvertToServerInfo(t *testing.T) {
	server := &hetzner.Server{
		ID:        123,
		Name:      "test-server",
		Status:    "running",
		IPAddress: "1.2.3.4",
		VolumeID:  "456",
		CreatedAt: time.Now(),
	}

	volume := &hetzner.Volume{
		ID:       456,
		Name:     "test-volume",
		Size:     20,
		Location: "fsn1",
		Status:   "attached",
	}

	serverInfo := convertToServerInfo(server, volume)

	assert.Equal(t, "123", serverInfo.ID)
	assert.Equal(t, "test-server", serverInfo.Name)
	assert.Equal(t, StatusRunning, serverInfo.Status)
	assert.Equal(t, "1.2.3.4", serverInfo.IPAddress)
	assert.Equal(t, "456", serverInfo.VolumeID)

	// Check metadata
	assert.Equal(t, "test-volume", serverInfo.Metadata["volume_name"])
	assert.Equal(t, "20", serverInfo.Metadata["volume_size"])
	assert.Equal(t, "fsn1", serverInfo.Metadata["volume_location"])
	assert.Equal(t, "/var/lib/docker", serverInfo.Metadata["docker_data_dir"])
}

func TestConvertToVolumeInfo(t *testing.T) {
	volume := &hetzner.Volume{
		ID:       123,
		Name:     "test-volume",
		Size:     15,
		Location: "fsn1",
		Status:   "available",
	}

	volumeInfo := convertToVolumeInfo(volume)

	assert.Equal(t, "123", volumeInfo.ID)
	assert.Equal(t, "test-volume", volumeInfo.Name)
	assert.Equal(t, 15, volumeInfo.Size)
	assert.Equal(t, VolumeStatusAvailable, volumeInfo.Status)
	assert.Equal(t, "/var/lib/docker", volumeInfo.MountPath)
}

// Integration test for Docker state persistence (requires real Hetzner API token)
func TestDockerStatePersistence_Integration(t *testing.T) {
	// Skip if no API token is provided
	apiToken := getTestAPIToken()
	if apiToken == "" {
		t.Skip("Skipping integration test: HETZNER_API_TOKEN not set")
	}

	// This test would:
	// 1. Create a server with volume
	// 2. Run some Docker commands to create state
	// 3. Destroy the server (preserving volume)
	// 4. Create a new server with the same volume
	// 5. Verify Docker state is preserved

	t.Log("Integration test for Docker state persistence would go here")
	t.Log("This requires careful implementation to avoid costs and cleanup")
}

func getTestAPIToken() string {
	// In a real test, this would read from environment variable
	// For now, return empty to skip integration tests
	return ""
}

// Benchmark tests
func BenchmarkManager_EnsureServer(b *testing.B) {
	mockClient := &MockHetznerClient{}
	config := &config.HetznerConfig{
		ServerType: "cpx21",
		Location:   "fsn1",
		VolumeSize: 10,
	}

	manager := NewManager(mockClient, config)

	// Setup mock for existing server scenario
	existingServer := &hetzner.Server{
		ID:        123,
		Name:      "dockbridge-existing",
		Status:    "running",
		IPAddress: "1.2.3.4",
		VolumeID:  "456",
		CreatedAt: time.Now(),
	}

	mockClient.On("ListServers", mock.Anything).Return([]*hetzner.Server{existingServer}, nil)
	mockClient.On("GetVolume", mock.Anything, "456").Return(&hetzner.Volume{
		ID:       456,
		Name:     "test-volume",
		Size:     10,
		Location: "fsn1",
		Status:   "attached",
	}, nil)

	for b.Loop() {
		_, err := manager.EnsureServer(context.Background())
		if err != nil {
			b.Fatal(err)
		}
	}
}
