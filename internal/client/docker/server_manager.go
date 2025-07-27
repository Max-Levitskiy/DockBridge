package docker

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/dockbridge/dockbridge/internal/client/hetzner"
	"github.com/dockbridge/dockbridge/internal/client/ssh"
	"github.com/dockbridge/dockbridge/pkg/logger"
	"github.com/pkg/errors"
)

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ServerManager handles server provisioning, selection, and lifecycle management
type ServerManager struct {
	hetznerClient hetzner.HetznerClient
	sshConfig     *ssh.ClientConfig
	hetznerConfig *hetzner.Config
	logger        logger.LoggerInterface
}

// NewServerManager creates a new server manager
func NewServerManager(hetznerClient hetzner.HetznerClient, sshConfig *ssh.ClientConfig, hetznerConfig *hetzner.Config, logger logger.LoggerInterface) *ServerManager {
	return &ServerManager{
		hetznerClient: hetznerClient,
		sshConfig:     sshConfig,
		hetznerConfig: hetznerConfig,
		logger:        logger,
	}
}

// GetOrProvisionServer gets an existing server or provisions a new one
func (sm *ServerManager) GetOrProvisionServer(ctx context.Context) (*hetzner.Server, error) {
	// First, try to find an existing DockBridge server
	servers, err := sm.hetznerClient.ListServers(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list servers")
	}

	sm.logger.WithFields(map[string]interface{}{
		"total_servers": len(servers),
	}).Debug("Listed servers from Hetzner")

	// Look for DockBridge servers and categorize them
	var runningServers []*hetzner.Server
	var staleServers []*hetzner.Server

	for _, server := range servers {
		if strings.HasPrefix(server.Name, "dockbridge-") {
			sm.logger.WithFields(map[string]interface{}{
				"server_id":   server.ID,
				"server_name": server.Name,
				"server_ip":   server.IPAddress,
				"status":      server.Status,
			}).Debug("Found DockBridge server")

			if server.Status == "running" {
				runningServers = append(runningServers, server)
			} else {
				// Server is not running (stopped, error, etc.)
				staleServers = append(staleServers, server)
			}
		}
	}

	// Clean up stale servers in background
	if len(staleServers) > 0 {
		go sm.cleanupStaleServers(context.Background(), staleServers)
	}

	// If we have multiple running servers, clean up extras and keep the newest
	if len(runningServers) > 1 {
		sm.logger.WithFields(map[string]interface{}{
			"running_count": len(runningServers),
		}).Warn("Multiple DockBridge servers running, cleaning up extras")

		// Sort by ID (newest first)
		selectedServer := runningServers[0]
		var serversToCleanup []*hetzner.Server

		for _, server := range runningServers {
			if server.ID > selectedServer.ID {
				serversToCleanup = append(serversToCleanup, selectedServer)
				selectedServer = server
			} else {
				serversToCleanup = append(serversToCleanup, server)
			}
		}

		// Clean up extra servers in background
		if len(serversToCleanup) > 0 {
			go sm.cleanupStaleServers(context.Background(), serversToCleanup)
		}

		sm.logger.WithFields(map[string]interface{}{
			"server_id":     selectedServer.ID,
			"server_ip":     selectedServer.IPAddress,
			"cleaned_count": len(serversToCleanup),
		}).Info("Selected newest server, cleaning up duplicates")

		return selectedServer, nil
	}

	// If we have exactly one running server, verify it's accessible
	if len(runningServers) == 1 {
		server := runningServers[0]

		sm.logger.WithFields(map[string]interface{}{
			"server_id": server.ID,
			"server_ip": server.IPAddress,
		}).Info("Found existing DockBridge server, verifying accessibility")

		// Quick accessibility check
		if sm.isServerAccessible(ctx, server) {
			sm.logger.WithFields(map[string]interface{}{
				"server_id": server.ID,
				"server_ip": server.IPAddress,
			}).Info("Server is accessible, using existing server")
			return server, nil
		} else {
			sm.logger.WithFields(map[string]interface{}{
				"server_id": server.ID,
				"server_ip": server.IPAddress,
			}).Warn("Server is not accessible, will provision new server")

			// Clean up inaccessible server in background
			go sm.cleanupStaleServers(context.Background(), []*hetzner.Server{server})
		}
	}

	// No accessible running server found, provision a new one
	sm.logger.Info("No accessible running server found, provisioning new server")
	server, err := sm.provisionNewServer(ctx)
	if err != nil {
		return nil, err
	}

	// For newly provisioned servers, we trust they will be ready after provisioning
	// Don't do accessibility check immediately as the server needs time to boot
	sm.logger.WithFields(map[string]interface{}{
		"server_id": server.ID,
		"server_ip": server.IPAddress,
	}).Info("New server provisioned successfully, will be ready shortly")

	return server, nil
}

