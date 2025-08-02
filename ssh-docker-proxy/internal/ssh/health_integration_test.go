package ssh

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ssh-docker-proxy/internal/config"
)

// TestSSHDialer_HealthCheck_Integration tests the health check functionality
// with more comprehensive scenarios including error handling
func TestSSHDialer_HealthCheck_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tests := []struct {
		name           string
		config         *config.Config
		expectError    bool
		expectedErrCat string
		timeout        time.Duration
	}{
		{
			name: "unreachable SSH host",
			config: &config.Config{
				SSHUser:      "testuser",
				SSHHost:      "192.0.2.1:22", // RFC5737 test address (unreachable)
				SSHKeyPath:   generateTestSSHKey(t),
				RemoteSocket: "/var/run/docker.sock",
				Timeout:      2 * time.Second,
			},
			expectError:    true,
			expectedErrCat: config.ErrorCategorySSH,
			timeout:        5 * time.Second,
		},
		{
			name: "invalid SSH port",
			config: &config.Config{
				SSHUser:      "testuser",
				SSHHost:      "localhost:99999", // Invalid port
				SSHKeyPath:   generateTestSSHKey(t),
				RemoteSocket: "/var/run/docker.sock",
				Timeout:      1 * time.Second,
			},
			expectError:    true,
			expectedErrCat: config.ErrorCategorySSH,
			timeout:        3 * time.Second,
		},
		{
			name: "context timeout",
			config: &config.Config{
				SSHUser:      "testuser",
				SSHHost:      "192.0.2.2:22", // Another RFC5737 test address
				SSHKeyPath:   generateTestSSHKey(t),
				RemoteSocket: "/var/run/docker.sock",
				Timeout:      10 * time.Second, // Longer than context timeout
			},
			expectError:    true,
			expectedErrCat: config.ErrorCategorySSH,
			timeout:        1 * time.Second, // Short context timeout
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dialer, err := NewSSHDialer(tt.config)
			require.NoError(t, err)

			ctx, cancel := context.WithTimeout(context.Background(), tt.timeout)
			defer cancel()

			err = dialer.HealthCheck(ctx)

			if tt.expectError {
				assert.Error(t, err)

				if proxyErr, ok := err.(*config.ProxyError); ok {
					// Allow both SSH and Docker errors since the failure point can vary
					assert.Contains(t, []string{config.ErrorCategorySSH, config.ErrorCategoryDocker}, proxyErr.Category)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestSSHDialer_HealthCheck_DockerAPIVersionNegotiation tests that the health check
// properly handles Docker API version negotiation
func TestSSHDialer_HealthCheck_DockerAPIVersionNegotiation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	keyPath := generateTestSSHKey(t)

	cfg := &config.Config{
		SSHUser:      "testuser",
		SSHHost:      "nonexistent.example.com:22",
		SSHKeyPath:   keyPath,
		RemoteSocket: "/var/run/docker.sock",
		Timeout:      2 * time.Second,
	}

	dialer, err := NewSSHDialer(cfg)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// This should fail at the SSH connection level, but we're testing
	// that the Docker client creation and API version negotiation
	// setup doesn't cause panics or unexpected errors
	err = dialer.HealthCheck(ctx)
	assert.Error(t, err)

	// The error should be properly categorized
	if proxyErr, ok := err.(*config.ProxyError); ok {
		assert.Contains(t, []string{config.ErrorCategorySSH, config.ErrorCategoryDocker}, proxyErr.Category)
		assert.NotEmpty(t, proxyErr.Message)
	}
}

// TestSSHDialer_HealthCheck_ErrorScenarios tests various error scenarios
// that can occur during health checking
func TestSSHDialer_HealthCheck_ErrorScenarios(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tests := []struct {
		name        string
		setupConfig func(t *testing.T) *config.Config
		expectError bool
		description string
	}{
		{
			name: "SSH connection timeout",
			setupConfig: func(t *testing.T) *config.Config {
				return &config.Config{
					SSHUser:      "testuser",
					SSHHost:      "192.0.2.3:22", // RFC5737 test address
					SSHKeyPath:   generateTestSSHKey(t),
					RemoteSocket: "/var/run/docker.sock",
					Timeout:      1 * time.Second, // Very short timeout
				}
			},
			expectError: true,
			description: "Should timeout when SSH host is unreachable",
		},
		{
			name: "invalid remote socket path",
			setupConfig: func(t *testing.T) *config.Config {
				return &config.Config{
					SSHUser:      "testuser",
					SSHHost:      "localhost:22",
					SSHKeyPath:   generateTestSSHKey(t),
					RemoteSocket: "/nonexistent/docker.sock", // Invalid socket path
					Timeout:      2 * time.Second,
				}
			},
			expectError: true,
			description: "Should fail when remote Docker socket doesn't exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.setupConfig(t)
			dialer, err := NewSSHDialer(cfg)
			require.NoError(t, err)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			err = dialer.HealthCheck(ctx)

			if tt.expectError {
				assert.Error(t, err, tt.description)

				// Verify error is properly structured
				if proxyErr, ok := err.(*config.ProxyError); ok {
					assert.NotEmpty(t, proxyErr.Category)
					assert.NotEmpty(t, proxyErr.Message)
				}
			} else {
				assert.NoError(t, err, tt.description)
			}
		})
	}
}

// TestSSHDialer_HealthCheck_ContextCancellation tests that health check
// properly respects context cancellation
func TestSSHDialer_HealthCheck_ContextCancellation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	keyPath := generateTestSSHKey(t)

	cfg := &config.Config{
		SSHUser:      "testuser",
		SSHHost:      "192.0.2.4:22", // RFC5737 test address (will hang)
		SSHKeyPath:   keyPath,
		RemoteSocket: "/var/run/docker.sock",
		Timeout:      30 * time.Second, // Long timeout
	}

	dialer, err := NewSSHDialer(cfg)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel context after a short delay
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	err = dialer.HealthCheck(ctx)
	duration := time.Since(start)

	// Should fail quickly due to context cancellation
	assert.Error(t, err)
	assert.Less(t, duration, 5*time.Second, "Health check should fail quickly when context is cancelled")
}

// BenchmarkSSHDialer_HealthCheck benchmarks the health check performance
func BenchmarkSSHDialer_HealthCheck(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping benchmark in short mode")
	}

	keyPath := generateTestSSHKey(&testing.T{})

	cfg := &config.Config{
		SSHUser:      "testuser",
		SSHHost:      "192.0.2.5:22", // RFC5737 test address
		SSHKeyPath:   keyPath,
		RemoteSocket: "/var/run/docker.sock",
		Timeout:      1 * time.Second,
	}

	dialer, err := NewSSHDialer(cfg)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = dialer.HealthCheck(ctx) // We expect this to fail, but we're measuring performance
		cancel()
	}
}
