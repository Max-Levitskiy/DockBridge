package ssh

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ssh-docker-proxy/internal/config"
)

// MockSSHConn implements net.Conn for testing
type MockSSHConn struct {
	closed bool
}

func (m *MockSSHConn) Read(b []byte) (n int, err error) {
	if m.closed {
		return 0, fmt.Errorf("connection closed")
	}
	return 0, nil
}

func (m *MockSSHConn) Write(b []byte) (n int, err error) {
	if m.closed {
		return 0, fmt.Errorf("connection closed")
	}
	return len(b), nil
}

func (m *MockSSHConn) Close() error {
	m.closed = true
	return nil
}

func (m *MockSSHConn) LocalAddr() net.Addr {
	return &net.UnixAddr{Name: "mock", Net: "unix"}
}

func (m *MockSSHConn) RemoteAddr() net.Addr {
	return &net.UnixAddr{Name: "mock", Net: "unix"}
}

func (m *MockSSHConn) SetDeadline(t time.Time) error {
	return nil
}

func (m *MockSSHConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (m *MockSSHConn) SetWriteDeadline(t time.Time) error {
	return nil
}

// generateTestSSHKey creates a temporary SSH private key for testing
func generateTestSSHKey(t *testing.T) string {
	// Generate RSA private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	// Encode private key to PEM format
	privateKeyPEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}

	// Create temporary file
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "test_key")
	keyFile, err := os.Create(keyPath)
	require.NoError(t, err)
	defer keyFile.Close()

	// Write PEM to file
	err = pem.Encode(keyFile, privateKeyPEM)
	require.NoError(t, err)

	return keyPath
}

func TestNewSSHDialer(t *testing.T) {
	tests := []struct {
		name        string
		config      *config.Config
		expectError bool
		errorType   string
	}{
		{
			name: "valid configuration",
			config: &config.Config{
				SSHUser:      "testuser",
				SSHHost:      "testhost:22",
				SSHKeyPath:   generateTestSSHKey(t),
				RemoteSocket: "/var/run/docker.sock",
				Timeout:      10 * time.Second,
			},
			expectError: false,
		},
		{
			name: "missing SSH key file",
			config: &config.Config{
				SSHUser:      "testuser",
				SSHHost:      "testhost:22",
				SSHKeyPath:   "/nonexistent/key",
				RemoteSocket: "/var/run/docker.sock",
				Timeout:      10 * time.Second,
			},
			expectError: true,
			errorType:   config.ErrorCategorySSH,
		},
		{
			name: "invalid SSH key format",
			config: &config.Config{
				SSHUser:      "testuser",
				SSHHost:      "testhost:22",
				SSHKeyPath:   createInvalidKeyFile(t),
				RemoteSocket: "/var/run/docker.sock",
				Timeout:      10 * time.Second,
			},
			expectError: true,
			errorType:   config.ErrorCategorySSH,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dialer, err := NewSSHDialer(tt.config)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, dialer)

				if proxyErr, ok := err.(*config.ProxyError); ok {
					assert.Equal(t, tt.errorType, proxyErr.Category)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, dialer)
				assert.Equal(t, tt.config, dialer.config)
				assert.NotNil(t, dialer.sshConfig)
				assert.Equal(t, tt.config.SSHUser, dialer.sshConfig.User)
				assert.Equal(t, tt.config.Timeout, dialer.sshConfig.Timeout)
				assert.NotNil(t, dialer.sshConfig.Auth)
				assert.Len(t, dialer.sshConfig.Auth, 1)
			}
		})
	}
}

func createInvalidKeyFile(t *testing.T) string {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "invalid_key")
	err := os.WriteFile(keyPath, []byte("invalid key content"), 0600)
	require.NoError(t, err)
	return keyPath
}

func TestSSHDialer_Dial(t *testing.T) {
	// This test requires a mock SSH server, which is complex to set up
	// For now, we'll test the error cases and structure
	keyPath := generateTestSSHKey(t)

	cfg := &config.Config{
		SSHUser:      "testuser",
		SSHHost:      "nonexistent:22",
		SSHKeyPath:   keyPath,
		RemoteSocket: "/var/run/docker.sock",
		Timeout:      1 * time.Second, // Short timeout for quick test
	}

	dialer, err := NewSSHDialer(cfg)
	require.NoError(t, err)

	// Test connection to nonexistent host (should fail)
	conn, err := dialer.Dial()
	assert.Error(t, err)
	assert.Nil(t, conn)

	// Verify error is properly categorized
	if proxyErr, ok := err.(*config.ProxyError); ok {
		assert.Equal(t, config.ErrorCategorySSH, proxyErr.Category)
		assert.Contains(t, proxyErr.Message, "failed to connect to SSH server")
	}
}

