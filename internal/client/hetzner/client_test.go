package hetzner

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// Note: Mock client structures would be defined here for comprehensive testing

// HetznerClientTestSuite defines the test suite for Hetzner client
type HetznerClientTestSuite struct {
	suite.Suite
	client *Client
	ctx    context.Context
}

func (suite *HetznerClientTestSuite) SetupTest() {
	suite.ctx = context.Background()

	config := &Config{
		APIToken:   "test-token",
		ServerType: "cpx21",
		Location:   "fsn1",
		VolumeSize: 10,
	}

	suite.client = &Client{
		config: config,
		// Note: In a real implementation, we'd inject a mock hcloud client
	}
}

func (suite *HetznerClientTestSuite) TestNewClient() {
	// Test successful client creation
	config := &Config{
		APIToken:   "test-token",
		ServerType: "cpx21",
		Location:   "fsn1",
		VolumeSize: 10,
	}

	client, err := NewClient(config)
	suite.NoError(err)
	suite.NotNil(client)
	suite.Equal(config, client.config)
}

func (suite *HetznerClientTestSuite) TestNewClientMissingToken() {
	// Test client creation with missing API token
	config := &Config{
		ServerType: "cpx21",
		Location:   "fsn1",
		VolumeSize: 10,
	}

	client, err := NewClient(config)
	suite.Error(err)
	suite.Nil(client)
	suite.Contains(err.Error(), "API token is required")
}

func (suite *HetznerClientTestSuite) TestConvertServer() {
	// Test server conversion
	ip := net.ParseIP("192.168.1.1")
	volume := &hcloud.Volume{ID: 67890}
	hcloudServer := &hcloud.Server{
		ID:      12345,
		Name:    "test-server",
		Status:  hcloud.ServerStatusRunning,
		Created: time.Now(),
		PublicNet: hcloud.ServerPublicNet{
			IPv4: hcloud.ServerPublicNetIPv4{
				IP: ip,
			},
		},
		Volumes: []*hcloud.Volume{volume},
	}

	server := convertServer(hcloudServer)

	suite.Equal(int64(12345), server.ID)
	suite.Equal("test-server", server.Name)
	suite.Equal("running", server.Status)
	suite.Equal("192.168.1.1", server.IPAddress)
	suite.Equal("67890", server.VolumeID)
}

func (suite *HetznerClientTestSuite) TestConvertVolume() {
	// Test volume conversion
	hcloudVolume := &hcloud.Volume{
		ID:   67890,
		Name: "test-volume",
		Size: 10,
		Location: &hcloud.Location{
			Name: "fsn1",
		},
		Status: hcloud.VolumeStatusAvailable,
	}

	volume := convertVolume(hcloudVolume)

	suite.Equal(int64(67890), volume.ID)
	suite.Equal("test-volume", volume.Name)
	suite.Equal(10, volume.Size)
	suite.Equal("fsn1", volume.Location)
	suite.Equal("available", volume.Status)
}

func (suite *HetznerClientTestSuite) TestConvertSSHKey() {
	// Test SSH key conversion
	hcloudSSHKey := &hcloud.SSHKey{
		ID:          11111,
		Name:        "test-key",
		Fingerprint: "aa:bb:cc:dd:ee:ff",
		PublicKey:   "ssh-rsa AAAAB3NzaC1yc2E...",
	}

	sshKey := convertSSHKey(hcloudSSHKey)

	suite.Equal(int64(11111), sshKey.ID)
	suite.Equal("test-key", sshKey.Name)
	suite.Equal("aa:bb:cc:dd:ee:ff", sshKey.Fingerprint)
	suite.Equal("ssh-rsa AAAAB3NzaC1yc2E...", sshKey.PublicKey)
}

func (suite *HetznerClientTestSuite) TestParseServerID() {
	// Test valid server ID parsing
	id := parseServerID("12345")
	suite.Equal(int64(12345), id)

	// Test invalid server ID parsing
	id = parseServerID("invalid")
	suite.Equal(int64(0), id)
}

func (suite *HetznerClientTestSuite) TestParseVolumeID() {
	// Test valid volume ID parsing
	id := parseVolumeID("67890")
	suite.Equal(int64(67890), id)

	// Test invalid volume ID parsing
	id = parseVolumeID("invalid")
	suite.Equal(int64(0), id)
}

// Note: Mock implementations would go here in a full implementation
// For now, we focus on testing the utility functions and data structures

func TestHetznerClientSuite(t *testing.T) {
	suite.Run(t, new(HetznerClientTestSuite))
}

// Additional unit tests for specific functions
func TestGenerateCloudInitScript(t *testing.T) {
	config := &CloudInitConfig{
		DockerVersion: "latest",
		SSHPublicKey:  "ssh-rsa AAAAB3NzaC1yc2E...",
		VolumeMount:   "/mnt/docker-data",
		KeepAlivePort: 8080,
		DockerAPIPort: 2376,
	}

	script := GenerateCloudInitScript(config)

	assert.Contains(t, script, "#cloud-config")
	assert.Contains(t, script, "ssh-rsa AAAAB3NzaC1yc2E...")
	assert.Contains(t, script, "/mnt/docker-data")
	assert.Contains(t, script, "docker-ce")
	assert.Contains(t, script, "2376")
	assert.Contains(t, script, "8080")
}

func TestGenerateCloudInitScriptDefaults(t *testing.T) {
	script := GenerateCloudInitScript(nil)

	assert.Contains(t, script, "#cloud-config")
	assert.Contains(t, script, "/mnt/docker-data")
	assert.Contains(t, script, "docker-ce")
	assert.Contains(t, script, "2376")
	assert.Contains(t, script, "8080")
}

func TestGetDefaultCloudInitConfig(t *testing.T) {
	config := GetDefaultCloudInitConfig()

	assert.Equal(t, "latest", config.DockerVersion)
	assert.Equal(t, "/mnt/docker-data", config.VolumeMount)
	assert.Equal(t, 8080, config.KeepAlivePort)
	assert.Equal(t, 2376, config.DockerAPIPort)
	assert.Contains(t, config.Packages, "htop")
	assert.Contains(t, config.Packages, "vim")
}

func TestGetDefaultProvisionConfig(t *testing.T) {
	config := GetDefaultProvisionConfig()

	assert.Contains(t, config.ServerName, "dockbridge-")
	assert.Equal(t, "cpx21", config.ServerType)
	assert.Equal(t, "fsn1", config.Location)
	assert.Equal(t, 10, config.VolumeSize)
	assert.Equal(t, "/mnt/docker-data", config.VolumeMount)
	assert.Equal(t, 8080, config.KeepAlivePort)
	assert.Equal(t, 2376, config.DockerAPIPort)
}
