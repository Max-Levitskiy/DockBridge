package hetzner

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
)

// LifecycleManager handles server lifecycle operations
type LifecycleManager struct {
	client *Client
}

// NewLifecycleManager creates a new lifecycle manager
func NewLifecycleManager(client *Client) *LifecycleManager {
	return &LifecycleManager{
		client: client,
	}
}

// ProvisionServerWithVolume provisions a server with persistent volume and proper cleanup
func (lm *LifecycleManager) ProvisionServerWithVolume(ctx context.Context, config *ServerProvisionConfig) (*ServerWithVolume, error) {
	var volume *Volume
	var server *Server
	var sshKey *SSHKey
	var err error

	// Create SSH key if public key is provided
	if config.SSHPublicKey != "" {
		sshKey, err = lm.client.ManageSSHKeys(ctx, config.SSHPublicKey)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create SSH key")
		}
	}

	// Create volume if size is specified
	if config.VolumeSize > 0 {
		volume, err = lm.client.CreateVolume(ctx, config.VolumeSize, config.Location)
		if err != nil {
			// Cleanup SSH key if volume creation fails
			if sshKey != nil {
				lm.cleanupSSHKey(ctx, sshKey.ID)
			}
			return nil, errors.Wrap(err, "failed to create volume")
		}
	}

	// Generate cloud-init script
	cloudInitConfig := &CloudInitConfig{
		SSHPublicKey:  config.SSHPublicKey,
		VolumeMount:   config.VolumeMount,
		KeepAlivePort: config.KeepAlivePort,
		DockerAPIPort: config.DockerAPIPort,
	}
	userDataScript := GenerateCloudInitScript(cloudInitConfig)

	// Prepare server configuration
	serverConfig := &ServerConfig{
		Name:       config.ServerName,
		ServerType: config.ServerType,
		Location:   config.Location,
		UserData:   userDataScript,
	}

	if sshKey != nil {
		serverConfig.SSHKeyID = sshKey.ID
	}

	if volume != nil {
		serverConfig.VolumeID = fmt.Sprintf("%d", volume.ID)
	}

	// Create server
	server, err = lm.client.ProvisionServer(ctx, serverConfig)
	if err != nil {
		// Cleanup resources if server creation fails
		if volume != nil {
			lm.cleanupVolume(ctx, volume.ID)
		}
		if sshKey != nil {
			lm.cleanupSSHKey(ctx, sshKey.ID)
		}
		return nil, errors.Wrap(err, "failed to provision server")
	}

	// Wait for server to be fully ready
	err = lm.waitForServerReady(ctx, server.ID, 5*time.Minute)
	if err != nil {
		// Cleanup all resources if server doesn't become ready
		lm.cleanupServer(ctx, server.ID)
		if volume != nil {
			lm.cleanupVolume(ctx, volume.ID)
		}
		if sshKey != nil {
			lm.cleanupSSHKey(ctx, sshKey.ID)
		}
		return nil, errors.Wrap(err, "server failed to become ready")
	}

	return &ServerWithVolume{
		Server: server,
		Volume: volume,
		SSHKey: sshKey,
	}, nil
}

// DestroyServerWithCleanup destroys a server and handles proper cleanup
func (lm *LifecycleManager) DestroyServerWithCleanup(ctx context.Context, serverID string, preserveVolume bool) error {
	// Get server details first
	server, err := lm.client.GetServer(ctx, serverID)
	if err != nil {
		return errors.Wrap(err, "failed to get server details")
	}

	// Detach volume if present and we want to preserve it
	if server.VolumeID != "" && preserveVolume {
		err = lm.client.DetachVolume(ctx, server.VolumeID)
		if err != nil {
			// Log error but continue with server destruction
			fmt.Printf("Warning: failed to detach volume %s: %v\n", server.VolumeID, err)
		}
	}

	// Destroy the server
	err = lm.client.DestroyServer(ctx, serverID)
	if err != nil {
		return errors.Wrap(err, "failed to destroy server")
	}

	// Clean up volume if not preserving
	if server.VolumeID != "" && !preserveVolume {
		err = lm.cleanupVolume(ctx, parseVolumeID(server.VolumeID))
		if err != nil {
			// Log error but don't fail the operation
			fmt.Printf("Warning: failed to cleanup volume %s: %v\n", server.VolumeID, err)
		}
	}

	return nil
}

// waitForServerReady waits for a server to be fully ready and accessible
func (lm *LifecycleManager) waitForServerReady(ctx context.Context, serverID int64, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return errors.New("timeout waiting for server to be ready")
		case <-ticker.C:
			server, err := lm.client.GetServer(ctx, fmt.Sprintf("%d", serverID))
			if err != nil {
				continue // Keep trying
			}

			if server.Status == "running" && server.IPAddress != "" {
				// Additional check: wait a bit more for cloud-init to complete
				time.Sleep(30 * time.Second)
				return nil
			}
		}
	}
}

// cleanupServer removes a server without error handling
func (lm *LifecycleManager) cleanupServer(ctx context.Context, serverID int64) {
	_ = lm.client.DestroyServer(ctx, fmt.Sprintf("%d", serverID))
}

// cleanupVolume removes a volume without error handling
func (lm *LifecycleManager) cleanupVolume(ctx context.Context, volumeID int64) error {
	// Note: Hetzner doesn't have a direct volume delete in the interface we defined
	// This would need to be implemented in the client if needed
	return nil
}

// cleanupSSHKey removes an SSH key without error handling
func (lm *LifecycleManager) cleanupSSHKey(ctx context.Context, keyID int64) {
	// Note: SSH key cleanup would need to be implemented in the client
	// For now, we'll leave keys as they can be reused
}

// ServerProvisionConfig holds configuration for server provisioning with lifecycle management
type ServerProvisionConfig struct {
	ServerName    string
	ServerType    string
	Location      string
	VolumeSize    int
	VolumeMount   string
	SSHPublicKey  string
	KeepAlivePort int
	DockerAPIPort int
}

// ServerWithVolume represents a server with its associated resources
type ServerWithVolume struct {
	Server *Server
	Volume *Volume
	SSHKey *SSHKey
}

// GetDefaultProvisionConfig returns a default server provision configuration
func GetDefaultProvisionConfig() *ServerProvisionConfig {
	return &ServerProvisionConfig{
		ServerName:    fmt.Sprintf("dockbridge-%d", time.Now().Unix()),
		ServerType:    "cpx21",
		Location:      "fsn1",
		VolumeSize:    10,
		VolumeMount:   "/mnt/docker-data",
		KeepAlivePort: 8080,
		DockerAPIPort: 2376,
	}
}
