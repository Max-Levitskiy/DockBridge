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
	"time"

	"github.com/dockbridge/dockbridge/client/hetzner"
	"github.com/dockbridge/dockbridge/client/monitor"
	"github.com/dockbridge/dockbridge/client/portforward"
	"github.com/dockbridge/dockbridge/client/ssh"
	"github.com/dockbridge/dockbridge/pkg/logger"
	"github.com/dockbridge/dockbridge/shared/config"
	"github.com/docker/docker/client"
	"github.com/pkg/errors"
)

// DockerContainerCreateResponse represents a Docker container creation response
type DockerContainerCreateResponse struct {
	ID       string                   `json:"Id"`
	Warnings []string                 `json:"Warnings"`
	Ports    map[string][]PortBinding `json:"Ports,omitempty"`
}

// PortBinding represents a Docker port binding
type PortBinding struct {
	HostIP   string `json:"HostIp"`
	HostPort string `json:"HostPort"`
}

// DockerErrorResponse represents a Docker API error response
type DockerErrorResponse struct {
	Message string `json:"message"`
}

// DockerClientManager manages Docker client connections over SSH tunnels
// This is a simplified approach that replaces the complex HTTP proxy layer with
// direct Docker Go client usage over SSH tunnels. Key simplifications:
// - No complex connection pooling, just simple SSH tunnel per server
// - Automatic server provisioning when Docker client connection fails
// - Direct Docker daemon access via SSH tunnel without proxy layer
// - Context-aware operations for proper cancellation and timeouts
// - Integrated container monitoring and port forwarding
type DockerClientManager interface {
	GetClient(ctx context.Context) (*client.Client, error)
	EnsureConnection(ctx context.Context) error
	Close() error

	// Tunnel access for direct socket forwarding
	GetTunnel() ssh.TunnelInterface

	// Port forwarding integration
	RegisterContainerEventHandler(handler monitor.ContainerEventHandler) error
	StartPortForwarding(ctx context.Context) error
	StopPortForwarding() error
	GetPortForwardManager() portforward.PortForwardManager

	// Docker API response interception
	InterceptDockerResponse(response []byte) ([]byte, error)
}

// dockerClientManagerImpl implements DockerClientManager
type dockerClientManagerImpl struct {
	hetznerClient hetzner.HetznerClient
	sshConfig     *config.SSHConfig
	hetznerConfig *config.HetznerConfig
	logger        logger.LoggerInterface

	// Current connection state
	currentServer *hetzner.Server
	sshClient     ssh.Client
	tunnel        ssh.TunnelInterface
	dockerClient  *client.Client

	// Port forwarding components
	containerMonitor   monitor.ContainerMonitor
	portForwardManager portforward.PortForwardManager
	portForwardConfig  *config.PortForwardConfig

	// Activity tracking (optional)
	activityTracker any
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

// NewDockerClientManagerWithPortForwarding creates a new Docker client manager with port forwarding support
func NewDockerClientManagerWithPortForwarding(hetznerClient hetzner.HetznerClient, sshConfig *config.SSHConfig, hetznerConfig *config.HetznerConfig, portForwardConfig *config.PortForwardConfig, logger logger.LoggerInterface) DockerClientManager {
	return &dockerClientManagerImpl{
		hetznerClient:     hetznerClient,
		sshConfig:         sshConfig,
		hetznerConfig:     hetznerConfig,
		portForwardConfig: portForwardConfig,
		logger:            logger,
	}
}

// NewDockerClientManagerWithActivity creates a new Docker client manager with activity tracking support
func NewDockerClientManagerWithActivity(hetznerClient hetzner.HetznerClient, sshConfig *config.SSHConfig, hetznerConfig *config.HetznerConfig, logger logger.LoggerInterface, activityTracker any) DockerClientManager {
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
	// Ensure we have a connection first
	if err := dcm.EnsureConnection(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to ensure connection")
	}

	// Return the existing client if available
	if dcm.dockerClient != nil {
		return dcm.dockerClient, nil
	}

	// Create Docker client pointing to SSH tunnel
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

	dcm.dockerClient = dockerClient
	dcm.logger.WithFields(map[string]any{
		"tunnel_addr": dcm.tunnel.LocalAddr(),
		"server_ip":   dcm.currentServer.IPAddress,
	}).Info("Docker client created successfully")

	return dcm.dockerClient, nil
}

// EnsureConnection ensures we have an active connection to a remote server
func (dcm *dockerClientManagerImpl) EnsureConnection(ctx context.Context) error {
	// Check if we already have an active connection
	if dcm.isConnectionHealthy() {
		dcm.logger.Debug("Using existing connection")
		return nil
	}

	dcm.logger.Info("Establishing connection to remote server")

	// Clean up any existing connection
	dcm.cleanup()

	// Get or provision a server
	server, err := dcm.getOrProvisionServer(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get or provision server")
	}

	dcm.currentServer = server

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

	// Connect to SSH server with retry logic
	var connectErr error
	maxRetries := 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		connectCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
		connectErr = dcm.sshClient.Connect(connectCtx)
		cancel()

		if connectErr == nil {
			break
		}

		dcm.logger.WithFields(map[string]any{
			"attempt":   attempt,
			"max_tries": maxRetries,
			"error":     connectErr.Error(),
			"server_ip": server.IPAddress,
		}).Warn("SSH connection attempt failed, retrying")

		if attempt < maxRetries {
			time.Sleep(time.Duration(attempt*2) * time.Second)
		}
	}

	if connectErr != nil {
		return errors.Wrap(connectErr, "failed to connect to SSH server after retries")
	}

	dcm.logger.WithFields(map[string]any{
		"server_ip": server.IPAddress,
	}).Info("SSH connection established successfully")

	// Create SSH tunnel for Docker API
	localAddr := "127.0.0.1:0" // Use random available port
	remoteAddr := "127.0.0.1:2376"

	tunnelCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	dcm.tunnel, err = dcm.sshClient.CreateTunnel(tunnelCtx, localAddr, remoteAddr)
	if err != nil {
		dcm.cleanup()
		return errors.Wrap(err, "failed to create SSH tunnel")
	}

	dcm.logger.WithFields(map[string]any{
		"local_addr":  dcm.tunnel.LocalAddr(),
		"remote_addr": remoteAddr,
		"server_ip":   server.IPAddress,
	}).Info("SSH tunnel established")

	return nil
}

