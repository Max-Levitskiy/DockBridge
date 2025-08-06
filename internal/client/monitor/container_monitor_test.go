package monitor

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/dockbridge/dockbridge/pkg/logger"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockDockerClient is a mock implementation of ContainerAPIClient for testing
type MockDockerClient struct {
	mock.Mock
}

func (m *MockDockerClient) ContainerList(ctx context.Context, options container.ListOptions) ([]types.Container, error) {
	args := m.Called(ctx, options)
	return args.Get(0).([]types.Container), args.Error(1)
}

func (m *MockDockerClient) ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error) {
	args := m.Called(ctx, containerID)
	return args.Get(0).(types.ContainerJSON), args.Error(1)
}

// createTestLogger creates a simple test logger that discards output
func createTestLogger() logger.LoggerInterface {
	testLogger := logger.NewDefault()
	testLogger.SetOutput(io.Discard) // Discard log output during tests
	return testLogger
}

// MockEventHandler is a mock implementation of ContainerEventHandler for testing
type MockEventHandler struct {
	mock.Mock
}

func (m *MockEventHandler) OnContainerCreated(container *ContainerInfo) error {
	args := m.Called(container)
	return args.Error(0)
}

func (m *MockEventHandler) OnContainerStopped(containerID string) error {
	args := m.Called(containerID)
	return args.Error(0)
}

func (m *MockEventHandler) OnContainerRemoved(containerID string) error {
	args := m.Called(containerID)
	return args.Error(0)
}

// Test helper functions
func createTestContainer(id, name, image string, ports []types.Port) types.Container {
	return types.Container{
		ID:      id,
		Names:   []string{"/" + name},
		Image:   image,
		Status:  "running",
		Ports:   ports,
		Labels:  map[string]string{"test": "true"},
		Created: time.Now().Unix(),
	}
}

func createTestContainerJSON(id, name, image string, portBindings nat.PortMap) types.ContainerJSON {
	return types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			ID:      id,
			Name:    "/" + name,
			Created: time.Now().Format(time.RFC3339Nano),
			State: &types.ContainerState{
				Status:  "running",
				Running: true,
			},
		},
		Config: &container.Config{
			Image:  image,
			Labels: map[string]string{"test": "true"},
		},
		NetworkSettings: &types.NetworkSettings{
			NetworkSettingsBase: types.NetworkSettingsBase{
				Ports: portBindings,
			},
		},
	}
}

