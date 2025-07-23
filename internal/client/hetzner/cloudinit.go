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
		config.VolumeMount = "/mnt/docker-data"
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
      "data-root": "` + config.VolumeMount + `/docker",
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
  
  # Configure volume mounting if attached
  - |
    if [ -b /dev/sdb ]; then
      # Format volume if not already formatted
      if ! blkid /dev/sdb; then
        mkfs.ext4 /dev/sdb
      fi
      
      # Mount volume
      mount /dev/sdb ` + config.VolumeMount + `
      
      # Add to fstab for persistent mounting
      echo "/dev/sdb ` + config.VolumeMount + ` ext4 defaults 0 2" >> /etc/fstab
      
      # Create docker directory on volume
      mkdir -p ` + config.VolumeMount + `/docker
      chown root:docker ` + config.VolumeMount + `/docker
      chmod 755 ` + config.VolumeMount + `/docker
      
      # Restart Docker to use new data directory
      systemctl restart docker
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
		VolumeMount:   "/mnt/docker-data",
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
