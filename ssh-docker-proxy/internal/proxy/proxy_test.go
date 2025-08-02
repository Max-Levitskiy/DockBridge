package proxy

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"ssh-docker-proxy/internal/config"
)

// mockConn implements net.Conn for testing
type mockConn struct {
	readBuffer  *bytes.Buffer
	writeBuffer *bytes.Buffer
	closed      bool
	mu          sync.Mutex
}

func newMockConn() *mockConn {
	return &mockConn{
		readBuffer:  &bytes.Buffer{},
		writeBuffer: &bytes.Buffer{},
	}
}

func (m *mockConn) Read(b []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return 0, io.EOF
	}
	return m.readBuffer.Read(b)
}

func (m *mockConn) Write(b []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return 0, io.ErrClosedPipe
	}
	return m.writeBuffer.Write(b)
}

func (m *mockConn) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *mockConn) LocalAddr() net.Addr                { return nil }
func (m *mockConn) RemoteAddr() net.Addr               { return nil }
func (m *mockConn) SetDeadline(t time.Time) error      { return nil }
func (m *mockConn) SetReadDeadline(t time.Time) error  { return nil }
func (m *mockConn) SetWriteDeadline(t time.Time) error { return nil }

// writeData writes data to the mock connection's read buffer (simulating incoming data)
func (m *mockConn) writeData(data []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.readBuffer.Write(data)
}

// readData reads data from the mock connection's write buffer (data that was written to the connection)
func (m *mockConn) readData() []byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.writeBuffer.Bytes()
}

func TestRelayTraffic(t *testing.T) {
	tests := []struct {
		name           string
		localData      []byte
		remoteData     []byte
		expectedLocal  []byte
		expectedRemote []byte
	}{
		{
			name:           "bidirectional data relay",
			localData:      []byte("GET /containers/json HTTP/1.1\r\n\r\n"),
			remoteData:     []byte("HTTP/1.1 200 OK\r\n\r\n[]"),
			expectedLocal:  []byte("HTTP/1.1 200 OK\r\n\r\n[]"),
			expectedRemote: []byte("GET /containers/json HTTP/1.1\r\n\r\n"),
		},
		{
			name:           "empty data",
			localData:      []byte(""),
			remoteData:     []byte(""),
			expectedLocal:  []byte(""),
			expectedRemote: []byte(""),
		},
		{
			name:           "large data transfer",
			localData:      bytes.Repeat([]byte("A"), 1024),
			remoteData:     bytes.Repeat([]byte("B"), 2048),
			expectedLocal:  bytes.Repeat([]byte("B"), 2048),
			expectedRemote: bytes.Repeat([]byte("A"), 1024),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			localConn := newMockConn()
			remoteConn := newMockConn()

			// Create a logger that discards output for testing
			logger := log.New(io.Discard, "", 0)

			// Write test data to connections
			localConn.writeData(tt.localData)
			remoteConn.writeData(tt.remoteData)

			// Start relay in goroutine
			done := make(chan struct{})
			go func() {
				defer close(done)
				relayTraffic(localConn, remoteConn, logger)
			}()

			// Give some time for data to be relayed
			time.Sleep(10 * time.Millisecond)

			// Close connections to stop relay
			localConn.Close()
			remoteConn.Close()

			// Wait for relay to complete
			select {
			case <-done:
			case <-time.After(1 * time.Second):
				t.Fatal("Relay did not complete within timeout")
			}

			// Verify data was relayed correctly
			if !bytes.Equal(localConn.readData(), tt.expectedLocal) {
				t.Errorf("Local connection received incorrect data.\nExpected: %q\nGot: %q",
					tt.expectedLocal, localConn.readData())
			}

			if !bytes.Equal(remoteConn.readData(), tt.expectedRemote) {
				t.Errorf("Remote connection received incorrect data.\nExpected: %q\nGot: %q",
					tt.expectedRemote, remoteConn.readData())
			}
		})
	}
}

