package hetzner

import (
	"fmt"
	"strings"
)

// CloudInitConfig holds configuration for cloud-init script generation
type CloudInitConfig struct {
	DockerVersion   string
	SSHPublicKey    string
	VolumeMount     string
	KeepAlivePort   int
	DockerAPIPort   int
	AdditionalUsers []string
	Packages        []string
	RunCommands     []string
}

// GenerateCloudInitScript creates a cloud-init script for Docker CE installation
func GenerateCloudInitScript(config *CloudInitConfig) string {
	if config == nil {
		config = &CloudInitConfig{}
	}

	// Set defaults
	if config.DockerVersion == "" {
		config.DockerVersion = "latest"
	}
	if config.VolumeMount == "" {
		config.VolumeMount = "/var/lib/docker" // Changed to Docker's default data directory
	}
	if config.KeepAlivePort == 0 {
		config.KeepAlivePort = 8080
	}
	if config.DockerAPIPort == 0 {
		config.DockerAPIPort = 2376
	}

	script := `#cloud-config

# Update package index
package_update: true
package_upgrade: true

# Install required packages
packages:
  - apt-transport-https
  - ca-certificates
  - curl
  - gnupg
  - lsb-release
  - software-properties-common
  - unzip
  - wget
  - htop
  - vim
`

	// Add additional packages if specified
	if len(config.Packages) > 0 {
		for _, pkg := range config.Packages {
			script += fmt.Sprintf("  - %s\n", pkg)
		}
	}

	// Add SSH key if provided
	if config.SSHPublicKey != "" {
		script += fmt.Sprintf(`
# Configure SSH access
ssh_authorized_keys:
  - %s
`, config.SSHPublicKey)
	}

	// Add additional users if specified
	if len(config.AdditionalUsers) > 0 {
		script += "\n# Create additional users\nusers:\n"
		for _, user := range config.AdditionalUsers {
			script += fmt.Sprintf(`  - name: %s
    groups: docker,sudo
    shell: /bin/bash
    sudo: ALL=(ALL) NOPASSWD:ALL
`, user)
		}
	}

	// Add run commands for Docker installation and configuration
	script += `
# Run commands for system setup
runcmd:
  # Create docker data directory
  - mkdir -p ` + config.VolumeMount + `
  
  # Install Docker CE
  - curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg
  - echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null
  - apt-get update
  - apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
  
  # Configure Docker daemon
  - mkdir -p /etc/docker
  - |
    cat > /etc/docker/daemon.json << 'EOF'
    {
      "data-root": "` + config.VolumeMount + `",
      "storage-driver": "overlay2",
      "log-driver": "json-file",
      "log-opts": {
        "max-size": "10m",
        "max-file": "3"
      },
      "hosts": ["unix:///var/run/docker.sock", "tcp://0.0.0.0:` + fmt.Sprintf("%d", config.DockerAPIPort) + `"],
      "tls": false,
      "experimental": false
    }
    EOF
  
  # Create systemd override for Docker daemon
  - mkdir -p /etc/systemd/system/docker.service.d
  - |
    cat > /etc/systemd/system/docker.service.d/override.conf << 'EOF'
    [Service]
    ExecStart=
    ExecStart=/usr/bin/dockerd
    EOF
  
  # Start and enable Docker
  - systemctl daemon-reload
  - systemctl enable docker
  - systemctl start docker
  
  # Add ubuntu user to docker group
  - usermod -aG docker ubuntu
  
  # Configure persistent volume for Docker data
  - |
    # Wait for volume to be available
    for i in {1..60}; do
      if [ -b /dev/sdb ]; then
        echo "Volume device /dev/sdb is available"
        break
      fi
      echo "Waiting for volume device... attempt $i/60"
      sleep 5
    done
    
    if [ -b /dev/sdb ]; then
      # Stop Docker before mounting volume
      systemctl stop docker
      
      # Format volume if not already formatted
      if ! blkid /dev/sdb | grep -q "TYPE="; then
        echo "Formatting volume with ext4 filesystem"
        mkfs.ext4 -F /dev/sdb
      else
        echo "Volume already has a filesystem"
      fi
      
      # Get volume UUID for reliable mounting
      VOLUME_UUID=$(blkid -s UUID -o value /dev/sdb)
      
      # Create mount point and mount volume
      mkdir -p ` + config.VolumeMount + `
      mount /dev/sdb ` + config.VolumeMount + `
      
      # Add to fstab for persistent mounting using UUID
      echo "UUID=$VOLUME_UUID ` + config.VolumeMount + ` ext4 defaults,nofail 0 2" >> /etc/fstab
      
      # Set proper permissions for Docker directory
      chown root:root ` + config.VolumeMount + `
      chmod 755 ` + config.VolumeMount + `
      
      # Restart Docker to use the mounted volume
      systemctl start docker
      
      echo "Docker volume mounted successfully at ` + config.VolumeMount + `"
    else
      echo "Warning: No volume device found, Docker will use local storage"
    fi
  
  # Install DockBridge server component (placeholder for future implementation)
  - |
    cat > /usr/local/bin/dockbridge-server << 'EOF'
    #!/bin/bash
    # DockBridge server placeholder
    # This will be replaced with actual server binary
    echo "DockBridge server starting on port ` + fmt.Sprintf("%d", config.KeepAlivePort) + `"
    while true; do
      sleep 30
    done
    EOF
  
  - chmod +x /usr/local/bin/dockbridge-server
  
  # Create systemd service for DockBridge server
  - |
    cat > /etc/systemd/system/dockbridge-server.service << 'EOF'
    [Unit]
    Description=DockBridge Server
    After=docker.service
    Requires=docker.service
    
    [Service]
    Type=simple
    User=root
    ExecStart=/usr/local/bin/dockbridge-server
    Restart=always
    RestartSec=10
    
    [Install]
    WantedBy=multi-user.target
    EOF
  
  # Enable and start DockBridge server
  - systemctl daemon-reload
  - systemctl enable dockbridge-server
  - systemctl start dockbridge-server
  
  # Configure firewall
  - ufw allow ssh
  - ufw allow ` + fmt.Sprintf("%d", config.DockerAPIPort) + `/tcp
  - ufw allow ` + fmt.Sprintf("%d", config.KeepAlivePort) + `/tcp
  - ufw --force enable
  
  # Set up log rotation for Docker
  - |
    cat > /etc/logrotate.d/docker << 'EOF'
    /var/lib/docker/containers/*/*.log {
      rotate 7
      daily
      compress
      size=1M
      missingok
      delaycompress
      copytruncate
    }
    EOF
`

	// Add additional run commands if specified
	if len(config.RunCommands) > 0 {
		script += "\n  # Additional custom commands\n"
		for _, cmd := range config.RunCommands {
			script += fmt.Sprintf("  - %s\n", cmd)
		}
	}

	// Add final configuration
	script += `
# Write completion marker
write_files:
  - path: /var/log/cloud-init-complete
    content: |
      Cloud-init setup completed at $(date)
      Docker version: $(docker --version)
      DockBridge server status: $(systemctl is-active dockbridge-server)
    permissions: '0644'

# Final message
final_message: "DockBridge server setup completed successfully"
`

	return strings.TrimSpace(script)
}

// GetDefaultCloudInitConfig returns a default cloud-init configuration
func GetDefaultCloudInitConfig() *CloudInitConfig {
	return &CloudInitConfig{
		DockerVersion: "latest",
		VolumeMount:   "/var/lib/docker", // Changed to Docker's default data directory
		KeepAlivePort: 8080,
		DockerAPIPort: 2376,
		Packages: []string{
			"htop",
			"vim",
			"curl",
			"wget",
			"unzip",
		},
	}
}
