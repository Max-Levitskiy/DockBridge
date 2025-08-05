package hetzner

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	"github.com/pkg/errors"
)

// HetznerClient defines the interface for Hetzner Cloud operations
type HetznerClient interface {
	ProvisionServer(ctx context.Context, config *ServerConfig) (*Server, error)
	DestroyServer(ctx context.Context, serverID string) error
	CreateVolume(ctx context.Context, size int, location string) (*Volume, error)
	FindOrCreateDockerVolume(ctx context.Context, location string) (*Volume, error)
	AttachVolume(ctx context.Context, serverID, volumeID string) error
	DetachVolume(ctx context.Context, volumeID string) error
	ManageSSHKeys(ctx context.Context, publicKey string) (*SSHKey, error)
	GetServer(ctx context.Context, serverID string) (*Server, error)
	ListServers(ctx context.Context) ([]*Server, error)
	GetVolume(ctx context.Context, volumeID string) (*Volume, error)
	ListVolumes(ctx context.Context) ([]*Volume, error)
}

// Client implements the HetznerClient interface
type Client struct {
	hcloud *hcloud.Client
	config *Config
}

// Config holds the Hetzner client configuration
type Config struct {
	APIToken   string
	ServerType string
	Location   string
	VolumeSize int
}

// NewClient creates a new Hetzner client instance
func NewClient(config *Config) (*Client, error) {
	if config.APIToken == "" {
		return nil, errors.New("Hetzner API token is required")
	}

	hcloudClient := hcloud.NewClient(hcloud.WithToken(config.APIToken))

	return &Client{
		hcloud: hcloudClient,
		config: config,
	}, nil
}

// ServerConfig defines configuration for server provisioning
type ServerConfig struct {
	Name       string
	ServerType string
	Location   string
	SSHKeyID   int64
	VolumeID   string
	UserData   string
}

// Server represents a Hetzner Cloud server
type Server struct {
	ID        int64
	Name      string
	Status    string
	IPAddress string
	VolumeID  string
	CreatedAt time.Time
}

// Volume represents a Hetzner Cloud volume
type Volume struct {
	ID       int64
	Name     string
	Size     int
	Location string
	Status   string
}

// SSHKey represents a Hetzner Cloud SSH key
type SSHKey struct {
	ID          int64
	Name        string
	Fingerprint string
	PublicKey   string
}

// ProvisionServer creates a new server with Docker CE and cloud-init configuration
func (c *Client) ProvisionServer(ctx context.Context, config *ServerConfig) (*Server, error) {
	// Get server type
	serverType, _, err := c.hcloud.ServerType.GetByName(ctx, config.ServerType)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get server type")
	}
	if serverType == nil {
		return nil, fmt.Errorf("server type %s not found", config.ServerType)
	}

	// Get location
	location, _, err := c.hcloud.Location.GetByName(ctx, config.Location)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get location")
	}
	if location == nil {
		return nil, fmt.Errorf("location %s not found", config.Location)
	}

	// Get image (Ubuntu 22.04)
	image, _, err := c.hcloud.Image.GetByName(ctx, "ubuntu-22.04")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get Ubuntu image")
	}
	if image == nil {
		return nil, errors.New("Ubuntu 22.04 image not found")
	}

	// Prepare server creation options
	opts := hcloud.ServerCreateOpts{
		Name:       config.Name,
		ServerType: serverType,
		Image:      image,
		Location:   location,
	}

	// Add UserData if provided
	if config.UserData != "" {
		opts.UserData = config.UserData
	}

	// Add SSH key if provided
	if config.SSHKeyID > 0 {
		sshKey := &hcloud.SSHKey{ID: config.SSHKeyID}
		opts.SSHKeys = []*hcloud.SSHKey{sshKey}
	}

	// Add volume if provided
	if config.VolumeID != "" {
		volume := &hcloud.Volume{ID: parseVolumeID(config.VolumeID)}
		opts.Volumes = []*hcloud.Volume{volume}
	}

	// Create the server
	result, _, err := c.hcloud.Server.Create(ctx, opts)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create server")
	}

	// Wait for server to be running
	_, errCh := c.hcloud.Action.WatchProgress(ctx, result.Action)
	if err := <-errCh; err != nil {
		return nil, errors.Wrap(err, "failed to wait for server creation")
	}

	// Get the created server details
	server, _, err := c.hcloud.Server.GetByID(ctx, result.Server.ID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get created server")
	}
	if server == nil {
		return nil, errors.New("created server not found")
	}

	convertedServer := convertServer(server)
	if convertedServer == nil {
		return nil, errors.New("failed to convert server")
	}

	return convertedServer, nil
}