func TestProxyCreation(t *testing.T) {
	// Create a temporary SSH key file for testing
	tmpFile, err := os.CreateTemp("", "test_ssh_key")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write a dummy SSH key (this won't be used for actual SSH in unit tests)
	sshKey := `-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAFwAAAAdzc2gtcn
NhAAAAAwEAAQAAAQEA1234567890abcdef
-----END OPENSSH PRIVATE KEY-----`

	if _, err := tmpFile.WriteString(sshKey); err != nil {
		t.Fatalf("Failed to write SSH key: %v", err)
	}
	tmpFile.Close()

	cfg := &config.Config{
		LocalSocket:  "/tmp/test-docker.sock",
		SSHUser:      "testuser",
		SSHHost:      "localhost:22",
		SSHKeyPath:   tmpFile.Name(),
		RemoteSocket: "/var/run/docker.sock",
		Timeout:      10 * time.Second,
	}

	logger := log.New(io.Discard, "", 0)

	// Test proxy creation - this should fail because we can't actually connect to SSH
	// but it should validate the configuration and create the proxy struct
	_, err = NewProxy(cfg, logger)

	// We expect this to fail with SSH-related error since we're using a dummy key
	// but the proxy should be created successfully up to the SSH dialer creation
	if err != nil {
		// Check that it's an SSH-related error, not a config error
		if !strings.Contains(err.Error(), "SSH") && !strings.Contains(err.Error(), "ssh") {
			t.Errorf("Expected SSH-related error, got: %v", err)
		}
	}
}

func TestConfigValidation(t *testing.T) {
	// Test configuration validation by testing the config.Validate() method directly
	// since the SSH dialer creation happens before we can test proxy creation

	tests := []struct {
		name        string
		config      *config.Config
		expectError bool
		errorType   string
	}{
		{
			name: "missing SSH user",
			config: &config.Config{
				LocalSocket:  "/tmp/test.sock",
				SSHHost:      "localhost",
				SSHKeyPath:   "/tmp/key",
				RemoteSocket: "/var/run/docker.sock",
			},
			expectError: true,
			errorType:   "CONFIG",
		},
		{
			name: "missing SSH host",
			config: &config.Config{
				LocalSocket:  "/tmp/test.sock",
				SSHUser:      "user",
				SSHKeyPath:   "/tmp/key",
				RemoteSocket: "/var/run/docker.sock",
			},
			expectError: true,
			errorType:   "CONFIG",
		},
		{
			name: "missing SSH key path",
			config: &config.Config{
				LocalSocket:  "/tmp/test.sock",
				SSHUser:      "user",
				SSHHost:      "localhost",
				RemoteSocket: "/var/run/docker.sock",
			},
			expectError: true,
			errorType:   "CONFIG",
		},
		{
			name: "missing local socket",
			config: &config.Config{
				SSHUser:      "user",
				SSHHost:      "localhost",
				SSHKeyPath:   "/tmp/key",
				RemoteSocket: "/var/run/docker.sock",
			},
			expectError: true,
			errorType:   "CONFIG",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
					return
				}

				if !strings.Contains(err.Error(), tt.errorType) {
					t.Errorf("Expected error type %s, got: %v", tt.errorType, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestRelayTrafficConcurrency(t *testing.T) {
	// Test that relayTraffic can handle concurrent read/write operations
	localConn := newMockConn()
	remoteConn := newMockConn()
	logger := log.New(io.Discard, "", 0)

	// Test data for concurrent operations
	testData1 := []byte("First message")
	testData2 := []byte("Second message")

	// Write initial data
	localConn.writeData(testData1)
	remoteConn.writeData(testData2)

	// Start relay
	done := make(chan struct{})
	go func() {
		defer close(done)
		relayTraffic(localConn, remoteConn, logger)
	}()

	// Give time for initial relay
	time.Sleep(10 * time.Millisecond)

	// Write more data while relay is running
	additionalData := []byte(" - additional")
	localConn.writeData(additionalData)

	// Give time for additional data to be relayed
	time.Sleep(10 * time.Millisecond)

	// Close connections
	localConn.Close()
	remoteConn.Close()

	// Wait for relay to complete
	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("Relay did not complete within timeout")
	}

	// Verify that data was relayed
	localReceived := localConn.readData()
	remoteReceived := remoteConn.readData()

	if len(localReceived) == 0 {
		t.Error("Local connection should have received data from remote")
	}
	if len(remoteReceived) == 0 {
		t.Error("Remote connection should have received data from local")
	}

	// Check that at least the initial data was relayed
	if !bytes.Contains(localReceived, testData2) {
		t.Errorf("Local connection should have received remote data: %q", testData2)
	}
	if !bytes.Contains(remoteReceived, testData1) {
		t.Errorf("Remote connection should have received local data: %q", testData1)
	}
}
func TestConcurrentConnections(t *testing.T) {
	// Test multiple simultaneous connections to ensure proper goroutine handling
	numConnections := 5
	connections := make([]*mockConn, numConnections*2) // local and remote pairs

	logger := log.New(io.Discard, "", 0)

	// Create connection pairs
	for i := 0; i < numConnections; i++ {
		connections[i*2] = newMockConn()   // local
		connections[i*2+1] = newMockConn() // remote
	}

	// Start concurrent relays
	done := make(chan struct{}, numConnections)

	for i := 0; i < numConnections; i++ {
		localConn := connections[i*2]
		remoteConn := connections[i*2+1]

		// Write test data for this connection
		testData := []byte(fmt.Sprintf("Connection %d data", i))
		responseData := []byte(fmt.Sprintf("Response %d data", i))

		localConn.writeData(testData)
		remoteConn.writeData(responseData)

		// Start relay in goroutine (simulating concurrent connections)
		go func(local, remote *mockConn, connNum int) {
			defer func() { done <- struct{}{} }()
			relayTraffic(local, remote, logger)
		}(localConn, remoteConn, i)
	}

	// Give time for all relays to process data
	time.Sleep(50 * time.Millisecond)

	// Close all connections
	for _, conn := range connections {
		conn.Close()
	}

	// Wait for all relays to complete
	for i := 0; i < numConnections; i++ {
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatalf("Relay %d did not complete within timeout", i)
		}
	}

	// Verify all connections processed their data
	for i := 0; i < numConnections; i++ {
		localConn := connections[i*2]
		remoteConn := connections[i*2+1]

		expectedLocal := fmt.Sprintf("Response %d data", i)
		expectedRemote := fmt.Sprintf("Connection %d data", i)

		if !bytes.Contains(localConn.readData(), []byte(expectedLocal)) {
			t.Errorf("Connection %d: local did not receive expected data", i)
		}

		if !bytes.Contains(remoteConn.readData(), []byte(expectedRemote)) {
			t.Errorf("Connection %d: remote did not receive expected data", i)
		}
	}
}

func TestConnectionLifecycleManagement(t *testing.T) {
	// Test proper connection cleanup and lifecycle management
	localConn := newMockConn()
	remoteConn := newMockConn()

	// Capture log output to verify lifecycle logging
	var logBuffer bytes.Buffer
	logger := log.New(&logBuffer, "", 0)

	// Write test data
	testData := []byte("Lifecycle test data")
	localConn.writeData(testData)

	// Start relay
	done := make(chan struct{})
	go func() {
		defer close(done)
		relayTraffic(localConn, remoteConn, logger)
	}()

	// Give time for data transfer
	time.Sleep(10 * time.Millisecond)

	// Close connections to trigger cleanup
	localConn.Close()
	remoteConn.Close()

	// Wait for relay to complete
	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("Relay did not complete within timeout")
	}

	// Verify logging occurred
	logOutput := logBuffer.String()
	if !strings.Contains(logOutput, "copy completed") && !strings.Contains(logOutput, "copy ended") {
		t.Error("Expected lifecycle logging not found in output")
	}

	// Verify data was transferred
	if !bytes.Equal(remoteConn.readData(), testData) {
		t.Error("Data was not properly transferred during lifecycle test")
	}
}

func TestConnectionIsolation(t *testing.T) {
	// Test that connections are properly isolated from each other
	conn1Local := newMockConn()
	conn1Remote := newMockConn()
	conn2Local := newMockConn()
	conn2Remote := newMockConn()

	logger := log.New(io.Discard, "", 0)

	// Different data for each connection
	data1 := []byte("Connection 1 exclusive data")
	data2 := []byte("Connection 2 exclusive data")

	conn1Local.writeData(data1)
	conn2Local.writeData(data2)

	// Start both relays
	done1 := make(chan struct{})
	done2 := make(chan struct{})

	go func() {
		defer close(done1)
		relayTraffic(conn1Local, conn1Remote, logger)
	}()

	go func() {
		defer close(done2)
		relayTraffic(conn2Local, conn2Remote, logger)
	}()

	// Give time for data transfer
	time.Sleep(20 * time.Millisecond)

	// Close all connections
	conn1Local.Close()
	conn1Remote.Close()
	conn2Local.Close()
	conn2Remote.Close()

	// Wait for both relays to complete
	select {
	case <-done1:
	case <-time.After(1 * time.Second):
		t.Fatal("Connection 1 relay did not complete")
	}

	select {
	case <-done2:
	case <-time.After(1 * time.Second):
		t.Fatal("Connection 2 relay did not complete")
	}

	// Verify isolation - each connection should only have its own data
	if bytes.Contains(conn1Remote.readData(), data2) {
		t.Error("Connection 1 received data from connection 2 - isolation failed")
	}

	if bytes.Contains(conn2Remote.readData(), data1) {
		t.Error("Connection 2 received data from connection 1 - isolation failed")
	}

	// Verify each connection got its correct data
	if !bytes.Equal(conn1Remote.readData(), data1) {
		t.Error("Connection 1 did not receive its correct data")
	}

	if !bytes.Equal(conn2Remote.readData(), data2) {
		t.Error("Connection 2 did not receive its correct data")
	}
}
