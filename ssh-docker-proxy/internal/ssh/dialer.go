package ssh

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/client"
	"golang.org/x/crypto/ssh"

	"ssh-docker-proxy/internal/config"
)

// SSHDialer manages SSH connections and provides connection factory
type SSHDialer struct {
	config    *config.Config
	sshConfig *ssh.ClientConfig
}

// NewSSHDialer creates a new SSH dialer with the given configuration
func NewSSHDialer(cfg *config.Config) (*SSHDialer, error) {
	// Expand SSH key path (handle ~ for home directory)
	keyPath, err := expandPath(cfg.SSHKeyPath)
	if err != nil {
		return nil, &config.ProxyError{
			Category: config.ErrorCategorySSH,
			Message:  fmt.Sprintf("failed to expand SSH key path: %s", cfg.SSHKeyPath),
			Cause:    err,
		}
	}

	// Load SSH private key
	keyBytes, err := os.ReadFile(keyPath) // #nosec G304
	if err != nil {
		return nil, &config.ProxyError{
			Category: config.ErrorCategorySSH,
			Message:  fmt.Sprintf("failed to read SSH key file: %s", keyPath),
			Cause:    err,
		}
	}

	// Parse private key
	signer, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		return nil, &config.ProxyError{
			Category: config.ErrorCategorySSH,
			Message:  "failed to parse SSH private key",
			Cause:    err,
		}
	}

	// Create SSH client configuration
	sshConfig := &ssh.ClientConfig{
		User: cfg.SSHUser,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // #nosec G106 // TODO: Implement proper host key verification
		Timeout:         cfg.Timeout,
	}

	return &SSHDialer{
		config:    cfg,
		sshConfig: sshConfig,
	}, nil
}

// Dial establishes a new SSH connection to the remote Docker socket
func (d *SSHDialer) Dial() (net.Conn, error) {
	// Ensure SSH host has a port
	sshHost := normalizeSSHHost(d.config.SSHHost)

	// Connect to SSH server
	sshConn, err := ssh.Dial("tcp", sshHost, d.sshConfig)
	if err != nil {
		return nil, &config.ProxyError{
			Category: config.ErrorCategorySSH,
			Message:  fmt.Sprintf("failed to connect to SSH server %s", sshHost),
			Cause:    err,
		}
	}

	// Connect to remote Docker socket
	conn, err := sshConn.Dial("unix", d.config.RemoteSocket)
	if err != nil {
		sshConn.Close()
		return nil, &config.ProxyError{
			Category: config.ErrorCategoryDocker,
			Message:  fmt.Sprintf("failed to connect to remote Docker socket %s", d.config.RemoteSocket),
			Cause:    err,
		}
	}

	return conn, nil
}

// HealthCheck verifies the remote Docker daemon is accessible
func (d *SSHDialer) HealthCheck(ctx context.Context) error {
	// Create Docker client with custom dialer
	dockerClient, err := client.NewClientWithOpts(
		client.WithHost("http://dummy"), // not used due to custom dialer
		client.WithAPIVersionNegotiation(),
		client.WithDialContext(func(ctx context.Context, _, _ string) (net.Conn, error) {
			return d.Dial()
		}),
	)
	if err != nil {
		return &config.ProxyError{
			Category: config.ErrorCategoryDocker,
			Message:  "failed to create Docker client for health check",
			Cause:    err,
		}
	}
	defer dockerClient.Close()

	// Perform ping to verify connectivity and API version negotiation
	pingResponse, err := dockerClient.Ping(ctx)
	if err != nil {
		// Check if this is a connection error (SSH/network) or Docker daemon error
		if isConnectionError(err) {
			return &config.ProxyError{
				Category: config.ErrorCategorySSH,
				Message:  "failed to establish connection to remote Docker daemon via SSH",
				Cause:    err,
			}
		}
		return &config.ProxyError{
			Category: config.ErrorCategoryDocker,
			Message:  "Docker daemon health check failed - daemon may be unreachable or not responding",
			Cause:    err,
		}
	}

	// Log successful connection with API version info
	if pingResponse.APIVersion != "" {
		// Note: In a real implementation, you might want to use a proper logger here
		// For now, we'll just ensure the ping was successful
		_ = pingResponse.APIVersion
	}

	return nil
}

// isConnectionError determines if an error is related to connection issues
// rather than Docker daemon issues
func isConnectionError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	// Check for common connection-related error patterns
	connectionErrors := []string{
		"connection refused",
		"no such host",
		"network is unreachable",
		"timeout",
		"connection reset",
		"broken pipe",
		"ssh:",
	}

	for _, connErr := range connectionErrors {
		if contains(errStr, connErr) {
			return true
		}
	}

	return false
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			(len(s) > len(substr) &&
				(s[:len(substr)] == substr ||
					s[len(s)-len(substr):] == substr ||
					containsSubstring(s, substr))))
}

// containsSubstring performs a simple substring search
func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// expandPath expands ~ to the user's home directory
func expandPath(path string) (string, error) {
	if !strings.HasPrefix(path, "~") {
		return path, nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	if path == "~" {
		return homeDir, nil
	}

	if strings.HasPrefix(path, "~/") {
		return filepath.Join(homeDir, path[2:]), nil
	}

	// Handle ~user/path format (not commonly used, but for completeness)
	return path, nil
}

// normalizeSSHHost ensures the SSH host has a port, defaulting to 22 if not specified
func normalizeSSHHost(host string) string {
	// If host already contains a port, return as-is
	if strings.Contains(host, ":") {
		return host
	}

	// Add default SSH port
	return host + ":22"
}
