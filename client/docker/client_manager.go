package docker

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dockbridge/dockbridge/client/ssh"
	"github.com/dockbridge/dockbridge/internal/client/hetzner"
	"github.com/dockbridge/dockbridge/internal/shared/config"
	"github.com/dockbridge/dockbridge/pkg/logger"
	"github.com/docker/docker/client"
	"github.com/pkg/errors"
)

// DockerClientManager manages Docker client connections over SSH tunnels
// This is a simplified approach that replaces the complex HTTP proxy layer with
// direct Docker Go client usage over SSH tunnels. Key simplifications:
// - No complex connection pooling, just simple SSH tunnel per server
// - Automatic server provisioning when Docker client connection fails
// - Direct Docker daemon access via SSH tunnel without proxy layer
// - Context-aware operations for proper cancellation and timeouts
type DockerClientManager interface {
	GetClient(ctx context.Context) (*client.Client, error)
	EnsureConnection(ctx context.Context) error
	GetTunnel() ssh.TunnelInterface
	Close() error
}

// dockerClientManagerImpl implements DockerClientManager
type dockerClientManagerImpl struct {
	hetznerClient   hetzner.HetznerClient
	sshConfig       *config.SSHConfig
	hetznerConfig   *config.HetznerConfig
	logger          logger.LoggerInterface
	activityTracker ActivityTracker // Interface for activity tracking

	// Current connection state
	currentServer *hetzner.Server
	sshClient     ssh.Client
	tunnel        ssh.TunnelInterface
	dockerClient  *client.Client

	// Synchronization to prevent race conditions
	mu sync.Mutex
}

// ActivityTracker interface for recording Docker activity
type ActivityTracker interface {
	RecordDockerCommand() error
	RecordConnectionActivity() error
}

// NewDockerClientManager creates a new Docker client manager
func NewDockerClientManager(hetznerClient hetzner.HetznerClient, sshConfig *config.SSHConfig, hetznerConfig *config.HetznerConfig, logger logger.LoggerInterface) DockerClientManager {
	return &dockerClientManagerImpl{
		hetznerClient: hetznerClient,
		sshConfig:     sshConfig,
		hetznerConfig: hetznerConfig,
		logger:        logger,
	}
}

// NewDockerClientManagerWithActivity creates a new Docker client manager with activity tracking
func NewDockerClientManagerWithActivity(hetznerClient hetzner.HetznerClient, sshConfig *config.SSHConfig, hetznerConfig *config.HetznerConfig, logger logger.LoggerInterface, activityTracker ActivityTracker) DockerClientManager {
	return &dockerClientManagerImpl{
		hetznerClient:   hetznerClient,
		sshConfig:       sshConfig,
		hetznerConfig:   hetznerConfig,
		logger:          logger,
		activityTracker: activityTracker,
	}
}

// GetClient returns a Docker client connected to the remote server via SSH tunnel
func (dcm *dockerClientManagerImpl) GetClient(ctx context.Context) (*client.Client, error) {
	// Record Docker command activity
	if dcm.activityTracker != nil {
		if err := dcm.activityTracker.RecordDockerCommand(); err != nil {
			dcm.logger.WithFields(map[string]interface{}{
				"error": err.Error(),
			}).Warn("Failed to record Docker command activity")
		}
	}

	// Ensure we have a connection first
	if err := dcm.EnsureConnection(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to ensure connection")
	}

	// Return the existing client if available
	if dcm.dockerClient != nil {
		return dcm.dockerClient, nil
	}

	// Create Docker client using the helper method
	dockerClient, err := dcm.createDockerClient()
	if err != nil {
		return nil, err
	}

	dcm.dockerClient = dockerClient
	dcm.logger.WithFields(map[string]interface{}{
		"tunnel_addr": dcm.tunnel.LocalAddr(),
		"server_ip":   dcm.currentServer.IPAddress,
	}).Debug("Docker client created successfully")

	return dcm.dockerClient, nil
}