// DestroyServer terminates a server and cleans up resources
func (c *Client) DestroyServer(ctx context.Context, serverID string) error {
	id := parseServerID(serverID)

	// Get server to check if it exists
	server, _, err := c.hcloud.Server.GetByID(ctx, id)
	if err != nil {
		return errors.Wrap(err, "failed to get server")
	}
	if server == nil {
		return fmt.Errorf("server %s not found", serverID)
	}

	// Delete the server
	_, _, err = c.hcloud.Server.DeleteWithResult(ctx, server)
	if err != nil {
		return errors.Wrap(err, "failed to delete server")
	}

	return nil
}

// CreateVolume creates a new persistent volume
func (c *Client) CreateVolume(ctx context.Context, size int, location string) (*Volume, error) {
	// Get location
	loc, _, err := c.hcloud.Location.GetByName(ctx, location)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get location")
	}
	if loc == nil {
		return nil, fmt.Errorf("location %s not found", location)
	}

	// Generate unique volume name
	volumeName := fmt.Sprintf("dockbridge-volume-%d", time.Now().Unix())

	// Create volume
	opts := hcloud.VolumeCreateOpts{
		Name:     volumeName,
		Size:     size,
		Location: loc,
		Format:   hcloud.Ptr("ext4"),
	}

	result, _, err := c.hcloud.Volume.Create(ctx, opts)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create volume")
	}

	// Wait for volume creation to complete
	_, errCh := c.hcloud.Action.WatchProgress(ctx, result.Action)
	if err := <-errCh; err != nil {
		return nil, errors.Wrap(err, "failed to wait for volume creation")
	}

	return convertVolume(result.Volume), nil
}

// FindOrCreateDockerVolume finds an existing Docker data volume or creates a new one
func (c *Client) FindOrCreateDockerVolume(ctx context.Context, location string) (*Volume, error) {
	// First, try to find an existing Docker data volume
	volumes, err := c.ListVolumes(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list volumes")
	}

	// Look for existing Docker data volume in the same location
	for _, volume := range volumes {
		if volume.Location == location && strings.Contains(volume.Name, "dockbridge-docker-data") {
			// Check if volume is available (not attached to another server)
			if volume.Status == "available" {
				return volume, nil
			}
		}
	}

	// No available volume found, create a new one with Docker data naming
	return c.createDockerDataVolume(ctx, location)
}

// createDockerDataVolume creates a new volume specifically for Docker data
func (c *Client) createDockerDataVolume(ctx context.Context, location string) (*Volume, error) {
	// Get location
	loc, _, err := c.hcloud.Location.GetByName(ctx, location)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get location")
	}
	if loc == nil {
		return nil, fmt.Errorf("location %s not found", location)
	}

	// Generate unique volume name with Docker data identifier
	volumeName := fmt.Sprintf("dockbridge-docker-data-%d", time.Now().Unix())

	// Create volume with ext4 filesystem for Docker data
	opts := hcloud.VolumeCreateOpts{
		Name:     volumeName,
		Size:     c.config.VolumeSize,
		Location: loc,
		Format:   hcloud.Ptr("ext4"),
		Labels: map[string]string{
			"purpose":    "docker-data",
			"created-by": "dockbridge",
		},
	}

	result, _, err := c.hcloud.Volume.Create(ctx, opts)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create volume")
	}

	// Wait for volume creation to complete
	_, errCh := c.hcloud.Action.WatchProgress(ctx, result.Action)
	if err := <-errCh; err != nil {
		return nil, errors.Wrap(err, "failed to wait for volume creation")
	}

	return convertVolume(result.Volume), nil
}

// AttachVolume attaches a volume to a server
func (c *Client) AttachVolume(ctx context.Context, serverID, volumeID string) error {
	sID := parseServerID(serverID)
	vID := parseVolumeID(volumeID)

	// Get server
	server, _, err := c.hcloud.Server.GetByID(ctx, sID)
	if err != nil {
		return errors.Wrap(err, "failed to get server")
	}
	if server == nil {
		return fmt.Errorf("server %s not found", serverID)
	}

	// Get volume
	volume, _, err := c.hcloud.Volume.GetByID(ctx, vID)
	if err != nil {
		return errors.Wrap(err, "failed to get volume")
	}
	if volume == nil {
		return fmt.Errorf("volume %s not found", volumeID)
	}

	// Attach volume to server
	_, _, err = c.hcloud.Volume.Attach(ctx, volume, server)
	if err != nil {
		return errors.Wrap(err, "failed to attach volume")
	}

	return nil
}

