package lifecycle

import (
	"context"
	"testing"
	"time"

	"github.com/dockbridge/dockbridge/client/activity"
	"github.com/dockbridge/dockbridge/pkg/logger"
	"github.com/dockbridge/dockbridge/server"
	"github.com/dockbridge/dockbridge/shared/config"
)

// MockActivityTracker implements activity.ActivityTracker for testing
type MockActivityTracker struct {
	stopCalled bool
}

func (m *MockActivityTracker) Start(ctx context.Context) error { return nil }
func (m *MockActivityTracker) Stop() error {
	m.stopCalled = true
	return nil
}
func (m *MockActivityTracker) RecordDockerCommand() error      { return nil }
func (m *MockActivityTracker) RecordConnectionActivity() error { return nil }
func (m *MockActivityTracker) GetLastActivity() time.Time      { return time.Now() }
func (m *MockActivityTracker) GetLastConnection() time.Time    { return time.Now() }
func (m *MockActivityTracker) GetTimeUntilShutdown() (time.Duration, string) {
	return time.Hour, "none"
}
func (m *MockActivityTracker) RegisterCallback(callback activity.ActivityCallback) {}

// MockServerManager implements server.ServerManager for testing
type MockServerManager struct {
	servers             []*server.ServerInfo
	destroyServerCalled bool
	destroyedServerID   string
}

func (m *MockServerManager) EnsureServer(ctx context.Context) (*server.ServerInfo, error) {
	return nil, nil
}

func (m *MockServerManager) DestroyServer(ctx context.Context, serverID string) error {
	m.destroyServerCalled = true
	m.destroyedServerID = serverID
	return nil
}

func (m *MockServerManager) GetServerStatus(ctx context.Context) (*server.ServerStatus, error) {
	status := server.StatusRunning
	return &status, nil
}

func (m *MockServerManager) ListServers(ctx context.Context) ([]*server.ServerInfo, error) {
	return m.servers, nil
}

func (m *MockServerManager) EnsureVolume(ctx context.Context) (*server.VolumeInfo, error) {
	return nil, nil
}

func TestManager_Stop(t *testing.T) {
	// Setup mocks
	activityTracker := &MockActivityTracker{}
	serverManager := &MockServerManager{}

	// Setup logger (discard output for tests, or use default)
	log := logger.NewDefault()

	// Setup config
	cfg := &config.ActivityConfig{
		IdleTimeout:       time.Minute,
		ConnectionTimeout: time.Minute,
		GracePeriod:       time.Second,
	}

	// Create manager
	manager := NewManager(activityTracker, serverManager, cfg, log)

	// Simulate running server
	runningServer := &server.ServerInfo{
		ID:     "server-123",
		Name:   "dockbridge-test",
		Status: server.StatusRunning,
	}
	serverManager.servers = []*server.ServerInfo{runningServer}

	// Start manager (to initialize context)
	ctx := context.Background()
	_ = manager.Start(ctx)

	// Call Stop
	err := manager.Stop()
	if err != nil {
		t.Fatalf("Stop() returned error: %v", err)
	}

	// Verify Stop behavior
	if !activityTracker.stopCalled {
		t.Error("ActivityTracker.Stop() was not called")
	}

	if !serverManager.destroyServerCalled {
		t.Error("ServerManager.DestroyServer() was not called")
	}

	if serverManager.destroyedServerID != "server-123" {
		t.Errorf("Expected destroyed server ID to be 'server-123', got '%s'", serverManager.destroyedServerID)
	}
}

func TestManager_Stop_NoRunningServers(t *testing.T) {
	// Setup mocks
	activityTracker := &MockActivityTracker{}
	serverManager := &MockServerManager{}

	// Setup logger
	log := logger.NewDefault()

	// Setup config
	cfg := &config.ActivityConfig{}

	// Create manager
	manager := NewManager(activityTracker, serverManager, cfg, log)

	// No running servers
	serverManager.servers = []*server.ServerInfo{}

	// Start manager
	ctx := context.Background()
	_ = manager.Start(ctx)

	// Call Stop
	err := manager.Stop()
	if err != nil {
		t.Fatalf("Stop() returned error: %v", err)
	}

	// Verify DestroyServer was NOT called
	if serverManager.destroyServerCalled {
		t.Error("ServerManager.DestroyServer() should NOT have been called")
	}
}