// EnsureConnection ensures we have an active connection to a remote server with Docker ready
func (dcm *dockerClientManagerImpl) EnsureConnection(ctx context.Context) error {
	// Record connection activity
	if dcm.activityTracker != nil {
		if err := dcm.activityTracker.RecordConnectionActivity(); err != nil {
			dcm.logger.WithFields(map[string]interface{}{
				"error": err.Error(),
			}).Warn("Failed to record connection activity")
		}
	}

	// Use mutex to prevent race conditions during server provisioning
	dcm.mu.Lock()
	defer dcm.mu.Unlock()

	// Check if we already have an active connection (double-check after acquiring lock)
	if dcm.isConnectionHealthy() {
		dcm.logger.Debug("Using existing healthy connection")
		return nil
	}

	dcm.logger.Info("🚀 Establishing connection to remote Docker server...")

	// Clean up any existing connection
	dcm.cleanup()

	// Get or provision a server with progress feedback
	server, err := dcm.getOrProvisionServerWithProgress(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get or provision server")
	}

	dcm.currentServer = server

	// Establish SSH connection with enhanced retry logic
	if err := dcm.establishSSHConnection(ctx, server); err != nil {
		// If SSH connection fails, the server might be deleted or corrupted
		// Try to recreate it once
		dcm.logger.WithFields(map[string]interface{}{
			"server_id": server.ID,
			"error":     err.Error(),
		}).Warn("SSH connection failed, attempting server recreation")

		// Clean up the failed server
		go dcm.cleanupStaleServers(context.Background(), []*hetzner.Server{server})

		// Try to provision a new server
		server, err = dcm.provisionNewServerWithProgress(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to provision new server after SSH failure")
		}

		dcm.currentServer = server

		// Try SSH connection again with the new server
		if err := dcm.establishSSHConnection(ctx, server); err != nil {
			return errors.Wrap(err, "failed to establish SSH connection even after server recreation")
		}
	}

	// Create SSH tunnel for Docker API
	if err := dcm.createDockerTunnel(ctx); err != nil {
		dcm.cleanup()
		return errors.Wrap(err, "failed to create Docker tunnel")
	}

	// Verify Docker daemon is ready and responsive
	if err := dcm.verifyDockerDaemonReady(ctx); err != nil {
		dcm.cleanup()
		return errors.Wrap(err, "Docker daemon is not ready")
	}

	dcm.logger.WithFields(map[string]interface{}{
		"server_ip":   dcm.currentServer.IPAddress,
		"tunnel_addr": dcm.tunnel.LocalAddr(),
	}).Info("✅ Remote Docker connection established and verified")

	return nil
}

// GetTunnel returns the current SSH tunnel
func (dcm *dockerClientManagerImpl) GetTunnel() ssh.TunnelInterface {
	return dcm.tunnel
}

// Close closes all connections and cleans up resources
func (dcm *dockerClientManagerImpl) Close() error {
	dcm.logger.Info("Closing Docker client manager")
	dcm.cleanup()
	dcm.logger.Info("Docker client manager closed")
	return nil
}

// establishSSHConnection creates and tests SSH connection to the server
func (dcm *dockerClientManagerImpl) establishSSHConnection(ctx context.Context, server *hetzner.Server) error {
	dcm.logger.WithFields(map[string]interface{}{
		"server_ip": server.IPAddress,
	}).Info("🔗 Establishing SSH connection...")

	// Create SSH client
	sshKeyPath := expandPath(dcm.sshConfig.KeyPath)
	sshConfig := &ssh.ClientConfig{
		Host:           server.IPAddress,
		Port:           dcm.sshConfig.Port,
		User:           "root",
		PrivateKeyPath: sshKeyPath,
		Timeout:        60 * time.Second,
	}

	dcm.sshClient = ssh.NewClient(sshConfig)

	// Connect to SSH server with enhanced retry logic
	var connectErr error
	maxRetries := 5 // Increased retries for better reliability
	for attempt := 1; attempt <= maxRetries; attempt++ {
		connectCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		connectErr = dcm.sshClient.Connect(connectCtx)
		cancel()

		if connectErr == nil {
			dcm.logger.WithFields(map[string]interface{}{
				"server_ip": server.IPAddress,
				"attempt":   attempt,
			}).Info("✅ SSH connection established")
			return nil
		}

		dcm.logger.WithFields(map[string]interface{}{
			"attempt":   attempt,
			"max_tries": maxRetries,
			"error":     connectErr.Error(),
			"server_ip": server.IPAddress,
		}).Warn("⚠️ SSH connection attempt failed, retrying...")

		if attempt < maxRetries {
			// Progressive backoff: 2s, 4s, 6s, 8s
			sleepDuration := time.Duration(attempt*2) * time.Second
			time.Sleep(sleepDuration)
		}
	}

	return errors.Wrapf(connectErr, "failed to connect to SSH server %s after %d retries", server.IPAddress, maxRetries)
}

