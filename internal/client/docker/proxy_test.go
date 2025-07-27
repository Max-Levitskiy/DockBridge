package docker

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/dockbridge/dockbridge/internal/client/hetzner"
	"github.com/dockbridge/dockbridge/internal/shared/config"
	"github.com/dockbridge/dockbridge/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

// MockHetznerClient is a mock implementation of HetznerClient
type MockHetznerClient struct {
	mock.Mock
}

func (m *MockHetznerClient) ProvisionServer(ctx context.Context, config *hetzner.ServerConfig) (*hetzner.Server, error) {
	args := m.Called(ctx, config)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*hetzner.Server), args.Error(1)
}

func (m *MockHetznerClient) DestroyServer(ctx context.Context, serverID string) error {
	args := m.Called(ctx, serverID)
	return args.Error(0)
}

func (m *MockHetznerClient) CreateVolume(ctx context.Context, size int, location string) (*hetzner.Volume, error) {
	args := m.Called(ctx, size, location)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
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
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*hetzner.SSHKey), args.Error(1)
}

func (m *MockHetznerClient) GetServer(ctx context.Context, serverID string) (*hetzner.Server, error) {
	args := m.Called(ctx, serverID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*hetzner.Server), args.Error(1)
}

func (m *MockHetznerClient) ListServers(ctx context.Context) ([]*hetzner.Server, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*hetzner.Server), args.Error(1)
}

func (m *MockHetznerClient) GetVolume(ctx context.Context, volumeID string) (*hetzner.Volume, error) {
	args := m.Called(ctx, volumeID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*hetzner.Volume), args.Error(1)
}

func (m *MockHetznerClient) ListVolumes(ctx context.Context) ([]*hetzner.Volume, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*hetzner.Volume), args.Error(1)
}

// MockLogger is a mock implementation of logger.LoggerInterface
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Debug(msg string, args ...interface{}) {
	m.Called(msg, args)
}

func (m *MockLogger) Info(msg string, args ...interface{}) {
	m.Called(msg, args)
}

func (m *MockLogger) Warn(msg string, args ...interface{}) {
	m.Called(msg, args)
}

func (m *MockLogger) Error(msg string, args ...interface{}) {
	m.Called(msg, args)
}

func (m *MockLogger) Fatal(msg string, args ...interface{}) {
	m.Called(msg, args)
}

func (m *MockLogger) WithFields(fields map[string]interface{}) *logger.Logger {
	args := m.Called(fields)
	if args.Get(0) == nil {
		return logger.NewDefault().WithFields(fields)
	}
	return args.Get(0).(*logger.Logger)
}

// DockerProxyTestSuite defines the test suite for DockerProxy
type DockerProxyTestSuite struct {
	suite.Suite
	proxy       DockerProxy
	mockHetzner *MockHetznerClient
	mockLogger  *MockLogger
	config      *ProxyConfig
	testServer  *httptest.Server
}

// SetupTest initializes the test suite
func (suite *DockerProxyTestSuite) SetupTest() {
	suite.mockHetzner = &MockHetznerClient{}
	suite.mockLogger = &MockLogger{}

	// Create test HTTP server to simulate Docker daemon
	suite.testServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/version":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"Version":"20.10.0","ApiVersion":"1.41"}`))
		case "/containers/json":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`[]`))
		case "/images/json":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`[]`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	// Use a unique socket path for each test to avoid conflicts
	socketPath := fmt.Sprintf("/tmp/docker-test-%d.sock", time.Now().UnixNano())

	suite.config = &ProxyConfig{
		SocketPath:    socketPath,
		ProxyPort:     2376,
		HetznerClient: suite.mockHetzner,
		SSHConfig: &config.SSHConfig{
			KeyPath: "/tmp/test_key",
			Port:    22,
			Timeout: 30 * time.Second,
		},
		HetznerConfig: &config.HetznerConfig{
			ServerType: "cpx21",
			Location:   "fsn1",
			VolumeSize: 10,
		},
		Logger: suite.mockLogger,
	}

	suite.proxy = NewDockerProxy()

	// Setup default mock expectations
	suite.mockLogger.On("Info", mock.AnythingOfType("string"), mock.Anything).Maybe()
	suite.mockLogger.On("Debug", mock.AnythingOfType("string"), mock.Anything).Maybe()
	suite.mockLogger.On("Error", mock.AnythingOfType("string"), mock.Anything, mock.Anything).Maybe()
	suite.mockLogger.On("WithFields", mock.AnythingOfType("map[string]interface {}")).Return(logger.NewDefault()).Maybe()
}

// TearDownTest cleans up after each test
func (suite *DockerProxyTestSuite) TearDownTest() {
	if suite.testServer != nil {
		suite.testServer.Close()
	}
	if suite.proxy.IsRunning() {
		suite.proxy.Stop()
	}
	// Clean up socket file if it exists
	if suite.config != nil && suite.config.SocketPath != "" {
		os.Remove(suite.config.SocketPath)
	}
}