// Close closes all connections and cleans up resources
func (dcm *dockerClientManagerImpl) Close() error {
	dcm.logger.Info("Closing Docker client manager")

	// Stop port forwarding first
	if err := dcm.StopPortForwarding(); err != nil {
		dcm.logger.WithFields(map[string]any{
			"error": err.Error(),
		}).Error("Error stopping port forwarding during close")
	}

	dcm.cleanup()
	dcm.logger.Info("Docker client manager closed")
	return nil
}

// GetTunnel returns the current SSH tunnel for direct socket forwarding
func (dcm *dockerClientManagerImpl) GetTunnel() ssh.TunnelInterface {
	return dcm.tunnel
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
			dcm.logger.WithFields(map[string]any{
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

// getOrProvisionServer gets an existing server or provisions a new one
func (dcm *dockerClientManagerImpl) getOrProvisionServer(ctx context.Context) (*hetzner.Server, error) {
	// First, try to find an existing DockBridge server
	servers, err := dcm.hetznerClient.ListServers(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list servers")
	}

	dcm.logger.WithFields(map[string]any{
		"total_servers": len(servers),
	}).Debug("Listed servers from Hetzner")

	// Look for running DockBridge servers
	var runningServers []*hetzner.Server
	var staleServers []*hetzner.Server

	for _, server := range servers {
		if strings.HasPrefix(server.Name, "dockbridge-") {
			dcm.logger.WithFields(map[string]any{
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

		dcm.logger.WithFields(map[string]any{
			"server_id": selectedServer.ID,
			"server_ip": selectedServer.IPAddress,
		}).Info("Using existing DockBridge server")

		return selectedServer, nil
	}

	// No running server found, provision a new one
	dcm.logger.Info("No running server found, provisioning new server")
	return dcm.provisionNewServer(ctx)
}

// provisionNewServer creates a new Hetzner server with Docker CE
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

	// Create cloud-init script for Docker CE installation
	cloudInitScript := fmt.Sprintf(`#!/bin/bash
set -e

# Log all output
exec > >(tee -a /var/log/dockbridge-setup.log)
exec 2>&1

echo "$(date): Starting DockBridge server setup"

# Update system
echo "$(date): Updating system packages"
apt-get update
apt-get upgrade -y

# Add SSH public key to root user
echo "$(date): Setting up SSH access"
mkdir -p /root/.ssh
echo "%s" >> /root/.ssh/authorized_keys
chmod 600 /root/.ssh/authorized_keys
chmod 700 /root/.ssh

# Install Docker CE
echo "$(date): Installing Docker CE"
curl -fsSL https://get.docker.com -o get-docker.sh
sh get-docker.sh

# Enable Docker service
echo "$(date): Enabling Docker service"
systemctl enable docker
systemctl start docker

# Wait for Docker to be ready
echo "$(date): Waiting for Docker daemon to be ready"
for i in {1..30}; do
    if docker version >/dev/null 2>&1; then
        echo "$(date): Docker daemon is ready"
        break
    fi
    echo "$(date): Waiting for Docker daemon... attempt $i/30"
    sleep 2
done

# Configure Docker daemon to listen on TCP port 2376
echo "$(date): Configuring Docker daemon for TCP access"
mkdir -p /etc/systemd/system/docker.service.d
cat > /etc/systemd/system/docker.service.d/override.conf << EOF
[Service]
ExecStart=
ExecStart=/usr/bin/dockerd -H fd:// -H tcp://0.0.0.0:2376
EOF

# Reload systemd and restart Docker
echo "$(date): Restarting Docker with new configuration"
systemctl daemon-reload
systemctl restart docker

# Wait for Docker to be ready again
echo "$(date): Waiting for Docker daemon to be ready after restart"
for i in {1..30}; do
    if docker version >/dev/null 2>&1; then
        echo "$(date): Docker daemon is ready after restart"
        break
    fi
    echo "$(date): Waiting for Docker daemon after restart... attempt $i/30"
    sleep 2
done

echo "$(date): DockBridge server setup completed successfully"
`, publicKeyContent)

	// Upload SSH key to Hetzner
	sshKey, err := dcm.hetznerClient.ManageSSHKeys(ctx, publicKeyContent)
	if err != nil {
		return nil, errors.Wrap(err, "failed to manage SSH key with Hetzner")
	}

	serverConfig := &hetzner.ServerConfig{
		Name:       serverName,
		ServerType: dcm.hetznerConfig.ServerType,
		Location:   dcm.hetznerConfig.Location,
		UserData:   cloudInitScript,
		SSHKeyID:   sshKey.ID,
	}

	server, err := dcm.hetznerClient.ProvisionServer(ctx, serverConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to provision server")
	}

	dcm.logger.WithFields(map[string]any{
		"server_id":   server.ID,
		"server_name": server.Name,
		"server_ip":   server.IPAddress,
	}).Info("New server provisioned successfully")

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
	dcm.logger.WithFields(map[string]any{
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

	dcm.logger.WithFields(map[string]any{
		"private_key": keyPath,
		"public_key":  keyPath + ".pub",
	}).Info("Generated SSH key pair")

	return nil
}

// waitForServerReady waits for the server to be fully configured and Docker to be running
func (dcm *dockerClientManagerImpl) waitForServerReady(ctx context.Context, server *hetzner.Server) error {
	dcm.logger.WithFields(map[string]any{
		"server_id": server.ID,
	}).Info("Waiting for server to be ready")

	timeout := time.After(8 * time.Minute)
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
			dcm.logger.WithFields(map[string]any{
				"server_id": server.ID,
				"attempts":  attempt,
				"elapsed":   elapsed.String(),
			}).Error("Timeout waiting for server to be ready")
			return errors.New("timeout waiting for server to be ready after 8 minutes")
		case <-ticker.C:
			attempt++
			elapsed := time.Since(startTime)
			dcm.logger.WithFields(map[string]any{
				"server_id": server.ID,
				"attempt":   attempt,
				"elapsed":   elapsed.String(),
			}).Info("Checking server readiness")

			if dcm.checkServerReady(ctx, server) {
				dcm.logger.WithFields(map[string]any{
					"server_id": server.ID,
					"attempts":  attempt,
					"elapsed":   elapsed.String(),
				}).Info("Server is ready")
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
		dcm.logger.WithFields(map[string]any{
			"server_id": server.ID,
			"server_ip": server.IPAddress,
			"error":     err.Error(),
		}).Debug("Server not ready - SSH connection failed")
		return false
	}

	// Check if Docker is running
	output, err := tempSSHClient.ExecuteCommand(checkCtx, "docker version --format '{{.Server.Version}}'")
	if err != nil {
		dcm.logger.WithFields(map[string]any{
			"server_id": server.ID,
			"error":     err.Error(),
		}).Debug("Server not ready - Docker daemon not responding")
		return false
	}

	dockerVersion := strings.TrimSpace(string(output))
	dcm.logger.WithFields(map[string]any{
		"server_id":      server.ID,
		"docker_version": dockerVersion,
	}).Info("Server ready - Docker daemon is responding")

	return true
}

// cleanupStaleServers removes stale or duplicate servers in the background
func (dcm *dockerClientManagerImpl) cleanupStaleServers(ctx context.Context, servers []*hetzner.Server) {
	for _, server := range servers {
		dcm.logger.WithFields(map[string]any{
			"server_id":   server.ID,
			"server_name": server.Name,
			"status":      server.Status,
		}).Info("Cleaning up stale DockBridge server")

		serverID := fmt.Sprintf("%d", server.ID)
		if err := dcm.hetznerClient.DestroyServer(ctx, serverID); err != nil {
			dcm.logger.WithFields(map[string]any{
				"server_id": server.ID,
				"error":     err.Error(),
			}).Error("Failed to cleanup stale server")
		} else {
			dcm.logger.WithFields(map[string]any{
				"server_id": server.ID,
			}).Info("Successfully cleaned up stale server")
		}
	}
}

// Port forwarding integration methods

// RegisterContainerEventHandler registers a handler for container lifecycle events
func (dcm *dockerClientManagerImpl) RegisterContainerEventHandler(handler monitor.ContainerEventHandler) error {
	if dcm.containerMonitor == nil {
		return fmt.Errorf("container monitor not initialized - call StartPortForwarding first")
	}

	return dcm.containerMonitor.RegisterContainerEventHandler(handler)
}

// StartPortForwarding initializes and starts the port forwarding system
func (dcm *dockerClientManagerImpl) StartPortForwarding(ctx context.Context) error {
	if dcm.portForwardConfig == nil {
		dcm.logger.Info("Port forwarding not configured, skipping initialization")
		return nil
	}

	if !dcm.portForwardConfig.Enabled {
		dcm.logger.Info("Port forwarding disabled in configuration")
		return nil
	}

	// Ensure we have a Docker client connection
	dockerClient, err := dcm.GetClient(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get Docker client for port forwarding")
	}

	// Initialize container monitor
	dcm.containerMonitor = monitor.NewContainerMonitor(dockerClient, dcm.logger)

	// Initialize port forward manager
	dcm.portForwardManager = portforward.NewPortForwardManager(dcm.portForwardConfig, dcm.logger)

	// Register port forward manager as container event handler
	err = dcm.containerMonitor.RegisterContainerEventHandler(dcm.portForwardManager)
	if err != nil {
		return errors.Wrap(err, "failed to register port forward manager as event handler")
	}

	// Start port forward manager
	err = dcm.portForwardManager.Start(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to start port forward manager")
	}

	// Start container monitor
	err = dcm.containerMonitor.Start(ctx)
	if err != nil {
		dcm.portForwardManager.Stop()
		return errors.Wrap(err, "failed to start container monitor")
	}

	dcm.logger.WithFields(map[string]any{
		"enabled":           dcm.portForwardConfig.Enabled,
		"conflict_strategy": dcm.portForwardConfig.ConflictStrategy,
		"monitor_interval":  dcm.portForwardConfig.MonitorInterval,
	}).Info("Port forwarding system started successfully")

	return nil
}

// StopPortForwarding stops the port forwarding system
func (dcm *dockerClientManagerImpl) StopPortForwarding() error {
	var errors []error

	if dcm.containerMonitor != nil {
		if err := dcm.containerMonitor.Stop(); err != nil {
			errors = append(errors, fmt.Errorf("failed to stop container monitor: %w", err))
		}
		dcm.containerMonitor = nil
	}

	if dcm.portForwardManager != nil {
		if err := dcm.portForwardManager.Stop(); err != nil {
			errors = append(errors, fmt.Errorf("failed to stop port forward manager: %w", err))
		}
		dcm.portForwardManager = nil
	}

	if len(errors) > 0 {
		dcm.logger.WithFields(map[string]any{
			"error_count": len(errors),
		}).Error("Errors occurred while stopping port forwarding system")
		return fmt.Errorf("multiple errors stopping port forwarding: %v", errors)
	}

	dcm.logger.Info("Port forwarding system stopped successfully")
	return nil
}

// GetPortForwardManager returns the port forward manager instance
func (dcm *dockerClientManagerImpl) GetPortForwardManager() portforward.PortForwardManager {
	return dcm.portForwardManager
}

// InterceptDockerResponse intercepts and modifies Docker API responses for port forwarding
func (dcm *dockerClientManagerImpl) InterceptDockerResponse(response []byte) ([]byte, error) {
	// If port forwarding is not enabled, return response unchanged
	if dcm.portForwardConfig == nil || !dcm.portForwardConfig.Enabled {
		return response, nil
	}

	// If port forward manager is not initialized, return response unchanged
	if dcm.portForwardManager == nil {
		return response, nil
	}

	// For now, this is a placeholder implementation
	// In a full implementation, this would:
	// 1. Parse JSON responses to detect container creation
	// 2. Extract port mapping information
	// 3. Check for port conflicts using the port conflict resolver
	// 4. Modify the response to reflect actual assigned ports
	// 5. Return Docker-compatible error responses for conflicts (fail strategy)

	dcm.logger.Debug("Docker API response interception called (placeholder implementation)")

	// Return response unchanged for now
	return response, nil
}