// createDockerTunnel creates SSH tunnel for Docker API access
func (dcm *dockerClientManagerImpl) createDockerTunnel(ctx context.Context) error {
	dcm.logger.Info("🚇 Creating SSH tunnel for Docker API...")

	localAddr := "127.0.0.1:0" // Use random available port
	remoteAddr := "127.0.0.1:2376"

	tunnelCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	tunnel, err := dcm.sshClient.CreateTunnel(tunnelCtx, localAddr, remoteAddr)
	if err != nil {
		return errors.Wrap(err, "failed to create SSH tunnel")
	}

	dcm.tunnel = tunnel

	dcm.logger.WithFields(map[string]interface{}{
		"local_addr":  dcm.tunnel.LocalAddr(),
		"remote_addr": remoteAddr,
		"server_ip":   dcm.currentServer.IPAddress,
	}).Info("✅ SSH tunnel created successfully")

	return nil
}

// verifyDockerDaemonReady verifies that Docker daemon is ready and responsive
func (dcm *dockerClientManagerImpl) verifyDockerDaemonReady(ctx context.Context) error {
	dcm.logger.Info("🐳 Verifying Docker daemon is ready...")

	// Create Docker client if not exists (with proper nil checking)
	// Note: This method is called from EnsureConnection which already holds the mutex
	if dcm.dockerClient == nil {
		dockerClient, err := dcm.createDockerClient()
		if err != nil {
			return errors.Wrap(err, "failed to create Docker client")
		}
		if dockerClient == nil {
			return errors.New("createDockerClient returned nil client without error")
		}
		dcm.dockerClient = dockerClient
	}

	// Double-check that dockerClient is not nil before using it
	if dcm.dockerClient == nil {
		return errors.New("Docker client is nil after creation attempt")
	}

	// Try to ping Docker daemon with retries
	maxRetries := 10
	for attempt := 1; attempt <= maxRetries; attempt++ {
		pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)

		// Additional nil check right before calling Ping
		if dcm.dockerClient == nil {
			cancel()
			return errors.New("Docker client became nil during ping attempts")
		}

		_, err := dcm.dockerClient.Ping(pingCtx)
		cancel()

		if err == nil {
			dcm.logger.WithFields(map[string]interface{}{
				"attempt": attempt,
			}).Info("✅ Docker daemon is ready and responsive")
			return nil
		}

		dcm.logger.WithFields(map[string]interface{}{
			"attempt":   attempt,
			"max_tries": maxRetries,
			"error":     err.Error(),
		}).Debug("Docker daemon not ready yet, retrying...")

		if attempt < maxRetries {
			// Wait 3 seconds between attempts
			time.Sleep(3 * time.Second)
		}
	}

	return errors.New("Docker daemon failed to become ready after maximum retries")
}

// createDockerClient creates a new Docker client connected via SSH tunnel
func (dcm *dockerClientManagerImpl) createDockerClient() (*client.Client, error) {
	if dcm.tunnel == nil {
		return nil, errors.New("no SSH tunnel available")
	}

	dockerClient, err := client.NewClientWithOpts(
		client.WithHost(fmt.Sprintf("tcp://%s", dcm.tunnel.LocalAddr())),
		client.WithAPIVersionNegotiation(),
		client.WithHTTPClient(&http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					// Always dial to our tunnel's local address
					dialer := &net.Dialer{
						Timeout:   30 * time.Second,
						KeepAlive: 30 * time.Second,
					}
					return dialer.DialContext(ctx, "tcp", dcm.tunnel.LocalAddr())
				},
				MaxIdleConns:          10,
				IdleConnTimeout:       60 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
			},
			Timeout: 0, // No timeout - rely on context cancellation
		}),
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create Docker client")
	}

	return dockerClient, nil
}

