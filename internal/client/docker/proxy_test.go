package docker

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dockbridge/dockbridge/internal/client/hetzner"
	"github.com/dockbridge/dockbridge/internal/client/ssh"
	"github.com/dockbridge/dockbridge/internal/shared/config"
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

// MockSSHClient is a mock implementation of ssh.Client
type MockSSHClient struct {
	mock.Mock
}

func (m *MockSSHClient) Connect(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockSSHClient) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockSSHClient) CreateTunnel(ctx context.Context, localAddr, remoteAddr string) (ssh.TunnelInterface, error) {
	args := m.Called(ctx, localAddr, remoteAddr)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(ssh.TunnelInterface), args.Error(1)
}

func (m *MockSSHClient) ExecuteCommand(ctx context.Context, command string) ([]byte, error) {
	args := m.Called(ctx, command)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockSSHClient) IsConnected() bool {
	args := m.Called()
	return args.Bool(0)
}

// MockTunnel is a mock implementation of ssh.TunnelInterface
type MockTunnel struct {
	mock.Mock
	localAddr string
}

func (m *MockTunnel) Start(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockTunnel) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockTunnel) LocalAddr() string {
	if m.localAddr != "" {
		return m.localAddr
	}
	args := m.Called()
	return args.String(0)
}

func (m *MockTunnel) RemoteAddr() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockTunnel) IsActive() bool {
	args := m.Called()
	return args.Bool(0)
}

// MockLogger is a mock implementation of logger.LoggerInterface
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Debug(msg string, fields map[string]interface{}) {
	m.Called(msg, fields)
}

func (m *MockLogger) Info(msg string, fields map[string]interface{}) {
	m.Called(msg, fields)
}

func (m *MockLogger) Warn(msg string, fields map[string]interface{}) {
	m.Called(msg, fields)
}

func (m *MockLogger) Error(msg string, err error, fields map[string]interface{}) {
	m.Called(msg, err, fields)
}

func (m *MockLogger) Fatal(msg string, err error, fields map[string]interface{}) {
	m.Called(msg, err, fields)
}

// DockerProxyTestSuite defines the test suite for DockerProxy
type DockerProxyTestSuite struct {
	suite.Suite
	proxy       DockerProxy
	mockHetzner *MockHetznerClient
	mockSSH     *MockSSHClient
	mockTunnel  *MockTunnel
	mockLogger  *MockLogger
	config      *ProxyConfig
	testServer  *httptest.Server
}

