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

// GenerateCloudInitScript creates a cloud-init script for Docker CE installation with enhanced volume management
func GenerateCloudInitScript(config *CloudInitConfig) string {
	if config == nil {
		config = &CloudInitConfig{}
	}

	// Set defaults
	if config.DockerVersion == "" {
		config.DockerVersion = "latest"
	}
	if config.VolumeMount == "" {
		config.VolumeMount = "/var/lib/docker" // Docker's default data directory
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
  - e2fsprogs
  - parted
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

	// Add run commands for Docker installation and enhanced volume configuration
	script += `
# Run commands for system setup
runcmd:
  # Install Docker CE first (before volume operations)
  - curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg
  - echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null
  - apt-get update
  - apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
  
  # Stop Docker before volume operations
  - systemctl stop docker
  
  # Enhanced persistent volume setup for Docker data
  - |
    echo "Setting up persistent volume for Docker data..."
    
    # Wait for volume device to be available (up to 5 minutes)
    VOLUME_DEVICE=""
    for i in {1..60}; do
      # Check for common volume device names
      for device in /dev/sdb /dev/vdb /dev/xvdb; do
        if [ -b "$device" ]; then
          VOLUME_DEVICE="$device"
          echo "Found volume device: $VOLUME_DEVICE"
          break 2
        fi
      done
      echo "Waiting for volume device... attempt $i/60"
      sleep 5
    done
    
    if [ -z "$VOLUME_DEVICE" ]; then
      echo "ERROR: No volume device found after 5 minutes"
      echo "Available block devices:"
      lsblk
      exit 1
    fi
    
    # Create backup of existing Docker data if it exists
    if [ -d "` + config.VolumeMount + `" ] && [ "$(ls -A ` + config.VolumeMount + `)" ]; then
      echo "Backing up existing Docker data..."
      mkdir -p /tmp/docker-backup
      cp -a ` + config.VolumeMount + `/* /tmp/docker-backup/ 2>/dev/null || true
    fi
    
    # Check if volume is already formatted
    EXISTING_FS=$(blkid -o value -s TYPE "$VOLUME_DEVICE" 2>/dev/null || echo "")
    
    if [ -z "$EXISTING_FS" ]; then
      echo "Formatting volume with ext4 filesystem..."
      mkfs.ext4 -F -L "docker-data" "$VOLUME_DEVICE"
      echo "Volume formatted successfully"
    else
      echo "Volume already has filesystem: $EXISTING_FS"
      
      # If it's not ext4, reformat it
      if [ "$EXISTING_FS" != "ext4" ]; then
        echo "Converting filesystem to ext4..."
        mkfs.ext4 -F -L "docker-data" "$VOLUME_DEVICE"
      fi
    fi
    
    # Get volume UUID for reliable mounting
    VOLUME_UUID=$(blkid -s UUID -o value "$VOLUME_DEVICE")
    if [ -z "$VOLUME_UUID" ]; then
      echo "ERROR: Could not get volume UUID"
      exit 1
    fi
    
    echo "Volume UUID: $VOLUME_UUID"
    
    # Create mount point and mount volume
    mkdir -p ` + config.VolumeMount + `
    
    # Mount the volume
    if mount "$VOLUME_DEVICE" ` + config.VolumeMount + `; then
      echo "Volume mounted successfully at ` + config.VolumeMount + `"
    else
      echo "ERROR: Failed to mount volume"
      exit 1
    fi
    
    # Add to fstab for persistent mounting using UUID
    # Remove any existing entries for this mount point
    sed -i '\|` + config.VolumeMount + `|d' /etc/fstab
    echo "UUID=$VOLUME_UUID ` + config.VolumeMount + ` ext4 defaults,nofail,noatime 0 2" >> /etc/fstab
    
    # Set proper permissions for Docker directory
    chown root:root ` + config.VolumeMount + `
    chmod 755 ` + config.VolumeMount + `
    
    # Restore backed up Docker data if it exists
    if [ -d "/tmp/docker-backup" ] && [ "$(ls -A /tmp/docker-backup)" ]; then
      echo "Restoring Docker data from backup..."
      cp -a /tmp/docker-backup/* ` + config.VolumeMount + `/
      rm -rf /tmp/docker-backup
      echo "Docker data restored successfully"
    fi
    
    # Verify mount is working
    if mountpoint -q ` + config.VolumeMount + `; then
      echo "Volume mount verification successful"
      df -h ` + config.VolumeMount + `
    else
      echo "ERROR: Volume mount verification failed"
      exit 1
    fi
  
  # Configure Docker daemon with enhanced settings
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
      "experimental": false,
      "live-restore": true,
      "userland-proxy": false,
      "no-new-privileges": true
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
  
  # Create Docker data directory structure if it doesn't exist
  - mkdir -p ` + config.VolumeMount + `/{containers,image,network,plugins,swarm,tmp,trust,volumes}
  
  # Start and enable Docker
  - systemctl daemon-reload
  - systemctl enable docker
  - systemctl start docker
  
  # Verify Docker is using the persistent volume
  - |
    echo "Verifying Docker configuration..."
    sleep 10  # Wait for Docker to fully start
    
    # Check Docker info
    docker info | grep "Docker Root Dir" || echo "Could not verify Docker root directory"
    
    # Test Docker functionality
    if docker run --rm hello-world > /dev/null 2>&1; then
      echo "Docker functionality test passed"
    else
      echo "WARNING: Docker functionality test failed"
    fi
    
    # Check volume usage
    df -h ` + config.VolumeMount + ` || echo "Could not check volume usage"
  
  # Add ubuntu user to docker group
  - usermod -aG docker ubuntu
  
  # Install DockBridge server component (placeholder for future implementation)
  - |
    cat > /usr/local/bin/dockbridge-server << 'EOF'
    #!/bin/bash
    # DockBridge server placeholder
    # This will be replaced with actual server binary
    echo "DockBridge server starting on port ` + fmt.Sprintf("%d", config.KeepAlivePort) + `"
    echo "Docker data directory: ` + config.VolumeMount + `"
    echo "Volume mount status: $(mountpoint ` + config.VolumeMount + ` && echo 'mounted' || echo 'not mounted')"
    
    while true; do
      # Simple health check
      if ! mountpoint -q ` + config.VolumeMount + `; then
        echo "ERROR: Docker volume not mounted!"
      fi
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
  
  # Set up enhanced log rotation for Docker
  - |
    cat > /etc/logrotate.d/docker << 'EOF'
    ` + config.VolumeMount + `/containers/*/*.log {
      rotate 7
      daily
      compress
      size=1M
      missingok
      delaycompress
      copytruncate
    }
    EOF
  
  # Create volume health check script
  - |
    cat > /usr/local/bin/docker-volume-health-check << 'EOF'
    #!/bin/bash
    # Docker volume health check script
    
    MOUNT_POINT="` + config.VolumeMount + `"
    
    # Check if volume is mounted
    if ! mountpoint -q "$MOUNT_POINT"; then
      echo "ERROR: Docker volume not mounted at $MOUNT_POINT"
      exit 1
    fi
    
    # Check if volume is writable
    if ! touch "$MOUNT_POINT/.health-check" 2>/dev/null; then
      echo "ERROR: Docker volume not writable at $MOUNT_POINT"
      exit 1
    fi
    rm -f "$MOUNT_POINT/.health-check"
    
    # Check disk space (warn if less than 1GB free)
    AVAILABLE=$(df --output=avail "$MOUNT_POINT" | tail -1)
    if [ "$AVAILABLE" -lt 1048576 ]; then  # 1GB in KB
      echo "WARNING: Low disk space on Docker volume: ${AVAILABLE}KB available"
    fi
    
    echo "Docker volume health check passed"
    exit 0
    EOF
  
  - chmod +x /usr/local/bin/docker-volume-health-check
  
  # Add cron job for volume health checks
  - echo "*/5 * * * * root /usr/local/bin/docker-volume-health-check >> /var/log/docker-volume-health.log 2>&1" >> /etc/crontab
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
# Write completion marker with enhanced information
write_files:
  - path: /var/log/cloud-init-complete
    content: |
      Cloud-init setup completed at $(date)
      Docker version: $(docker --version 2>/dev/null || echo "Not available")
      Docker data directory: ` + config.VolumeMount + `
      Volume mount status: $(mountpoint ` + config.VolumeMount + ` && echo 'mounted' || echo 'not mounted')
      Volume filesystem: $(df -T ` + config.VolumeMount + ` | tail -1 | awk '{print $2}' 2>/dev/null || echo "Unknown")
      Available space: $(df -h ` + config.VolumeMount + ` | tail -1 | awk '{print $4}' 2>/dev/null || echo "Unknown")
      DockBridge server status: $(systemctl is-active dockbridge-server 2>/dev/null || echo "Not available")
    permissions: '0644'
  
  - path: /etc/dockbridge/volume-info
    content: |
      # DockBridge Volume Information
      DOCKER_DATA_DIR=` + config.VolumeMount + `
      VOLUME_DEVICE=$(findmnt -n -o SOURCE ` + config.VolumeMount + ` 2>/dev/null || echo "unknown")
      VOLUME_UUID=$(findmnt -n -o UUID ` + config.VolumeMount + ` 2>/dev/null || echo "unknown")
      SETUP_DATE=$(date)
    permissions: '0644'

# Final message
final_message: "DockBridge server with enhanced volume management setup completed successfully"
`

	return strings.TrimSpace(script)
}

// GetDefaultCloudInitConfig returns a default cloud-init configuration with enhanced volume settings
func GetDefaultCloudInitConfig() *CloudInitConfig {
	return &CloudInitConfig{
		DockerVersion: "latest",
		VolumeMount:   "/var/lib/docker", // Docker's default data directory
		KeepAlivePort: 8080,
		DockerAPIPort: 2376,
		Packages: []string{
			"htop",
			"vim",
			"curl",
			"wget",
			"unzip",
			"e2fsprogs",
			"parted",
		},
	}
}