// isConnectionHealthy checks if the current connection is healthy
func (dcm *dockerClientManagerImpl) isConnectionHealthy() bool {
	if dcm.sshClient == nil || !dcm.sshClient.IsConnected() || dcm.tunnel == nil {
		return false
	}

	// Quick health check - try to ping Docker daemon
	if dcm.dockerClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := dcm.dockerClient.Ping(ctx)
		if err != nil {
			dcm.logger.WithFields(map[string]interface{}{
				"error": err.Error(),
			}).Debug("Docker ping failed, connection unhealthy")
			return false
		}
	}

	return true
}

// cleanup closes all connections without locking
func (dcm *dockerClientManagerImpl) cleanup() {
	if dcm.dockerClient != nil {
		dcm.dockerClient.Close()
		dcm.dockerClient = nil
	}

	if dcm.tunnel != nil {
		dcm.tunnel.Close()
		dcm.tunnel = nil
	}

	if dcm.sshClient != nil {
		dcm.sshClient.Close()
		dcm.sshClient = nil
	}

	dcm.currentServer = nil
}

// getOrProvisionServerWithProgress gets an existing server or provisions a new one with progress feedback
func (dcm *dockerClientManagerImpl) getOrProvisionServerWithProgress(ctx context.Context) (*hetzner.Server, error) {
	dcm.logger.Info("🔍 Searching for existing DockBridge servers...")
	return dcm.getOrProvisionServer(ctx)
}

// getOrProvisionServer gets an existing server or provisions a new one
func (dcm *dockerClientManagerImpl) getOrProvisionServer(ctx context.Context) (*hetzner.Server, error) {
	// First, try to find an existing DockBridge server
	servers, err := dcm.hetznerClient.ListServers(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list servers")
	}

	dcm.logger.WithFields(map[string]interface{}{
		"total_servers": len(servers),
	}).Debug("Listed servers from Hetzner")

	// Look for running DockBridge servers
	var runningServers []*hetzner.Server
	var staleServers []*hetzner.Server

	for _, server := range servers {
		if strings.HasPrefix(server.Name, "dockbridge-") {
			dcm.logger.WithFields(map[string]interface{}{
				"server_id":   server.ID,
				"server_name": server.Name,
				"server_ip":   server.IPAddress,
				"status":      server.Status,
			}).Debug("Found DockBridge server")

			if server.Status == "running" {
				runningServers = append(runningServers, server)
			} else {
				staleServers = append(staleServers, server)
			}
		}
	}

	// Clean up stale servers in background
	if len(staleServers) > 0 {
		go dcm.cleanupStaleServers(context.Background(), staleServers)
	}

	// If we have running servers, use the first one (cleanup extras in background)
	if len(runningServers) > 0 {
		selectedServer := runningServers[0]

		// Clean up extra servers in background
		if len(runningServers) > 1 {
			go dcm.cleanupStaleServers(context.Background(), runningServers[1:])
		}

		dcm.logger.WithFields(map[string]interface{}{
			"server_id": selectedServer.ID,
			"server_ip": selectedServer.IPAddress,
		}).Info("Using existing DockBridge server")

		return selectedServer, nil
	}

	// No running server found, provision a new one
	dcm.logger.Info("No running server found, provisioning new server")
	return dcm.provisionNewServerWithProgress(ctx)
}

// provisionNewServerWithProgress creates a new Hetzner server with Docker CE and progress feedback
func (dcm *dockerClientManagerImpl) provisionNewServerWithProgress(ctx context.Context) (*hetzner.Server, error) {
	dcm.logger.Info("🏗️ Provisioning new DockBridge server (this may take 2-3 minutes)...")
	return dcm.provisionNewServer(ctx)
}