// SetupTest initializes the test suite
func (suite *DockerProxyTestSuite) SetupTest() {
	suite.mockHetzner = &MockHetznerClient{}
	suite.mockSSH = &MockSSHClient{}
	suite.mockTunnel = &MockTunnel{localAddr: "127.0.0.1:12345"}
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

	suite.config = &ProxyConfig{
		SocketPath:    "/tmp/docker-test.sock",
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
}

// TearDownTest cleans up after each test
func (suite *DockerProxyTestSuite) TearDownTest() {
	if suite.testServer != nil {
		suite.testServer.Close()
	}
	if suite.proxy.IsRunning() {
		suite.proxy.Stop()
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

	// Mock expectations for server listing (no existing servers)
	suite.mockHetzner.On("ListServers", ctx).Return([]*hetzner.Server{}, nil)

	// Mock expectations for server provisioning
	expectedServer := &hetzner.Server{
		ID:        12345,
		Name:      "dockbridge-test",
		Status:    "running",
		IPAddress: "192.168.1.100",
	}
	suite.mockHetzner.On("ProvisionServer", ctx, mock.AnythingOfType("*hetzner.ServerConfig")).Return(expectedServer, nil)

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
	suite.mockHetzner.On("ListServers", ctx).Return([]*hetzner.Server{}, nil)
	expectedServer := &hetzner.Server{
		ID:        12345,
		Name:      "dockbridge-test",
		Status:    "running",
		IPAddress: "192.168.1.100",
	}
	suite.mockHetzner.On("ProvisionServer", ctx, mock.AnythingOfType("*hetzner.ServerConfig")).Return(expectedServer, nil)

	err := suite.proxy.Start(ctx, suite.config)
	suite.NoError(err)

	// Stop proxy
	err = suite.proxy.Stop()
	suite.NoError(err)
	suite.False(suite.proxy.IsRunning())
}

// TestGetOrProvisionServerExisting tests finding existing server
func (suite *DockerProxyTestSuite) TestGetOrProvisionServerExisting() {
	ctx := context.Background()

	// Mock existing server
	existingServer := &hetzner.Server{
		ID:        12345,
		Name:      "dockbridge-existing",
		Status:    "running",
		IPAddress: "192.168.1.100",
	}

	suite.mockHetzner.On("ListServers", ctx).Return([]*hetzner.Server{existingServer}, nil)

	// Create proxy implementation to test internal method
	proxyImpl := &proxyImpl{
		config: suite.config,
		logger: suite.mockLogger,
	}

	server, err := proxyImpl.getOrProvisionServer(ctx)
	suite.NoError(err)
	suite.Equal(existingServer.ID, server.ID)
	suite.Equal(existingServer.IPAddress, server.IPAddress)

	suite.mockHetzner.AssertExpectations(suite.T())
}

// TestGetOrProvisionServerNew tests provisioning new server
func (suite *DockerProxyTestSuite) TestGetOrProvisionServerNew() {
	ctx := context.Background()

	// Mock no existing servers
	suite.mockHetzner.On("ListServers", ctx).Return([]*hetzner.Server{}, nil)

	// Mock server provisioning
	newServer := &hetzner.Server{
		ID:        67890,
		Name:      "dockbridge-new",
		Status:    "running",
		IPAddress: "192.168.1.200",
	}
	suite.mockHetzner.On("ProvisionServer", ctx, mock.AnythingOfType("*hetzner.ServerConfig")).Return(newServer, nil)

	// Create proxy implementation to test internal method
	proxyImpl := &proxyImpl{
		config: suite.config,
		logger: suite.mockLogger,
	}

	server, err := proxyImpl.getOrProvisionServer(ctx)
	suite.NoError(err)
	suite.Equal(newServer.ID, server.ID)
	suite.Equal(newServer.IPAddress, server.IPAddress)

	suite.mockHetzner.AssertExpectations(suite.T())
}

// TestForwardRequest tests request forwarding functionality
func (suite *DockerProxyTestSuite) TestForwardRequest() {
	// Create a test request
	reqBody := strings.NewReader(`{"test": "data"}`)
	req, err := http.NewRequest("GET", "/version", reqBody)
	suite.NoError(err)
	req.Header.Set("Content-Type", "application/json")

	// Create proxy implementation with mocked tunnel
	proxyImpl := &proxyImpl{
		config:  suite.config,
		logger:  suite.mockLogger,
		tunnel:  suite.mockTunnel,
		running: true,
		connPool: &connectionPool{
			client: suite.testServer.Client(),
		},
	}

	// Mock tunnel local address to point to test server
	suite.mockTunnel.localAddr = strings.TrimPrefix(suite.testServer.URL, "http://")

	resp, err := proxyImpl.ForwardRequest(req)
	suite.NoError(err)
	suite.NotNil(resp)
	suite.Equal(http.StatusOK, resp.StatusCode)

	// Read response body
	body, err := io.ReadAll(resp.Body)
	suite.NoError(err)
	resp.Body.Close()

	suite.Contains(string(body), "Version")
}

// TestStreamingResponse tests handling of streaming responses
func (suite *DockerProxyTestSuite) TestStreamingResponse() {
	// Create test server that returns streaming response
	streamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.docker.raw-stream")
		w.Header().Set("Transfer-Encoding", "chunked")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}

		// Send streaming data
		for i := 0; i < 3; i++ {
			fmt.Fprintf(w, "chunk %d\n", i)
			flusher.Flush()
			time.Sleep(10 * time.Millisecond)
		}
	}))
	defer streamServer.Close()

	// Create proxy implementation
	proxyImpl := &proxyImpl{
		config:  suite.config,
		logger:  suite.mockLogger,
		tunnel:  suite.mockTunnel,
		running: true,
		connPool: &connectionPool{
			client: streamServer.Client(),
		},
	}

	// Mock tunnel local address
	suite.mockTunnel.localAddr = strings.TrimPrefix(streamServer.URL, "http://")

	// Create test request
	req, err := http.NewRequest("GET", "/logs", nil)
	suite.NoError(err)

	resp, err := proxyImpl.ForwardRequest(req)
	suite.NoError(err)
	suite.NotNil(resp)

	// Read streaming response
	body, err := io.ReadAll(resp.Body)
	suite.NoError(err)
	resp.Body.Close()

	suite.Contains(string(body), "chunk 0")
	suite.Contains(string(body), "chunk 1")
	suite.Contains(string(body), "chunk 2")
}

