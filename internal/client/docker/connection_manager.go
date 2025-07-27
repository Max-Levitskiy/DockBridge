package docker

import (
	"context"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/dockbridge/dockbridge/internal/client/ssh"
	"github.com/dockbridge/dockbridge/internal/shared/config"
	"github.com/dockbridge/dockbridge/pkg/logger"
	"github.com/pkg/errors"
)

// ProvisioningState represents the current state of server provisioning
type ProvisioningState int

const (
	StateIdle ProvisioningState = iota
	StateProvisioning
	StateReady
	StateFailed
)

// connectionPool manages HTTP connections for performance optimization
type connectionPool struct {
	transport *http.Transport
	client    *http.Client
}

// ConnectionManager handles SSH connections, tunnels, and connection pooling
type ConnectionManager struct {
	sshClient         ssh.Client
	tunnel            ssh.TunnelInterface
	connPool          *connectionPool
	serverManager     *ServerManager
	sshConfig         *config.SSHConfig
	logger            logger.LoggerInterface
	mu                sync.RWMutex
	provisionMutex    sync.Mutex
	provisioningState ProvisioningState
	lastError         error
	lastAttempt       time.Time
	retryCount        int
}

// NewConnectionManager creates a new connection manager
func NewConnectionManager(serverManager *ServerManager, sshConfig *config.SSHConfig, logger logger.LoggerInterface) *ConnectionManager {
	return &ConnectionManager{
		serverManager: serverManager,
		sshConfig:     sshConfig,
		logger:        logger,
		connPool: &connectionPool{
			transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
				DisableKeepAlives:   false,
			},
		},
	}
}

// Initialize sets up the connection pool
func (cm *ConnectionManager) Initialize() {
	cm.connPool.client = &http.Client{
		Transport: cm.connPool.transport,
		Timeout:   30 * time.Second,
	}
}

// EnsureRemoteConnection ensures we have an active connection to a remote server
func (cm *ConnectionManager) EnsureRemoteConnection(ctx context.Context) error {
	// Use mutex to prevent concurrent provisioning
	cm.provisionMutex.Lock()
	defer cm.provisionMutex.Unlock()

	// Check if we already have an active SSH connection
	if cm.sshClient != nil && cm.sshClient.IsConnected() && cm.tunnel != nil {
		cm.mu.Lock()
		cm.provisioningState = StateReady
		cm.mu.Unlock()
		return nil
	}

	// Check current provisioning state
	cm.mu.RLock()
	currentState := cm.provisioningState
	lastError := cm.lastError
	lastAttempt := cm.lastAttempt
	retryCount := cm.retryCount
	cm.mu.RUnlock()

	// If we're currently provisioning, return appropriate error
	if currentState == StateProvisioning {
		return errors.New("server provisioning in progress, please wait")
	}

	// Implement exponential backoff for failed attempts
	if currentState == StateFailed && time.Since(lastAttempt) < cm.getBackoffDuration(retryCount) {
		return errors.Errorf("waiting for retry backoff, last error: %v", lastError)
	}

	// Set state to provisioning
	cm.mu.Lock()
	cm.provisioningState = StateProvisioning
	cm.lastAttempt = time.Now()
	cm.mu.Unlock()

	cm.logger.Info("Establishing connection to remote server")

	// Create a fresh context for provisioning (not derived from request context)
	provisionCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Perform the actual connection establishment
	if err := cm.doEstablishConnection(provisionCtx); err != nil {
		// Update state on failure
		cm.mu.Lock()
		cm.provisioningState = StateFailed
		cm.lastError = err
		cm.retryCount++
		cm.mu.Unlock()

		cm.logger.WithFields(map[string]interface{}{
			"error":       err.Error(),
			"retry_count": cm.retryCount,
		}).Error("Failed to establish connection")

		return err
	}

	// Update state on success
	cm.mu.Lock()
	cm.provisioningState = StateReady
	cm.lastError = nil
	cm.retryCount = 0
	cm.mu.Unlock()

	return nil
}