func TestContainerMonitor_BasicLifecycle(t *testing.T) {
	mockClient := &MockDockerClient{}
	mockLogger := createTestLogger()

	monitor := NewContainerMonitor(mockClient, mockLogger)

	// Test initial state
	assert.False(t, monitor.(*containerMonitorImpl).running)

	// Mock initial container list (empty)
	mockClient.On("ContainerList", mock.Anything, mock.Anything).Return([]types.Container{}, nil)

	// Start monitor
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := monitor.Start(ctx)
	require.NoError(t, err)
	assert.True(t, monitor.(*containerMonitorImpl).running)

	// Try to start again (should fail)
	err = monitor.Start(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already running")

	// Stop monitor
	err = monitor.Stop()
	require.NoError(t, err)
	assert.False(t, monitor.(*containerMonitorImpl).running)

	// Stop again (should not error)
	err = monitor.Stop()
	assert.NoError(t, err)

	mockClient.AssertExpectations(t)
}

func TestContainerMonitor_EventHandlerRegistration(t *testing.T) {
	mockClient := &MockDockerClient{}
	mockLogger := createTestLogger()

	monitor := NewContainerMonitor(mockClient, mockLogger)

	// Register event handlers
	handler1 := &MockEventHandler{}
	handler2 := &MockEventHandler{}

	err := monitor.RegisterContainerEventHandler(handler1)
	assert.NoError(t, err)

	err = monitor.RegisterContainerEventHandler(handler2)
	assert.NoError(t, err)

	// Verify handlers are registered
	impl := monitor.(*containerMonitorImpl)
	assert.Len(t, impl.handlers, 2)
}

func TestContainerMonitor_ListRunningContainers(t *testing.T) {
	mockClient := &MockDockerClient{}
	mockLogger := createTestLogger()

	monitor := NewContainerMonitor(mockClient, mockLogger)

	// Create test containers
	testContainers := []types.Container{
		createTestContainer("container1", "nginx", "nginx:latest", []types.Port{
			{PrivatePort: 80, PublicPort: 8080, Type: "tcp", IP: "0.0.0.0"},
		}),
		createTestContainer("container2", "redis", "redis:latest", []types.Port{
			{PrivatePort: 6379, PublicPort: 6379, Type: "tcp", IP: "0.0.0.0"},
		}),
	}

	mockClient.On("ContainerList", mock.Anything, mock.MatchedBy(func(opts container.ListOptions) bool {
		return !opts.All // Should only list running containers
	})).Return(testContainers, nil)

	ctx := context.Background()
	containers, err := monitor.ListRunningContainers(ctx)

	require.NoError(t, err)
	assert.Len(t, containers, 2)

	// Verify first container
	assert.Equal(t, "container1", containers[0].ID)
	assert.Equal(t, "nginx", containers[0].Name)
	assert.Equal(t, "nginx:latest", containers[0].Image)
	assert.Equal(t, "running", containers[0].Status)
	assert.Len(t, containers[0].Ports, 1)
	assert.Equal(t, 80, containers[0].Ports[0].ContainerPort)
	assert.Equal(t, 8080, containers[0].Ports[0].HostPort)
	assert.Equal(t, "tcp", containers[0].Ports[0].Protocol)

	// Verify second container
	assert.Equal(t, "container2", containers[1].ID)
	assert.Equal(t, "redis", containers[1].Name)
	assert.Equal(t, "redis:latest", containers[1].Image)

	mockClient.AssertExpectations(t)
}

func TestContainerMonitor_GetContainer(t *testing.T) {
	mockClient := &MockDockerClient{}
	mockLogger := createTestLogger()

	monitor := NewContainerMonitor(mockClient, mockLogger)

	// Create test container JSON with port bindings
	portBindings := nat.PortMap{
		"80/tcp": []nat.PortBinding{
			{HostIP: "0.0.0.0", HostPort: "8080"},
		},
		"443/tcp": []nat.PortBinding{
			{HostIP: "0.0.0.0", HostPort: "8443"},
		},
	}

	testContainerJSON := createTestContainerJSON("container1", "nginx", "nginx:latest", portBindings)

	mockClient.On("ContainerInspect", mock.Anything, "container1").Return(testContainerJSON, nil)

	ctx := context.Background()
	container, err := monitor.GetContainer(ctx, "container1")

	require.NoError(t, err)
	assert.Equal(t, "container1", container.ID)
	assert.Equal(t, "nginx", container.Name)
	assert.Equal(t, "nginx:latest", container.Image)
	assert.Equal(t, "running", container.Status)
	assert.Len(t, container.Ports, 2)

	// Check port mappings
	portMap := make(map[int]int) // containerPort -> hostPort
	for _, port := range container.Ports {
		portMap[port.ContainerPort] = port.HostPort
	}
	assert.Equal(t, 8080, portMap[80])
	assert.Equal(t, 8443, portMap[443])

	mockClient.AssertExpectations(t)
}

func TestContainerMonitor_SetPollingInterval(t *testing.T) {
	mockClient := &MockDockerClient{}
	mockLogger := createTestLogger()

	monitor := NewContainerMonitor(mockClient, mockLogger)

	// Test setting polling interval
	newInterval := 10 * time.Second
	err := monitor.SetPollingInterval(newInterval)
	assert.NoError(t, err)

	impl := monitor.(*containerMonitorImpl)
	assert.Equal(t, newInterval, impl.pollingInterval)
}