// TestNewDockerProxy tests proxy creation
func (suite *DockerProxyTestSuite) TestNewDockerProxy() {
	proxy := NewDockerProxy()
	suite.NotNil(proxy)
	suite.False(proxy.IsRunning())
}

// TestStartProxy tests starting the proxy
func (suite *DockerProxyTestSuite) TestStartProxy() {
	ctx := context.Background()

	err := suite.proxy.Start(ctx, suite.config)
	suite.NoError(err)
	suite.True(suite.proxy.IsRunning())

	// Clean up
	suite.proxy.Stop()
}

// TestStopProxy tests stopping the proxy
func (suite *DockerProxyTestSuite) TestStopProxy() {
	ctx := context.Background()

	// Start proxy first
	err := suite.proxy.Start(ctx, suite.config)
	suite.NoError(err)

	// Stop proxy
	err = suite.proxy.Stop()
	suite.NoError(err)
	suite.False(suite.proxy.IsRunning())
}

// TestProxyConfigValidation tests configuration validation
func (suite *DockerProxyTestSuite) TestProxyConfigValidation() {
	ctx := context.Background()

	// Test with nil config
	err := suite.proxy.Start(ctx, nil)
	suite.Error(err)
	suite.Contains(err.Error(), "proxy config cannot be nil")

	// Test with missing logger
	invalidConfig := &ProxyConfig{
		SocketPath:    "/tmp/test.sock",
		HetznerClient: suite.mockHetzner,
		SSHConfig:     suite.config.SSHConfig,
		HetznerConfig: suite.config.HetznerConfig,
		// Logger is nil
	}
	err = suite.proxy.Start(ctx, invalidConfig)
	suite.Error(err)
	suite.Contains(err.Error(), "logger cannot be nil")
}

// TestDoubleStart tests starting proxy twice
func (suite *DockerProxyTestSuite) TestDoubleStart() {
	ctx := context.Background()

	// First start should succeed
	err := suite.proxy.Start(ctx, suite.config)
	suite.NoError(err)
	suite.True(suite.proxy.IsRunning())

	// Second start should return error
	err = suite.proxy.Start(ctx, suite.config)
	suite.Error(err)
	suite.Contains(err.Error(), "already running")

	// Clean up
	suite.proxy.Stop()
}

// Run the test suite
func TestDockerProxyTestSuite(t *testing.T) {
	suite.Run(t, new(DockerProxyTestSuite))
}

// Additional unit tests

func TestNewDockerProxy(t *testing.T) {
	proxy := NewDockerProxy()
	assert.NotNil(t, proxy)
	assert.False(t, proxy.IsRunning())
}

func TestProxyConfigValidation(t *testing.T) {
	proxy := NewDockerProxy()
	ctx := context.Background()

	// Test with nil config
	err := proxy.Start(ctx, nil)
	assert.Error(t, err)

	// Test with invalid config
	invalidConfig := &ProxyConfig{}
	err = proxy.Start(ctx, invalidConfig)
	assert.Error(t, err)
}

func TestDoubleStart(t *testing.T) {
	proxy := NewDockerProxy()
	ctx := context.Background()

	mockHetzner := &MockHetznerClient{}
	mockLogger := &MockLogger{}

	// Use a unique socket path to avoid conflicts
	socketPath := fmt.Sprintf("/tmp/docker-test-%d.sock", time.Now().UnixNano())

	config := &ProxyConfig{
		SocketPath:    socketPath,
		ProxyPort:     2376,
		HetznerClient: mockHetzner,
		SSHConfig: &config.SSHConfig{
			KeyPath: "/tmp/test_key",
			Port:    22,
			Timeout: 30 * time.Second,
		},
		HetznerConfig: &config.HetznerConfig{
			ServerType: "cpx21",
			Location:   "fsn1",
			VolumeSize: 10,
		},
		Logger: mockLogger,
	}

	// Setup mocks
	mockLogger.On("Info", mock.AnythingOfType("string"), mock.Anything).Maybe()
	mockLogger.On("Debug", mock.AnythingOfType("string"), mock.Anything).Maybe()
	mockLogger.On("Error", mock.AnythingOfType("string"), mock.Anything, mock.Anything).Maybe()
	mockLogger.On("WithFields", mock.AnythingOfType("map[string]interface {}")).Return(logger.NewDefault()).Maybe()

	// First start should succeed
	err := proxy.Start(ctx, config)
	assert.NoError(t, err)
	assert.True(t, proxy.IsRunning())

	// Second start should return error
	err = proxy.Start(ctx, config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already running")

	// Clean up
	proxy.Stop()
	os.Remove(socketPath)
}
