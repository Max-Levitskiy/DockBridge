package ssh

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTunnel tests the tunnel functionality with a mock SSH client
func TestTunnel(t *testing.T) {
	// This is a simplified test that doesn't use a real SSH connection
	// but tests the tunnel logic independently

	// Create a mock echo server that will echo back any data sent to it
	echoListener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer echoListener.Close()

	// Start the echo server
	echoServerAddr := echoListener.Addr().String()
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := echoListener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				io.Copy(c, c) // Echo back any data received
			}(conn)
		}
	}()

	// Create a mock SSH client that will connect directly to our echo server
	// This bypasses the actual SSH protocol but allows us to test the tunnel logic
	mockSSHClient := &mockSSHClient{
		echoServerAddr: echoServerAddr,
	}

	// Create a tunnel using our mock SSH client
	localAddr := "127.0.0.1:0" // Let the system choose a port
	tunnel := NewTunnelWithClient(mockSSHClient, localAddr, echoServerAddr)

	// Start the tunnel
	ctx := context.Background()
	err = tunnel.Start(ctx)
	require.NoError(t, err)
	defer tunnel.Close()

	// Get the actual local address that was assigned
	localAddr = tunnel.listener.Addr().String()

	// Connect to the tunnel
	conn, err := net.Dial("tcp", localAddr)
	require.NoError(t, err)
	defer conn.Close()

	// Send data through the tunnel
	testData := []byte("Hello, Tunnel!")
	_, err = conn.Write(testData)
	require.NoError(t, err)

	// Read the echoed data
	buffer := make([]byte, len(testData))
	deadline := time.Now().Add(2 * time.Second)
	err = conn.SetReadDeadline(deadline)
	require.NoError(t, err)

	n, err := conn.Read(buffer)
	require.NoError(t, err)
	assert.Equal(t, len(testData), n)
	assert.Equal(t, testData, buffer[:n])

	// Test closing the tunnel
	err = tunnel.Close()
	require.NoError(t, err)
	assert.False(t, tunnel.IsActive())

	// Verify that the listener is closed by trying to connect again
	_, err = net.Dial("tcp", localAddr)
	assert.Error(t, err)
}

// mockSSHClient is a simplified mock of an SSH client for testing
type mockSSHClient struct {
	echoServerAddr string
}

// Dial implements the SSH client Dial method but connects directly to the echo server
func (m *mockSSHClient) Dial(network, addr string) (net.Conn, error) {
	// Ignore the addr parameter and connect to our echo server instead
	return net.Dial(network, m.echoServerAddr)
}

// NewTunnelWithClient creates a new tunnel with a custom SSH client for testing
func NewTunnelWithClient(client sshDialer, localAddr, remoteAddr string) *Tunnel {
	ctx, cancel := context.WithCancel(context.Background())
	return &Tunnel{
		sshClient:  nil, // Not used in this test
		localAddr:  localAddr,
		remoteAddr: remoteAddr,
		ctx:        ctx,
		cancel:     cancel,
		dialer:     client,
	}
}

// The sshDialer interface is defined in tunnel.go
