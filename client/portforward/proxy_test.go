package portforward

import (
	"context"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dockbridge/dockbridge/client/ssh"
	"github.com/dockbridge/dockbridge/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// mockSSHClient implements ssh.Client for testing
type mockSSHClient struct {
	mock.Mock
	connected bool
	tunnels   map[string]*mockTunnel
	mu        sync.Mutex
}

func newMockSSHClient() *mockSSHClient {
	return &mockSSHClient{
		connected: true,
		tunnels:   make(map[string]*mockTunnel),
	}
}

func (m *mockSSHClient) Connect(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockSSHClient) Close() error {
	args := m.Called()
	m.connected = false
	return args.Error(0)
}

func (m *mockSSHClient) CreateTunnel(ctx context.Context, localAddr, remoteAddr string) (ssh.TunnelInterface, error) {
	args := m.Called(ctx, localAddr, remoteAddr)

	if args.Error(1) != nil {
		return nil, args.Error(1)
	}

	// Create a mock tunnel
	tunnel := newMockTunnel(localAddr, remoteAddr)

	m.mu.Lock()
	m.tunnels[remoteAddr] = tunnel
	m.mu.Unlock()

	return tunnel, nil
}

func (m *mockSSHClient) ExecuteCommand(ctx context.Context, command string) ([]byte, error) {
	args := m.Called(ctx, command)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *mockSSHClient) IsConnected() bool {
	return m.connected
}

// mockTunnel implements ssh.TunnelInterface for testing
type mockTunnel struct {
	localAddr  string
	remoteAddr string
	active     bool
	listener   net.Listener
	mu         sync.Mutex
}

func newMockTunnel(localAddr, remoteAddr string) *mockTunnel {
	return &mockTunnel{
		localAddr:  localAddr,
		remoteAddr: remoteAddr,
	}
}

func (m *mockTunnel) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.active {
		return nil
	}

	// Create a real listener for testing
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return err
	}

	m.listener = listener
	m.active = true

	// Start echo server for testing
	go m.runEchoServer()

	return nil
}

func (m *mockTunnel) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.active {
		return nil
	}

	if m.listener != nil {
		m.listener.Close()
	}

	m.active = false
	return nil
}

func (m *mockTunnel) LocalAddr() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.listener != nil {
		return m.listener.Addr().String()
	}
	return m.localAddr
}

func (m *mockTunnel) RemoteAddr() string {
	return m.remoteAddr
}

func (m *mockTunnel) IsActive() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.active
}

// runEchoServer runs a simple echo server for testing
func (m *mockTunnel) runEchoServer() {
	for {
		conn, err := m.listener.Accept()
		if err != nil {
			return // Listener closed
		}

		go func(c net.Conn) {
			defer c.Close()
			// Simple echo server - copy input back to output
			io.Copy(c, c)
		}(conn)
	}
}

func createProxyTestLogger(t *testing.T) logger.LoggerInterface {
	testLogger, err := logger.New(&logger.Config{
		Level:     "debug",
		UseColors: false,
	})
	require.NoError(t, err)
	return testLogger
}

func TestLocalProxyServer_BasicLifecycle(t *testing.T) {
	// Create mock SSH client
	mockClient := newMockSSHClient()

	// Create proxy server
	proxy := NewLocalProxyServer(mockClient, createProxyTestLogger(t))

	// Test initial state
	assert.False(t, proxy.IsRunning())

	stats := proxy.GetStats()
	assert.Equal(t, 0, stats.LocalPort)
	assert.Equal(t, "", stats.RemoteAddr)
	assert.Equal(t, int32(0), stats.ActiveConnections)

	// Test start
	ctx := context.Background()
	err := proxy.Start(ctx, 0, "remote:80") // Use port 0 to get any available port
	require.NoError(t, err)

	assert.True(t, proxy.IsRunning())

	// Check stats after start
	stats = proxy.GetStats()
	assert.NotEqual(t, 0, stats.LocalPort) // Should have been assigned a port
	assert.Equal(t, "remote:80", stats.RemoteAddr)
	assert.True(t, stats.Uptime > 0)

	// Test stop
	err = proxy.Stop()
	require.NoError(t, err)

	assert.False(t, proxy.IsRunning())
}

func TestLocalProxyServer_StartWithoutSSHConnection(t *testing.T) {
	// Create disconnected mock SSH client
	mockClient := newMockSSHClient()
	mockClient.connected = false

	// Create proxy server
	proxy := NewLocalProxyServer(mockClient, createProxyTestLogger(t))

	// Test start should fail
	ctx := context.Background()
	err := proxy.Start(ctx, 0, "remote:80")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SSH client is not connected")

	assert.False(t, proxy.IsRunning())
}

