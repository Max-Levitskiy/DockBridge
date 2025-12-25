package hetzner

import (
	"strings"
	"testing"
)

func TestGenerateCloudInitForImage(t *testing.T) {
	config := GetDefaultCloudInitConfig()

	tests := []struct {
		name      string
		imageName string
		expectOpt bool // expect optimized version
	}{
		{
			name:      "Docker CE image should use optimized script",
			imageName: "docker-ce",
			expectOpt: true,
		},
		{
			name:      "Docker image should use optimized script",
			imageName: "docker",
			expectOpt: true,
		},
		{
			name:      "Ubuntu image should use full installation script",
			imageName: "ubuntu-22.04",
			expectOpt: false,
		},
		{
			name:      "Other image should use full installation script",
			imageName: "centos-7",
			expectOpt: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			script := GenerateCloudInitForImage(config, tt.imageName)

			if tt.expectOpt {
				// Optimized script should skip package updates
				if !strings.Contains(script, "package_update: false") {
					t.Errorf("Expected optimized script with package_update: false, got: %s", script)
				}
				// Should not contain Docker installation commands
				if strings.Contains(script, "curl -fsSL https://download.docker.com") {
					t.Errorf("Expected optimized script without Docker installation, but found Docker installation commands")
				}
			} else {
				// Full script should include package updates
				if !strings.Contains(script, "package_update: true") {
					t.Errorf("Expected full script with package_update: true, got: %s", script)
				}
				// Should contain Docker installation commands
				if !strings.Contains(script, "curl -fsSL https://download.docker.com") {
					t.Errorf("Expected full script with Docker installation, but Docker installation commands not found")
				}
			}

			// Both scripts should contain volume setup
			if !strings.Contains(script, "Enhanced persistent volume setup for Docker data") {
				t.Errorf("Expected volume setup in script, but not found")
			}

			// Both scripts should contain DockBridge server setup
			if !strings.Contains(script, "DockBridge server placeholder") {
				t.Errorf("Expected DockBridge server setup in script, but not found")
			}
		})
	}
}

func TestGenerateOptimizedCloudInitScript(t *testing.T) {
	config := &CloudInitConfig{
		DockerVersion: "latest",
		VolumeMount:   "/var/lib/docker",
		KeepAlivePort: 8080,
		DockerAPIPort: 2376,
		SSHPublicKey:  "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQ...",
		Packages:      []string{"curl", "wget"},
	}

	script := generateOptimizedCloudInitScript(config)

	// Check optimized characteristics
	if !strings.Contains(script, "package_update: false") {
		t.Error("Expected package_update: false in optimized script")
	}

	if !strings.Contains(script, "package_upgrade: false") {
		t.Error("Expected package_upgrade: false in optimized script")
	}

	// Should contain SSH key
	if !strings.Contains(script, config.SSHPublicKey) {
		t.Error("Expected SSH key in script")
	}

	// Should contain additional packages
	if !strings.Contains(script, "curl") || !strings.Contains(script, "wget") {
		t.Error("Expected additional packages in script")
	}

	// Should contain volume mount path
	if !strings.Contains(script, config.VolumeMount) {
		t.Error("Expected volume mount path in script")
	}

	// Should contain ports
	if !strings.Contains(script, "8080") || !strings.Contains(script, "2376") {
		t.Error("Expected configured ports in script")
	}
}

func TestGenerateFullDockerInstallScript(t *testing.T) {
	config := &CloudInitConfig{
		DockerVersion: "latest",
		VolumeMount:   "/var/lib/docker",
		KeepAlivePort: 8080,
		DockerAPIPort: 2376,
	}

	script := generateFullDockerInstallScript(config)

	// Check full installation characteristics
	if !strings.Contains(script, "package_update: true") {
		t.Error("Expected package_update: true in full script")
	}

	if !strings.Contains(script, "package_upgrade: true") {
		t.Error("Expected package_upgrade: true in full script")
	}

	// Should contain Docker installation
	if !strings.Contains(script, "curl -fsSL https://download.docker.com") {
		t.Error("Expected Docker installation commands in full script")
	}

	if !strings.Contains(script, "apt-get install -y docker-ce") {
		t.Error("Expected Docker CE installation in full script")
	}

	// Should still contain volume setup
	if !strings.Contains(script, "Enhanced persistent volume setup for Docker data") {
		t.Error("Expected volume setup in full script")
	}
}

func TestGetDefaultCloudInitConfig(t *testing.T) {
	config := GetDefaultCloudInitConfig()

	if config.DockerVersion != "latest" {
		t.Errorf("Expected DockerVersion 'latest', got %s", config.DockerVersion)
	}

	if config.VolumeMount != "/var/lib/docker" {
		t.Errorf("Expected VolumeMount '/var/lib/docker', got %s", config.VolumeMount)
	}

	if config.KeepAlivePort != 8080 {
		t.Errorf("Expected KeepAlivePort 8080, got %d", config.KeepAlivePort)
	}

	if config.DockerAPIPort != 2376 {
		t.Errorf("Expected DockerAPIPort 2376, got %d", config.DockerAPIPort)
	}

	// Should have reduced package list for faster startup
	expectedPackages := []string{"htop", "vim", "e2fsprogs", "parted"}
	if len(config.Packages) != len(expectedPackages) {
		t.Errorf("Expected %d packages, got %d", len(expectedPackages), len(config.Packages))
	}

	for i, pkg := range expectedPackages {
		if i >= len(config.Packages) || config.Packages[i] != pkg {
			t.Errorf("Expected package %s at index %d, got %s", pkg, i, config.Packages[i])
		}
	}
}