// provisionNewServer creates a new Hetzner server with Docker CE
func (sm *ServerManager) provisionNewServer(ctx context.Context) (*hetzner.Server, error) {
	// Generate server name with timestamp
	serverName := fmt.Sprintf("dockbridge-%d", time.Now().Unix())

	// Get SSH public key first
	sshKeyPath := expandPath(sm.sshConfig.PrivateKeyPath)
	publicKeyPath := sshKeyPath + ".pub"

	sm.logger.WithFields(map[string]interface{}{
		"private_key_path": sshKeyPath,
		"public_key_path":  publicKeyPath,
	}).Debug("Reading SSH key files")

	publicKeyBytes, err := os.ReadFile(publicKeyPath)
	if err != nil {
		sm.logger.WithFields(map[string]interface{}{
			"public_key_path": publicKeyPath,
			"error":           err.Error(),
		}).Error("Failed to read SSH public key")
		return nil, errors.Wrap(err, "failed to read SSH public key")
	}

	publicKeyContent := strings.TrimSpace(string(publicKeyBytes))

	// Create cloud-init script for Docker CE installation with SSH key
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

# Configure Docker daemon to listen on TCP port 2376 (insecure for internal use)
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

# Install additional tools
echo "$(date): Installing additional tools"
apt-get install -y htop curl wget git

# Create dockbridge user
echo "$(date): Creating dockbridge user"
useradd -m -s /bin/bash dockbridge
usermod -aG docker dockbridge

echo "$(date): DockBridge server setup completed successfully"
`, publicKeyContent)

	// Upload SSH key to Hetzner (ManageSSHKeys will reuse existing keys)
	sm.logger.WithFields(map[string]interface{}{
		"public_key_length": len(publicKeyContent),
		"public_key_prefix": publicKeyContent[:min(50, len(publicKeyContent))],
	}).Debug("Managing SSH key with Hetzner")

	sshKey, err := sm.hetznerClient.ManageSSHKeys(ctx, publicKeyContent)
	if err != nil {
		sm.logger.WithFields(map[string]interface{}{
			"error": err.Error(),
		}).Error("Failed to manage SSH key with Hetzner")
		return nil, errors.Wrap(err, "failed to manage SSH key with Hetzner")
	}

	serverConfig := &hetzner.ServerConfig{
		Name:       serverName,
		ServerType: sm.hetznerConfig.ServerType,
		Location:   sm.hetznerConfig.Location,
		UserData:   cloudInitScript,
	}

	// SSH key should always be available now
	sm.logger.WithFields(map[string]interface{}{
		"ssh_key_id":          sshKey.ID,
		"ssh_key_fingerprint": sshKey.Fingerprint,
		"ssh_key_name":        sshKey.Name,
	}).Info("Using SSH key for server provisioning")
	serverConfig.SSHKeyID = sshKey.ID

	server, err := sm.hetznerClient.ProvisionServer(ctx, serverConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to provision server")
	}

	sm.logger.WithFields(map[string]interface{}{
		"server_id":   server.ID,
		"server_name": server.Name,
		"server_ip":   server.IPAddress,
	}).Info("New server provisioned successfully")

	// Wait for server to be fully ready (Docker service to start)
	if err := sm.waitForServerReady(ctx, server); err != nil {
		// If server failed to become ready, clean it up
		go sm.cleanupStaleServers(context.Background(), []*hetzner.Server{server})
		return nil, errors.Wrap(err, "server provisioned but not ready")
	}

	return server, nil
}

// waitForServerReady waits for the server to be fully configured and Docker to be running
func (sm *ServerManager) waitForServerReady(ctx context.Context, server *hetzner.Server) error {
	sm.logger.WithFields(map[string]interface{}{
		"server_id": server.ID,
	}).Info("Waiting for server to be ready")

	// Wait up to 8 minutes for server to be ready (cloud-init can take time)
	timeout := time.After(8 * time.Minute)
	ticker := time.NewTicker(15 * time.Second) // Check every 15 seconds to be less aggressive
	defer ticker.Stop()

	attempt := 0
	startTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			elapsed := time.Since(startTime)
			sm.logger.WithFields(map[string]interface{}{
				"server_id": server.ID,
				"attempts":  attempt,
				"elapsed":   elapsed.String(),
			}).Error("Timeout waiting for server to be ready")
			return errors.New("timeout waiting for server to be ready after 8 minutes")
		case <-ticker.C:
			attempt++
			elapsed := time.Since(startTime)
			sm.logger.WithFields(map[string]interface{}{
				"server_id": server.ID,
				"attempt":   attempt,
				"elapsed":   elapsed.String(),
			}).Info("Checking server readiness")

			// Try to connect and check if Docker is running
			if sm.checkServerReady(ctx, server) {
				sm.logger.WithFields(map[string]interface{}{
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
func (sm *ServerManager) checkServerReady(ctx context.Context, server *hetzner.Server) bool {
	// Create temporary SSH client to check server status
	sshKeyPath := expandPath(sm.sshConfig.PrivateKeyPath)
	sshConfig := &ssh.ClientConfig{
		Host:           server.IPAddress,
		Port:           sm.sshConfig.Port,
		User:           "root",
		PrivateKeyPath: sshKeyPath,
		Timeout:        15 * time.Second, // Shorter timeout for readiness checks
	}

	sm.logger.WithFields(map[string]interface{}{
		"server_id":    server.ID,
		"server_ip":    server.IPAddress,
		"ssh_key_path": sshKeyPath,
		"ssh_port":     sm.sshConfig.Port,
	}).Debug("Attempting SSH connection for readiness check")

	tempSSHClient := ssh.NewClient(sshConfig)
	defer tempSSHClient.Close()

	// Create a context with timeout for this check
	checkCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	// Try to connect
	if err := tempSSHClient.Connect(checkCtx); err != nil {
		sm.logger.WithFields(map[string]interface{}{
			"server_id":    server.ID,
			"server_ip":    server.IPAddress,
			"ssh_key_path": sshKeyPath,
			"error":        err.Error(),
		}).Debug("Server not ready - SSH connection failed")
		return false
	}

	sm.logger.WithFields(map[string]interface{}{
		"server_id": server.ID,
		"server_ip": server.IPAddress,
	}).Debug("SSH connection successful, checking Docker status")

	// Check if Docker is running
	output, err := tempSSHClient.ExecuteCommand(checkCtx, "systemctl is-active docker")
	if err != nil {
		sm.logger.WithFields(map[string]interface{}{
			"server_id": server.ID,
			"error":     err.Error(),
			"output":    string(output),
		}).Debug("Server not ready - Docker status check failed")
		return false
	}

	dockerStatus := strings.TrimSpace(string(output))
	if dockerStatus != "active" {
		sm.logger.WithFields(map[string]interface{}{
			"server_id":     server.ID,
			"docker_status": dockerStatus,
		}).Debug("Server not ready - Docker not active")
		return false
	}

	// Additional check: verify Docker daemon is responding
	output, err = tempSSHClient.ExecuteCommand(checkCtx, "docker version --format '{{.Server.Version}}'")
	if err != nil {
		sm.logger.WithFields(map[string]interface{}{
			"server_id": server.ID,
			"error":     err.Error(),
		}).Debug("Server not ready - Docker daemon not responding")
		return false
	}

	dockerVersion := strings.TrimSpace(string(output))
	sm.logger.WithFields(map[string]interface{}{
		"server_id":      server.ID,
		"docker_version": dockerVersion,
	}).Info("Server ready - Docker daemon is responding")

	return true
}

// isServerAccessible performs a quick check to see if the server is accessible via SSH
// This should only be used for existing servers, not newly provisioned ones
func (sm *ServerManager) isServerAccessible(ctx context.Context, server *hetzner.Server) bool {
	// Create a short timeout context for the accessibility check
	checkCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// Create temporary SSH client to check server accessibility
	sshConfig := &ssh.ClientConfig{
		Host:           server.IPAddress,
		Port:           sm.sshConfig.Port,
		User:           "root",
		PrivateKeyPath: expandPath(sm.sshConfig.PrivateKeyPath),
		Timeout:        10 * time.Second, // Longer timeout for existing servers
	}

	tempSSHClient := ssh.NewClient(sshConfig)
	defer tempSSHClient.Close()

	// Try to connect
	if err := tempSSHClient.Connect(checkCtx); err != nil {
		sm.logger.WithFields(map[string]interface{}{
			"server_id": server.ID,
			"server_ip": server.IPAddress,
			"error":     err.Error(),
		}).Debug("Server accessibility check failed - server may be stale")
		return false
	}

	// Quick Docker check
	output, err := tempSSHClient.ExecuteCommand(checkCtx, "docker version --format '{{.Server.Version}}' 2>/dev/null")
	if err != nil || len(strings.TrimSpace(string(output))) == 0 {
		sm.logger.WithFields(map[string]interface{}{
			"server_id": server.ID,
			"server_ip": server.IPAddress,
			"error":     err,
		}).Debug("Server accessibility check failed - Docker not ready")
		return false
	}

	sm.logger.WithFields(map[string]interface{}{
		"server_id":      server.ID,
		"server_ip":      server.IPAddress,
		"docker_version": strings.TrimSpace(string(output)),
	}).Debug("Server accessibility check passed")

	return true
}

// cleanupStaleServers removes stale or duplicate servers in the background
func (sm *ServerManager) cleanupStaleServers(ctx context.Context, servers []*hetzner.Server) {
	for _, server := range servers {
		sm.logger.WithFields(map[string]interface{}{
			"server_id":   server.ID,
			"server_name": server.Name,
			"status":      server.Status,
		}).Info("Cleaning up stale DockBridge server")

		serverID := fmt.Sprintf("%d", server.ID)
		if err := sm.hetznerClient.DestroyServer(ctx, serverID); err != nil {
			sm.logger.WithFields(map[string]interface{}{
				"server_id": server.ID,
				"error":     err.Error(),
			}).Error("Failed to cleanup stale server")
		} else {
			sm.logger.WithFields(map[string]interface{}{
				"server_id": server.ID,
			}).Info("Successfully cleaned up stale server")
		}
	}
}