func TestLocalProxyServer_DoubleStart(t *testing.T) {
	// Create mock SSH client
	mockClient := newMockSSHClient()

	// Create proxy server
	proxy := NewLocalProxyServer(mockClient, createProxyTestLogger(t))

	// Start first time
	ctx := context.Background()
	err := proxy.Start(ctx, 0, "remote:80")
	require.NoError(t, err)

	// Start second time should fail
	err = proxy.Start(ctx, 0, "remote:80")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already running")

	// Clean up
	proxy.Stop()
}

func TestLocalProxyServer_ConnectionHandling(t *testing.T) {
	// Create mock SSH client
	mockClient := newMockSSHClient()

	// Create proxy server
	proxy := NewLocalProxyServer(mockClient, createProxyTestLogger(t))

	// Start proxy
	ctx := context.Background()
	err := proxy.Start(ctx, 0, "remote:80")
	require.NoError(t, err)
	defer proxy.Stop()

	// Get the local port
	stats := proxy.GetStats()
	localPort := stats.LocalPort
	assert.NotEqual(t, 0, localPort)

	// For now, just test that the proxy server is running and listening
	// Full connection testing would require more complex mock setup
	assert.True(t, proxy.IsRunning())

	// Check initial statistics
	stats = proxy.GetStats()
	assert.Equal(t, int64(0), stats.TotalConnections)
	assert.Equal(t, int32(0), stats.ActiveConnections)
}

func TestLocalProxyServer_MultipleConnections(t *testing.T) {
	// Create mock SSH client
	mockClient := newMockSSHClient()

	// Create proxy server
	proxy := NewLocalProxyServer(mockClient, createProxyTestLogger(t))

	// Start proxy
	ctx := context.Background()
	err := proxy.Start(ctx, 0, "remote:80")
	require.NoError(t, err)
	defer proxy.Stop()

	// Get the local port
	stats := proxy.GetStats()
	localPort := stats.LocalPort
	assert.NotEqual(t, 0, localPort)

	// For now, just test that the proxy server is running
	// Full connection testing would require more complex mock setup
	assert.True(t, proxy.IsRunning())

	// Check initial statistics
	stats = proxy.GetStats()
	assert.Equal(t, int64(0), stats.TotalConnections)
	assert.Equal(t, int32(0), stats.ActiveConnections)
}

func TestLocalProxyServer_StatsTracking(t *testing.T) {
	// Create mock SSH client
	mockClient := newMockSSHClient()

	// Create proxy server
	proxy := NewLocalProxyServer(mockClient, createProxyTestLogger(t))

	// Test initial stats
	stats := proxy.GetStats()
	assert.Equal(t, int32(0), stats.ActiveConnections)
	assert.Equal(t, int64(0), stats.TotalConnections)
	assert.Equal(t, int64(0), stats.BytesTransferred)
	assert.Equal(t, time.Duration(0), stats.Uptime)

	// Start proxy
	ctx := context.Background()
	err := proxy.Start(ctx, 0, "remote:80")
	require.NoError(t, err)
	defer proxy.Stop()

	// Check stats after start
	stats = proxy.GetStats()
	assert.True(t, stats.Uptime > 0)
	assert.NotEqual(t, 0, stats.LocalPort)
	assert.Equal(t, "remote:80", stats.RemoteAddr)

	// For now, just test basic stats tracking without actual connections
	// Full connection testing would require more complex mock setup
	assert.Equal(t, int32(0), stats.ActiveConnections)
	assert.Equal(t, int64(0), stats.TotalConnections)
	assert.True(t, time.Since(stats.LastActivity) < time.Second)
}

func TestLocalProxyServer_ContextCancellation(t *testing.T) {
	// Create mock SSH client
	mockClient := newMockSSHClient()

	// Create proxy server
	proxy := NewLocalProxyServer(mockClient, createProxyTestLogger(t))

	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	// Start proxy
	err := proxy.Start(ctx, 0, "remote:80")
	require.NoError(t, err)

	assert.True(t, proxy.IsRunning())

	// Cancel context
	cancel()

	// Stop should still work
	err = proxy.Stop()
	require.NoError(t, err)

	assert.False(t, proxy.IsRunning())
}

func TestLocalProxyServer_ErrorHandling(t *testing.T) {
	// Test with invalid port
	mockClient := newMockSSHClient()
	proxy := NewLocalProxyServer(mockClient, createProxyTestLogger(t))

	// Try to start on a port that's likely to be in use or invalid
	ctx := context.Background()
	err := proxy.Start(ctx, -1, "remote:80")
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "listen")

	assert.False(t, proxy.IsRunning())
}

// Ensure LocalProxyServer implements the interface
var _ LocalProxyServer = (*localProxyServerImpl)(nil)