// TestIsStreamingResponse tests streaming response detection
func (suite *DockerProxyTestSuite) TestIsStreamingResponse() {
	proxyImpl := &proxyImpl{}

	// Test streaming content type
	resp1 := &http.Response{
		Header: http.Header{
			"Content-Type": []string{"application/vnd.docker.raw-stream"},
		},
	}
	suite.True(proxyImpl.isStreamingResponse(resp1))

	// Test chunked transfer encoding
	resp2 := &http.Response{
		Header: http.Header{
			"Transfer-Encoding": []string{"chunked"},
		},
	}
	suite.True(proxyImpl.isStreamingResponse(resp2))

	// Test non-streaming response
	resp3 := &http.Response{
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
	}
	suite.False(proxyImpl.isStreamingResponse(resp3))
}

// TestHandleStreamingResponse tests streaming response handling
func (suite *DockerProxyTestSuite) TestHandleStreamingResponse() {
	proxyImpl := &proxyImpl{}

	// Create mock response with streaming data
	streamData := "line 1\nline 2\nline 3\n"
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(streamData)),
		Header: http.Header{
			"Content-Type": []string{"application/vnd.docker.raw-stream"},
		},
	}

	// Create response recorder
	recorder := httptest.NewRecorder()

	err := proxyImpl.handleStreamingResponse(recorder, resp)
	suite.NoError(err)

	// Check that all data was written
	suite.Equal(streamData, recorder.Body.String())
}

// TestConnectionPooling tests connection pooling functionality
func (suite *DockerProxyTestSuite) TestConnectionPooling() {
	proxy := NewDockerProxy()
	proxyImpl := proxy.(*proxyImpl)

	// Test that connection pool is initialized
	suite.NotNil(proxyImpl.connPool)
	suite.NotNil(proxyImpl.connPool.transport)

	// Test connection pool configuration
	transport := proxyImpl.connPool.transport
	suite.Equal(100, transport.MaxIdleConns)
	suite.Equal(10, transport.MaxIdleConnsPerHost)
	suite.Equal(90*time.Second, transport.IdleConnTimeout)
}

// TestProxyErrorHandling tests error handling scenarios
func (suite *DockerProxyTestSuite) TestProxyErrorHandling() {
	ctx := context.Background()

	// Test error when Hetzner client fails
	suite.mockHetzner.On("ListServers", ctx).Return(nil, fmt.Errorf("hetzner api error"))

	err := suite.proxy.Start(ctx, suite.config)
	suite.Error(err)
	suite.False(suite.proxy.IsRunning())

	suite.mockHetzner.AssertExpectations(suite.T())
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

	config := &ProxyConfig{
		SocketPath:    "/tmp/docker-test.sock",
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
	mockHetzner.On("ListServers", ctx).Return([]*hetzner.Server{}, nil)
	expectedServer := &hetzner.Server{
		ID:        12345,
		Name:      "dockbridge-test",
		Status:    "running",
		IPAddress: "192.168.1.100",
	}
	mockHetzner.On("ProvisionServer", ctx, mock.AnythingOfType("*hetzner.ServerConfig")).Return(expectedServer, nil)

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
}