// DetachVolume detaches a volume from its server
func (c *Client) DetachVolume(ctx context.Context, volumeID string) error {
	vID := parseVolumeID(volumeID)

	// Get volume
	volume, _, err := c.hcloud.Volume.GetByID(ctx, vID)
	if err != nil {
		return errors.Wrap(err, "failed to get volume")
	}
	if volume == nil {
		return fmt.Errorf("volume %s not found", volumeID)
	}

	// Detach volume
	_, _, err = c.hcloud.Volume.Detach(ctx, volume)
	if err != nil {
		return errors.Wrap(err, "failed to detach volume")
	}

	return nil
}

// ManageSSHKeys uploads and manages SSH keys, reusing existing keys if they match
func (c *Client) ManageSSHKeys(ctx context.Context, publicKey string) (*SSHKey, error) {
	// First, try to find an existing SSH key with the same public key
	existingKey, err := c.findExistingSSHKey(ctx, publicKey)
	if err != nil {
		return nil, errors.Wrap(err, "failed to search for existing SSH key")
	}

	if existingKey != nil {
		// Found existing key, reuse it
		fmt.Printf("Found existing SSH key: %s (ID: %d)\n", existingKey.Name, existingKey.ID)
		return existingKey, nil
	}

	// No existing key found, create a new one
	keyName := fmt.Sprintf("dockbridge-key-%d", time.Now().Unix())
	fmt.Printf("Creating new SSH key: %s\n", keyName)

	// Create SSH key
	opts := hcloud.SSHKeyCreateOpts{
		Name:      keyName,
		PublicKey: publicKey,
	}

	result, _, err := c.hcloud.SSHKey.Create(ctx, opts)
	if err != nil {
		// If creation failed, it might be because the key already exists
		// Try to find it again in case it was created between our initial check and now
		if existingKey, searchErr := c.findExistingSSHKey(ctx, publicKey); searchErr == nil && existingKey != nil {
			return existingKey, nil
		}
		return nil, errors.Wrap(err, "failed to create SSH key")
	}

	return convertSSHKey(result), nil
}

// findExistingSSHKey searches for an existing SSH key that matches the given public key
func (c *Client) findExistingSSHKey(ctx context.Context, publicKey string) (*SSHKey, error) {
	// List all SSH keys
	sshKeys, err := c.hcloud.SSHKey.All(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list SSH keys")
	}

	// Look for a key with matching public key content
	// Normalize the input key for comparison
	normalizedInputKey := strings.TrimSpace(publicKey)

	for _, key := range sshKeys {
		// Normalize the stored key for comparison
		normalizedStoredKey := strings.TrimSpace(key.PublicKey)

		if normalizedStoredKey == normalizedInputKey {
			return convertSSHKey(key), nil
		}

		// Also try comparing just the key part (without comment)
		// SSH keys format: "type key-data comment"
		inputParts := strings.Fields(normalizedInputKey)
		storedParts := strings.Fields(normalizedStoredKey)

		if len(inputParts) >= 2 && len(storedParts) >= 2 {
			// Compare type and key data (ignore comment)
			if inputParts[0] == storedParts[0] && inputParts[1] == storedParts[1] {
				return convertSSHKey(key), nil
			}
		}
	}

	return nil, nil // No matching key found
}

// GetServer retrieves server information by ID
func (c *Client) GetServer(ctx context.Context, serverID string) (*Server, error) {
	id := parseServerID(serverID)

	server, _, err := c.hcloud.Server.GetByID(ctx, id)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get server")
	}
	if server == nil {
		return nil, fmt.Errorf("server %s not found", serverID)
	}

	return convertServer(server), nil
}

// ListServers retrieves all servers
func (c *Client) ListServers(ctx context.Context) ([]*Server, error) {
	servers, err := c.hcloud.Server.All(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list servers")
	}

	result := make([]*Server, 0, len(servers))
	for _, server := range servers {
		if converted := convertServer(server); converted != nil {
			result = append(result, converted)
		}
	}

	return result, nil
}

// GetVolume retrieves volume information by ID
func (c *Client) GetVolume(ctx context.Context, volumeID string) (*Volume, error) {
	id := parseVolumeID(volumeID)

	volume, _, err := c.hcloud.Volume.GetByID(ctx, id)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get volume")
	}
	if volume == nil {
		return nil, fmt.Errorf("volume %s not found", volumeID)
	}

	return convertVolume(volume), nil
}

// ListVolumes retrieves all volumes
func (c *Client) ListVolumes(ctx context.Context) ([]*Volume, error) {
	volumes, err := c.hcloud.Volume.All(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list volumes")
	}

	result := make([]*Volume, len(volumes))
	for i, volume := range volumes {
		result[i] = convertVolume(volume)
	}

	return result, nil
}
