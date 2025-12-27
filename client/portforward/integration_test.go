package portforward

import (
	"testing"
	"time"

	"github.com/dockbridge/dockbridge/client/monitor"
	"github.com/dockbridge/dockbridge/shared/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestContainerMonitorPortForwardIntegration tests the integration between
// ContainerMonitor and PortForwardManager using direct event calls
func TestContainerMonitorPortForwardIntegration(t *testing.T) {
	mockLogger := createTestLogger()

	// Create port forward manager
	pfConfig := &config.PortForwardConfig{
		Enabled:          true,
		ConflictStrategy: config.ConflictStrategyIncrement,
		MonitorInterval:  30 * time.Second,
	}
	pfManager := NewPortForwardManager(pfConfig, mockLogger)

	// Start port forward manager
	ctx := t.Context()

	err := pfManager.Start(ctx)
	require.NoError(t, err)
	defer pfManager.Stop()

	// Create test container info
	container := &monitor.ContainerInfo{
		ID:     "nginx-123",
		Name:   "nginx",
		Image:  "nginx:latest",
		Status: "running",
		Ports: []monitor.PortMapping{
			{ContainerPort: 80, HostPort: 8080, Protocol: "tcp", HostIP: "0.0.0.0"},
			{ContainerPort: 443, HostPort: 8443, Protocol: "tcp", HostIP: "0.0.0.0"},
		},
		Labels:  map[string]string{"app": "test"},
		Created: time.Now(),
	}

	// Test container created event
	err = pfManager.OnContainerCreated(container)
	require.NoError(t, err)

	// Verify port forwards were created
	forwards, err := pfManager.ListPortForwards()
	require.NoError(t, err)
	assert.Len(t, forwards, 2)

	// Verify port forward details
	portMap := make(map[int]*PortForward)
	for _, forward := range forwards {
		portMap[forward.RemotePort] = forward
	}

	// Check HTTP port forward
	httpForward := portMap[80]
	require.NotNil(t, httpForward)
	assert.Equal(t, "nginx-123", httpForward.ContainerID)
	assert.Equal(t, "nginx", httpForward.ContainerName)
	assert.Equal(t, 8080, httpForward.LocalPort)
	assert.Equal(t, 80, httpForward.RemotePort)
	assert.Equal(t, ForwardStatusActive, httpForward.Status)

	// Check HTTPS port forward
	httpsForward := portMap[443]
	require.NotNil(t, httpsForward)
	assert.Equal(t, "nginx-123", httpsForward.ContainerID)
	assert.Equal(t, "nginx", httpsForward.ContainerName)
	assert.Equal(t, 8443, httpsForward.LocalPort)
	assert.Equal(t, 443, httpsForward.RemotePort)
	assert.Equal(t, ForwardStatusActive, httpsForward.Status)

	// Test container stopped event
	err = pfManager.OnContainerStopped(container.ID)
	require.NoError(t, err)

	// Verify port forwards were cleaned up
	forwards, err = pfManager.ListPortForwards()
	require.NoError(t, err)
	assert.Len(t, forwards, 0)
}

// TestContainerMonitorPortForwardRemoval tests container removal handling
func TestContainerMonitorPortForwardRemoval(t *testing.T) {
	mockLogger := createTestLogger()

	// Create port forward manager
	pfConfig := &config.PortForwardConfig{
		Enabled:          true,
		ConflictStrategy: config.ConflictStrategyIncrement,
		MonitorInterval:  30 * time.Second,
	}
	pfManager := NewPortForwardManager(pfConfig, mockLogger)

	// Start port forward manager
	ctx := t.Context()

	err := pfManager.Start(ctx)
	require.NoError(t, err)
	defer pfManager.Stop()

	// Create test container info
	container := &monitor.ContainerInfo{
		ID:     "redis-456",
		Name:   "redis",
		Image:  "redis:latest",
		Status: "running",
		Ports: []monitor.PortMapping{
			{ContainerPort: 6379, HostPort: 6379, Protocol: "tcp", HostIP: "0.0.0.0"},
		},
		Labels:  map[string]string{"app": "test"},
		Created: time.Now(),
	}

	// Test container created event
	err = pfManager.OnContainerCreated(container)
	require.NoError(t, err)

	// Verify port forward was created
	forwards, err := pfManager.ListPortForwards()
	require.NoError(t, err)
	assert.Len(t, forwards, 1)
	assert.Equal(t, "redis-456", forwards[0].ContainerID)
	assert.Equal(t, 6379, forwards[0].LocalPort)

	// Test container removed event
	err = pfManager.OnContainerRemoved(container.ID)
	require.NoError(t, err)

	// Verify port forwards were cleaned up
	forwards, err = pfManager.ListPortForwards()
	require.NoError(t, err)
	assert.Len(t, forwards, 0)
}

// TestContainerMonitorMultipleContainers tests handling multiple containers
func TestContainerMonitorMultipleContainers(t *testing.T) {
	mockLogger := createTestLogger()

	// Create port forward manager
	pfConfig := &config.PortForwardConfig{
		Enabled:          true,
		ConflictStrategy: config.ConflictStrategyIncrement,
		MonitorInterval:  30 * time.Second,
	}
	pfManager := NewPortForwardManager(pfConfig, mockLogger)

	// Start port forward manager
	ctx := t.Context()

	err := pfManager.Start(ctx)
	require.NoError(t, err)
	defer pfManager.Stop()

	// Create multiple test containers
	containers := []*monitor.ContainerInfo{
		{
			ID:     "nginx-123",
			Name:   "nginx",
			Image:  "nginx:latest",
			Status: "running",
			Ports: []monitor.PortMapping{
				{ContainerPort: 80, HostPort: 8080, Protocol: "tcp", HostIP: "0.0.0.0"},
			},
			Labels:  map[string]string{"app": "web"},
			Created: time.Now(),
		},
		{
			ID:     "redis-456",
			Name:   "redis",
			Image:  "redis:latest",
			Status: "running",
			Ports: []monitor.PortMapping{
				{ContainerPort: 6379, HostPort: 6379, Protocol: "tcp", HostIP: "0.0.0.0"},
			},
			Labels:  map[string]string{"app": "cache"},
			Created: time.Now(),
		},
		{
			ID:     "postgres-789",
			Name:   "postgres",
			Image:  "postgres:latest",
			Status: "running",
			Ports: []monitor.PortMapping{
				{ContainerPort: 5432, HostPort: 5432, Protocol: "tcp", HostIP: "0.0.0.0"},
			},
			Labels:  map[string]string{"app": "db"},
			Created: time.Now(),
		},
	}

	// Create port forwards for all containers
	for _, container := range containers {
		err = pfManager.OnContainerCreated(container)
		require.NoError(t, err)
	}

	// Verify port forwards were created for all containers
	forwards, err := pfManager.ListPortForwards()
	require.NoError(t, err)
	assert.Len(t, forwards, 3)

	// Verify each container has its port forward
	containerMap := make(map[string]*PortForward)
	for _, forward := range forwards {
		containerMap[forward.ContainerID] = forward
	}

	// Check nginx forward
	nginxForward := containerMap["nginx-123"]
	require.NotNil(t, nginxForward)
	assert.Equal(t, "nginx", nginxForward.ContainerName)
	assert.Equal(t, 8080, nginxForward.LocalPort)
	assert.Equal(t, 80, nginxForward.RemotePort)

	// Check redis forward
	redisForward := containerMap["redis-456"]
	require.NotNil(t, redisForward)
	assert.Equal(t, "redis", redisForward.ContainerName)
	assert.Equal(t, 6379, redisForward.LocalPort)
	assert.Equal(t, 6379, redisForward.RemotePort)

	// Check postgres forward
	postgresForward := containerMap["postgres-789"]
	require.NotNil(t, postgresForward)
	assert.Equal(t, "postgres", postgresForward.ContainerName)
	assert.Equal(t, 5432, postgresForward.LocalPort)
	assert.Equal(t, 5432, postgresForward.RemotePort)

	// Stop one container (redis)
	err = pfManager.OnContainerStopped("redis-456")
	require.NoError(t, err)

	// Verify only redis port forward was cleaned up
	forwards, err = pfManager.ListPortForwards()
	require.NoError(t, err)
	assert.Len(t, forwards, 2)

	// Verify remaining forwards
	containerMap = make(map[string]*PortForward)
	for _, forward := range forwards {
		containerMap[forward.ContainerID] = forward
	}

	assert.NotNil(t, containerMap["nginx-123"])
	assert.NotNil(t, containerMap["postgres-789"])
	assert.Nil(t, containerMap["redis-456"]) // Should be cleaned up
}