// doEstablishConnection performs the actual connection establishment
func (cm *ConnectionManager) doEstablishConnection(ctx context.Context) error {
	// Check if SSH key exists before proceeding
	sshKeyPath := expandPath(cm.sshConfig.KeyPath)
	if _, err := os.Stat(sshKeyPath); os.IsNotExist(err) {
		cm.logger.WithFields(map[string]interface{}{
			"ssh_key_path": sshKeyPath,
		}).Info("SSH private key not found, attempting to generate it")

		if err := cm.generateSSHKey(sshKeyPath); err != nil {
			return errors.Errorf("SSH private key not found at %s and failed to generate it: %v\nPlease generate an SSH key pair manually:\n  ssh-keygen -t rsa -b 4096 -f %s", sshKeyPath, err, sshKeyPath)
		}
	}

	// Get or provision a server
	server, err := cm.serverManager.GetOrProvisionServer(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get or provision server")
	}

	// Create SSH client
	sshConfig := &ssh.ClientConfig{
		Host:           server.IPAddress,
		Port:           cm.sshConfig.Port,
		User:           "root",
		PrivateKeyPath: sshKeyPath,
		Timeout:        cm.sshConfig.Timeout,
	}

	cm.sshClient = ssh.NewClient(sshConfig)

	// Connect to SSH server with a separate timeout
	connectCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := cm.sshClient.Connect(connectCtx); err != nil {
		return errors.Wrap(err, "failed to connect to SSH server")
	}

	// Create SSH tunnel for Docker API (port 2376 on remote)
	localAddr := "127.0.0.1:0" // Use random available port
	remoteAddr := "127.0.0.1:2376"

	tunnelCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cm.tunnel, err = cm.sshClient.CreateTunnel(tunnelCtx, localAddr, remoteAddr)
	if err != nil {
		return errors.Wrap(err, "failed to create SSH tunnel")
	}

	cm.logger.WithFields(map[string]interface{}{
		"local_addr":  cm.tunnel.LocalAddr(),
		"remote_addr": remoteAddr,
		"server_ip":   server.IPAddress,
	}).Info("SSH tunnel established")

	return nil
}

// getBackoffDuration calculates exponential backoff duration
func (cm *ConnectionManager) getBackoffDuration(retryCount int) time.Duration {
	// Exponential backoff: 1s, 2s, 4s, 8s, 16s, max 60s
	duration := time.Duration(1<<uint(retryCount)) * time.Second
	if duration > 60*time.Second {
		duration = 60 * time.Second
	}
	return duration
}

// GetProvisioningState returns the current provisioning state
func (cm *ConnectionManager) GetProvisioningState() (ProvisioningState, error, int) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.provisioningState, cm.lastError, cm.retryCount
}

// GetTunnel returns the current SSH tunnel
func (cm *ConnectionManager) GetTunnel() ssh.TunnelInterface {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.tunnel
}

// GetHTTPClient returns the HTTP client for connection pooling
func (cm *ConnectionManager) GetHTTPClient() *http.Client {
	return cm.connPool.client
}

// IsConnected returns true if we have an active connection
func (cm *ConnectionManager) IsConnected() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.sshClient != nil && cm.sshClient.IsConnected() && cm.tunnel != nil
}

// Close closes all connections and cleans up resources
func (cm *ConnectionManager) Close() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Close SSH tunnel if exists
	if cm.tunnel != nil {
		if err := cm.tunnel.Close(); err != nil {
			cm.logger.WithFields(map[string]interface{}{
				"error": err.Error(),
			}).Error("Failed to close SSH tunnel")
		}
		cm.tunnel = nil
	}

	// Close SSH client if exists
	if cm.sshClient != nil {
		if err := cm.sshClient.Close(); err != nil {
			cm.logger.WithFields(map[string]interface{}{
				"error": err.Error(),
			}).Error("Failed to close SSH client")
		}
		cm.sshClient = nil
	}

	// Close connection pool
	if cm.connPool != nil && cm.connPool.transport != nil {
		cm.connPool.transport.CloseIdleConnections()
	}

	return nil
}

// generateSSHKey generates an SSH key pair if it doesn't exist
func (cm *ConnectionManager) generateSSHKey(keyPath string) error {
	// Create directory if it doesn't exist
	keyDir := filepath.Dir(keyPath)
	if err := os.MkdirAll(keyDir, 0700); err != nil {
		return errors.Wrap(err, "failed to create SSH key directory")
	}

	// Generate SSH key using ssh-keygen
	cmd := exec.Command("ssh-keygen", "-t", "rsa", "-b", "4096", "-f", keyPath, "-N", "")
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "failed to generate SSH key")
	}

	cm.logger.WithFields(map[string]interface{}{
		"private_key": keyPath,
		"public_key":  keyPath + ".pub",
	}).Info("Generated SSH key pair")

	return nil
}