func TestSSHDialer_HealthCheck(t *testing.T) {
	keyPath := generateTestSSHKey(t)

	cfg := &config.Config{
		SSHUser:      "testuser",
		SSHHost:      "nonexistent:22",
		SSHKeyPath:   keyPath,
		RemoteSocket: "/var/run/docker.sock",
		Timeout:      1 * time.Second,
	}

	dialer, err := NewSSHDialer(cfg)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Test health check with nonexistent host (should fail)
	err = dialer.HealthCheck(ctx)
	assert.Error(t, err)

	// Verify error is properly categorized
	if proxyErr, ok := err.(*config.ProxyError); ok {
		// Could be SSH or Docker error depending on where it fails
		assert.Contains(t, []string{config.ErrorCategorySSH, config.ErrorCategoryDocker}, proxyErr.Category)
	}
}

func TestSSHDialer_Configuration(t *testing.T) {
	keyPath := generateTestSSHKey(t)

	cfg := &config.Config{
		SSHUser:      "testuser",
		SSHHost:      "testhost:2222",
		SSHKeyPath:   keyPath,
		RemoteSocket: "/custom/docker.sock",
		Timeout:      30 * time.Second,
	}

	dialer, err := NewSSHDialer(cfg)
	require.NoError(t, err)

	// Verify SSH configuration is set up correctly
	assert.Equal(t, cfg.SSHUser, dialer.sshConfig.User)
	assert.Equal(t, cfg.Timeout, dialer.sshConfig.Timeout)
	assert.NotNil(t, dialer.sshConfig.HostKeyCallback)
	assert.Len(t, dialer.sshConfig.Auth, 1)

	// Verify config is stored
	assert.Equal(t, cfg, dialer.config)
}

func TestSSHDialer_AuthMethodSetup(t *testing.T) {
	keyPath := generateTestSSHKey(t)

	cfg := &config.Config{
		SSHUser:      "testuser",
		SSHHost:      "testhost:22",
		SSHKeyPath:   keyPath,
		RemoteSocket: "/var/run/docker.sock",
		Timeout:      10 * time.Second,
	}

	dialer, err := NewSSHDialer(cfg)
	require.NoError(t, err)

	// Verify that public key authentication is set up
	assert.Len(t, dialer.sshConfig.Auth, 1)

	// The auth method should be a public key method
	// We can't easily test the actual key without more complex setup,
	// but we can verify the structure is correct
	authMethod := dialer.sshConfig.Auth[0]
	assert.NotNil(t, authMethod)
}

// Benchmark tests for performance
func BenchmarkNewSSHDialer(b *testing.B) {
	keyPath := generateTestSSHKey(&testing.T{})

	cfg := &config.Config{
		SSHUser:      "testuser",
		SSHHost:      "testhost:22",
		SSHKeyPath:   keyPath,
		RemoteSocket: "/var/run/docker.sock",
		Timeout:      10 * time.Second,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dialer, err := NewSSHDialer(cfg)
		if err != nil {
			b.Fatal(err)
		}
		_ = dialer
	}
}

// TestIsConnectionError tests the connection error detection logic
func TestIsConnectionError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "connection refused",
			err:      fmt.Errorf("dial tcp: connection refused"),
			expected: true,
		},
		{
			name:     "no such host",
			err:      fmt.Errorf("dial tcp: no such host"),
			expected: true,
		},
		{
			name:     "network unreachable",
			err:      fmt.Errorf("dial tcp: network is unreachable"),
			expected: true,
		},
		{
			name:     "timeout error",
			err:      fmt.Errorf("dial tcp: timeout"),
			expected: true,
		},
		{
			name:     "ssh error",
			err:      fmt.Errorf("ssh: handshake failed"),
			expected: true,
		},
		{
			name:     "docker daemon error",
			err:      fmt.Errorf("docker daemon error: invalid request"),
			expected: false,
		},
		{
			name:     "generic error",
			err:      fmt.Errorf("some other error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isConnectionError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestContains tests the string contains helper function
func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		substr   string
		expected bool
	}{
		{
			name:     "exact match",
			s:        "timeout",
			substr:   "timeout",
			expected: true,
		},
		{
			name:     "substring at start",
			s:        "timeout occurred",
			substr:   "timeout",
			expected: true,
		},
		{
			name:     "substring at end",
			s:        "connection timeout",
			substr:   "timeout",
			expected: true,
		},
		{
			name:     "substring in middle",
			s:        "dial tcp: timeout error",
			substr:   "timeout",
			expected: true,
		},
		{
			name:     "not found",
			s:        "some other error",
			substr:   "timeout",
			expected: false,
		},
		{
			name:     "empty substring",
			s:        "any string",
			substr:   "",
			expected: true,
		},
		{
			name:     "empty string",
			s:        "",
			substr:   "timeout",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contains(tt.s, tt.substr)
			assert.Equal(t, tt.expected, result)
		})
	}
}