// provisionNewServer creates a new Hetzner server with Docker CE and persistent volume
func (dcm *dockerClientManagerImpl) provisionNewServer(ctx context.Context) (*hetzner.Server, error) {
	// Generate server name with timestamp
	serverName := fmt.Sprintf("dockbridge-%d", time.Now().Unix())

	// Get SSH public key
	sshKeyPath := expandPath(dcm.sshConfig.KeyPath)
	publicKeyPath := sshKeyPath + ".pub"

	publicKeyBytes, err := dcm.readOrGenerateSSHKey(sshKeyPath, publicKeyPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get SSH key")
	}

	publicKeyContent := string(publicKeyBytes)

	// Find or create persistent volume for Docker data
	dcm.logger.Info("🗄️ Setting up persistent volume for Docker data...")
	volume, err := dcm.hetznerClient.FindOrCreateDockerVolume(ctx, dcm.hetznerConfig.Location)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create or find Docker volume")
	}

	dcm.logger.WithFields(map[string]interface{}{
		"volume_id":       volume.ID,
		"volume_name":     volume.Name,
		"volume_size":     volume.Size,
		"volume_location": volume.Location,
		"volume_status":   volume.Status,
	}).Info("Docker volume ready for attachment")

	// Generate cloud-init script with persistent volume mounting
	cloudInitConfig := &hetzner.CloudInitConfig{
		SSHPublicKey:  publicKeyContent,
		VolumeMount:   "/var/lib/docker", // Mount volume directly at Docker data directory
		DockerAPIPort: 2376,
		KeepAlivePort: 8080,
	}
	cloudInitScript := hetzner.GenerateCloudInitScript(cloudInitConfig)

	// Upload SSH key to Hetzner
	sshKey, err := dcm.hetznerClient.ManageSSHKeys(ctx, publicKeyContent)
	if err != nil {
		return nil, errors.Wrap(err, "failed to manage SSH key with Hetzner")
	}

	volumeIDStr := fmt.Sprintf("%d", volume.ID)
	serverConfig := &hetzner.ServerConfig{
		Name:       serverName,
		ServerType: dcm.hetznerConfig.ServerType,
		Location:   dcm.hetznerConfig.Location,
		UserData:   cloudInitScript,
		SSHKeyID:   sshKey.ID,
		VolumeID:   volumeIDStr,
	}

	dcm.logger.WithFields(map[string]interface{}{
		"server_name": serverName,
		"volume_id":   volumeIDStr,
		"ssh_key_id":  sshKey.ID,
	}).Info("Creating server with volume attachment")

	server, err := dcm.hetznerClient.ProvisionServer(ctx, serverConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to provision server")
	}

	dcm.logger.WithFields(map[string]interface{}{
		"server_id":   server.ID,
		"server_name": server.Name,
		"server_ip":   server.IPAddress,
	}).Info("New server provisioned successfully")

	// Volume should already be attached during server creation via VolumeID in ServerConfig
	// No need for explicit attachment if it was specified during creation
	dcm.logger.WithFields(map[string]interface{}{
		"server_id": server.ID,
		"volume_id": volume.ID,
	}).Info("Docker volume should be attached via server creation")

	// Wait for server to be ready
	if err := dcm.waitForServerReady(ctx, server); err != nil {
		// Clean up failed server in background
		go dcm.cleanupStaleServers(context.Background(), []*hetzner.Server{server})
		return nil, errors.Wrap(err, "server provisioned but not ready")
	}

	return server, nil
}

// readOrGenerateSSHKey reads existing SSH key or generates a new one
func (dcm *dockerClientManagerImpl) readOrGenerateSSHKey(privateKeyPath, publicKeyPath string) ([]byte, error) {
	// Try to read existing public key
	if publicKeyBytes, err := os.ReadFile(publicKeyPath); err == nil {
		return publicKeyBytes, nil
	}

	// Generate new SSH key pair if it doesn't exist
	dcm.logger.WithFields(map[string]interface{}{
		"private_key_path": privateKeyPath,
		"public_key_path":  publicKeyPath,
	}).Info("SSH key not found, generating new key pair")

	if err := dcm.generateSSHKey(privateKeyPath); err != nil {
		return nil, errors.Wrap(err, "failed to generate SSH key")
	}

	// Read the newly generated public key
	return os.ReadFile(publicKeyPath)
}

// generateSSHKey generates an SSH key pair
func (dcm *dockerClientManagerImpl) generateSSHKey(keyPath string) error {
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

	dcm.logger.WithFields(map[string]interface{}{
		"private_key": keyPath,
		"public_key":  keyPath + ".pub",
	}).Info("Generated SSH key pair")

	return nil
}

