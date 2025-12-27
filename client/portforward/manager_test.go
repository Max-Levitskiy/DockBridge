package portforward

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/dockbridge/dockbridge/client/monitor"
	"github.com/dockbridge/dockbridge/pkg/logger"
	"github.com/dockbridge/dockbridge/shared/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestLogger() logger.LoggerInterface {
	testLogger := logger.NewDefault()
	testLogger.SetOutput(io.Discard) // Discard log output during tests
	return testLogger
}

func TestPortForwardManager_BasicLifecycle(t *testing.T) {
	// Create test configuration
	cfg := &config.PortForwardConfig{
		Enabled:          true,
		ConflictStrategy: config.ConflictStrategyIncrement,
		MonitorInterval:  30 * time.Second,
	}

	// Create manager
	manager := NewPortForwardManager(cfg, createTestLogger())

	// Test start
	ctx := context.Background()
	err := manager.Start(ctx)
	require.NoError(t, err)

	// Test that we can list forwards (should be empty initially)
	forwards, err := manager.ListPortForwards()
	require.NoError(t, err)
	assert.Empty(t, forwards)

	// Test stop
	err = manager.Stop()
	require.NoError(t, err)
}

func TestPortForwardManager_ContainerEvents(t *testing.T) {
	// Create test configuration
	cfg := &config.PortForwardConfig{
		Enabled:          true,
		ConflictStrategy: config.ConflictStrategyIncrement,
		MonitorInterval:  30 * time.Second,
	}

	// Create manager
	manager := NewPortForwardManager(cfg, createTestLogger())

	// Start manager
	ctx := context.Background()
	err := manager.Start(ctx)
	require.NoError(t, err)
	defer manager.Stop()

	// Create test container info
	container := &monitor.ContainerInfo{
		ID:     "test-container-123",
		Name:   "test-nginx",
		Image:  "nginx:latest",
		Status: "running",
		Ports: []monitor.PortMapping{
			{
				ContainerPort: 80,
				HostPort:      8080,
				Protocol:      "tcp",
				HostIP:        "0.0.0.0",
			},
		},
		Labels:  map[string]string{"app": "test"},
		Created: time.Now(),
	}

	// Test container created event
	err = manager.OnContainerCreated(container)
	require.NoError(t, err)

	// Verify port forward was created
	forwards, err := manager.ListPortForwards()
	require.NoError(t, err)
	assert.Len(t, forwards, 1)

	forward := forwards[0]
	assert.Equal(t, container.ID, forward.ContainerID)
	assert.Equal(t, container.Name, forward.ContainerName)
	assert.Equal(t, 8080, forward.LocalPort)
	assert.Equal(t, 80, forward.RemotePort)
	assert.Equal(t, ForwardStatusActive, forward.Status)

	// Test getting specific port forward
	retrievedForward, err := manager.GetPortForward(container.ID, 80)
	require.NoError(t, err)
	assert.Equal(t, forward.ID, retrievedForward.ID)

	// Test container stopped event
	err = manager.OnContainerStopped(container.ID)
	require.NoError(t, err)

	// Verify port forward was removed
	forwards, err = manager.ListPortForwards()
	require.NoError(t, err)
	assert.Empty(t, forwards)
}

func TestPortForwardManager_ManualPortManagement(t *testing.T) {
	// Create test configuration
	cfg := &config.PortForwardConfig{
		Enabled:          true,
		ConflictStrategy: config.ConflictStrategyIncrement,
		MonitorInterval:  30 * time.Second,
	}

	// Create manager
	manager := NewPortForwardManager(cfg, createTestLogger())

	// Start manager
	ctx := context.Background()
	err := manager.Start(ctx)
	require.NoError(t, err)
	defer manager.Stop()

	// Create test container info first
	container := &monitor.ContainerInfo{
		ID:      "test-container-456",
		Name:    "test-app",
		Image:   "app:latest",
		Status:  "running",
		Ports:   []monitor.PortMapping{}, // No initial ports
		Labels:  map[string]string{"app": "test"},
		Created: time.Now(),
	}

	// Add container to manager
	err = manager.OnContainerCreated(container)
	require.NoError(t, err)

	// Test manual port forward addition
	err = manager.AddPortForward(container.ID, 3000, 3000)
	require.NoError(t, err)

	// Verify port forward was created
	forwards, err := manager.ListPortForwards()
	require.NoError(t, err)
	assert.Len(t, forwards, 1)

	forward := forwards[0]
	assert.Equal(t, container.ID, forward.ContainerID)
	assert.Equal(t, 3000, forward.LocalPort)
	assert.Equal(t, 3000, forward.RemotePort)

	// Test manual port forward removal
	err = manager.RemovePortForward(container.ID, 3000)
	require.NoError(t, err)

	// Verify port forward was removed
	forwards, err = manager.ListPortForwards()
	require.NoError(t, err)
	assert.Empty(t, forwards)
}

func TestPortForwardManager_DisabledConfiguration(t *testing.T) {
	// Create disabled configuration
	cfg := &config.PortForwardConfig{
		Enabled:          false,
		ConflictStrategy: config.ConflictStrategyIncrement,
		MonitorInterval:  30 * time.Second,
	}

	// Create manager
	manager := NewPortForwardManager(cfg, createTestLogger())

	// Start manager (should succeed but do nothing)
	ctx := context.Background()
	err := manager.Start(ctx)
	require.NoError(t, err)
	defer manager.Stop()

	// Create test container info
	container := &monitor.ContainerInfo{
		ID:     "test-container-789",
		Name:   "test-disabled",
		Image:  "nginx:latest",
		Status: "running",
		Ports: []monitor.PortMapping{
			{
				ContainerPort: 80,
				HostPort:      8080,
				Protocol:      "tcp",
				HostIP:        "0.0.0.0",
			},
		},
		Labels:  map[string]string{"app": "test"},
		Created: time.Now(),
	}

	// Test container created event (should be ignored)
	err = manager.OnContainerCreated(container)
	require.NoError(t, err)

	// Verify no port forwards were created
	forwards, err := manager.ListPortForwards()
	require.NoError(t, err)
	assert.Empty(t, forwards)
}

func TestPortForwardManager_ConfigurationUpdate(t *testing.T) {
	// Create initial configuration
	cfg := &config.PortForwardConfig{
		Enabled:          true,
		ConflictStrategy: config.ConflictStrategyIncrement,
		MonitorInterval:  30 * time.Second,
	}

	// Create manager
	manager := NewPortForwardManager(cfg, createTestLogger())

	// Start manager
	ctx := context.Background()
	err := manager.Start(ctx)
	require.NoError(t, err)
	defer manager.Stop()

	// Update configuration
	newCfg := &config.PortForwardConfig{
		Enabled:          true,
		ConflictStrategy: config.ConflictStrategyFail,
		MonitorInterval:  60 * time.Second,
	}

	err = manager.SetConfig(newCfg)
	require.NoError(t, err)

	// Configuration update should succeed
	// (Actual behavior changes would be tested in integration tests)
}
