package hetzner

import (
	"strings"
	"testing"

	"github.com/dockbridge/dockbridge/client/hetzner"
	"github.com/stretchr/testify/assert"
)

func TestGenerateDockerCloudInitScript(t *testing.T) {
	tests := []struct {
		name             string
		publicKey        string
		volumeMount      string
		expectedContains []string
	}{
		{
			name:        "basic script generation",
			publicKey:   "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC7... test@example.com",
			volumeMount: "/var/lib/docker",
			expectedContains: []string{
				"#cloud-config",
				"ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC7... test@example.com",
				"/dev/sdb",
				"/var/lib/docker",
				"mkfs.ext4",
				"mount",
				"fstab",
				"docker-ce",
				"data-root",
				"systemctl enable docker",
			},
		},
		{
			name:        "script with special characters in key",
			publicKey:   `ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC7... "test user"@example.com`,
			volumeMount: "/var/lib/docker",
			expectedContains: []string{
				`ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC7... "test user"@example.com`,
				"/var/lib/docker",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &hetzner.CloudInitConfig{
				SSHPublicKey: tt.publicKey,
				VolumeMount:  tt.volumeMount,
			}
			script := hetzner.GenerateCloudInitScript(config)

			// Check that script is not empty
			assert.NotEmpty(t, script)

			// Check that script starts with cloud-config
			assert.True(t, strings.HasPrefix(script, "#cloud-config"))

			// Check for all expected content
			for _, expected := range tt.expectedContains {
				assert.Contains(t, script, expected, "Script should contain: %s", expected)
			}

			// Verify script structure
			lines := strings.Split(script, "\n")
			assert.Greater(t, len(lines), 50, "Script should have substantial content")

			// Check for volume-specific operations
			assert.Contains(t, script, "blkid", "Script should check filesystem")
			assert.Contains(t, script, "UUID=", "Script should use UUID for mounting")
			assert.Contains(t, script, "chmod 755", "Script should set proper permissions")

			// Check for Docker configuration
			assert.Contains(t, script, "daemon.json", "Script should configure Docker daemon")
			assert.Contains(t, script, `"data-root"`, "Script should set Docker data root")
		})
	}
}

func TestGetDefaultCloudInitConfig(t *testing.T) {
	config := hetzner.GetDefaultCloudInitConfig()
	assert.Equal(t, "/var/lib/docker", config.VolumeMount)
	assert.Equal(t, 2376, config.DockerAPIPort)
	assert.Equal(t, 8080, config.KeepAlivePort)
}

func TestCloudInitScriptDockerConfiguration(t *testing.T) {
	config := &hetzner.CloudInitConfig{
		SSHPublicKey:  "ssh-rsa test",
		VolumeMount:   "/var/lib/docker",
		DockerAPIPort: 2376,
	}
	script := hetzner.GenerateCloudInitScript(config)

	// Check Docker configuration
	assert.Contains(t, script, `"data-root": "/var/lib/docker"`, "Script should set Docker data root")
	assert.Contains(t, script, "tcp://0.0.0.0:2376", "Script should configure Docker API port")
	assert.Contains(t, script, "daemon.json", "Script should create daemon.json")
}

func TestCloudInitScriptVolumeOperations(t *testing.T) {
	config := &hetzner.CloudInitConfig{
		SSHPublicKey: "ssh-rsa test",
		VolumeMount:  "/var/lib/docker",
	}
	script := hetzner.GenerateCloudInitScript(config)

	// Check volume operations are present
	assert.Contains(t, script, "Waiting for volume device", "Script should wait for volume")
	assert.Contains(t, script, "mkfs.ext4", "Script should format volume")
	assert.Contains(t, script, "mount /dev/sdb", "Script should mount volume")
	assert.Contains(t, script, "fstab", "Script should add to fstab")
	assert.Contains(t, script, "systemctl stop docker", "Script should stop Docker before mounting")
	assert.Contains(t, script, "systemctl start docker", "Script should start Docker after mounting")
}