// waitForServerReady waits for the server to be fully configured and Docker to be running
func (dcm *dockerClientManagerImpl) waitForServerReady(ctx context.Context, server *hetzner.Server) error {
	dcm.logger.WithFields(map[string]interface{}{
		"server_id": server.ID,
	}).Info("⏳ Waiting for server setup to complete...")

	timeout := time.After(10 * time.Minute) // Increased timeout for reliability
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	attempt := 0
	startTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			elapsed := time.Since(startTime)
			dcm.logger.WithFields(map[string]interface{}{
				"server_id": server.ID,
				"attempts":  attempt,
				"elapsed":   elapsed.String(),
			}).Error("❌ Timeout waiting for server to be ready")
			return errors.New("timeout waiting for server to be ready after 10 minutes")
		case <-ticker.C:
			attempt++
			elapsed := time.Since(startTime)

			// Show progress every 30 seconds
			if attempt%2 == 0 {
				dcm.logger.WithFields(map[string]interface{}{
					"server_id": server.ID,
					"elapsed":   elapsed.Truncate(time.Second).String(),
				}).Info("⏳ Still waiting for server setup (installing Docker, configuring services)...")
			}

			if dcm.checkServerReady(ctx, server) {
				dcm.logger.WithFields(map[string]interface{}{
					"server_id": server.ID,
					"attempts":  attempt,
					"elapsed":   elapsed.Truncate(time.Second).String(),
				}).Info("✅ Server is ready and Docker daemon is running")
				return nil
			}
		}
	}
}

// checkServerReady checks if the server is ready by attempting to connect and verify Docker
func (dcm *dockerClientManagerImpl) checkServerReady(ctx context.Context, server *hetzner.Server) bool {
	sshKeyPath := expandPath(dcm.sshConfig.KeyPath)
	sshConfig := &ssh.ClientConfig{
		Host:           server.IPAddress,
		Port:           dcm.sshConfig.Port,
		User:           "root",
		PrivateKeyPath: sshKeyPath,
		Timeout:        15 * time.Second,
	}

	tempSSHClient := ssh.NewClient(sshConfig)
	defer tempSSHClient.Close()

	checkCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	// Try to connect
	if err := tempSSHClient.Connect(checkCtx); err != nil {
		dcm.logger.WithFields(map[string]interface{}{
			"server_id": server.ID,
			"server_ip": server.IPAddress,
			"error":     err.Error(),
		}).Debug("🔍 Server not ready - SSH connection failed")
		return false
	}

	// Check if Docker is running and accessible on port 2376
	dockerCheckCmd := "docker version --format '{{.Server.Version}}' && curl -s http://localhost:2376/version >/dev/null"
	output, err := tempSSHClient.ExecuteCommand(checkCtx, dockerCheckCmd)
	if err != nil {
		dcm.logger.WithFields(map[string]interface{}{
			"server_id": server.ID,
			"error":     err.Error(),
		}).Debug("🔍 Server not ready - Docker daemon not fully configured")
		return false
	}

	dockerVersion := strings.TrimSpace(string(output))
	dcm.logger.WithFields(map[string]interface{}{
		"server_id":      server.ID,
		"docker_version": dockerVersion,
	}).Debug("✅ Server ready - Docker daemon is responding on both CLI and API")

	return true
}

// cleanupStaleServers removes stale or duplicate servers in the background
func (dcm *dockerClientManagerImpl) cleanupStaleServers(ctx context.Context, servers []*hetzner.Server) {
	for _, server := range servers {
		dcm.logger.WithFields(map[string]interface{}{
			"server_id":   server.ID,
			"server_name": server.Name,
			"status":      server.Status,
		}).Info("Cleaning up stale DockBridge server")

		serverID := fmt.Sprintf("%d", server.ID)
		if err := dcm.hetznerClient.DestroyServer(ctx, serverID); err != nil {
			dcm.logger.WithFields(map[string]interface{}{
				"server_id": server.ID,
				"error":     err.Error(),
			}).Error("Failed to cleanup stale server")
		} else {
			dcm.logger.WithFields(map[string]interface{}{
				"server_id": server.ID,
			}).Info("Successfully cleaned up stale server")
		}
	}
}
