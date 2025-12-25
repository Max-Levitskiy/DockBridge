package hetzner

import (
	"context"
	"fmt"
	"log"
	"os"

)

// ExampleUsage demonstrates how to use the Hetzner Cloud API client
func ExampleUsage() {
	// Get API token from environment
	apiToken := os.Getenv("HETZNER_API_TOKEN")
	if apiToken == "" {
		log.Fatal("HETZNER_API_TOKEN environment variable is required")
	}

	// Create client configuration
	config := &Config{
		APIToken:   apiToken,
		ServerType: "cpx21",
		Location:   "fsn1",
		VolumeSize: 10,
	}

	// Create Hetzner client
	client, err := NewClient(config)
	if err != nil {
		log.Fatalf("Failed to create Hetzner client: %v", err)
	}

	// Create lifecycle manager
	lifecycleManager := NewLifecycleManager(client)

	ctx := context.Background()

	// Example: Provision a server with volume
	provisionConfig := &ServerProvisionConfig{
		ServerName:    "dockbridge-example",
		ServerType:    "cpx21",
		Location:      "fsn1",
		VolumeSize:    10,
		SSHPublicKey:  "ssh-rsa AAAAB3NzaC1yc2E... your-public-key-here",
		KeepAlivePort: 8080,
		DockerAPIPort: 2376,
	}

	fmt.Println("Provisioning server with volume...")
	serverWithVolume, err := lifecycleManager.ProvisionServerWithVolume(ctx, provisionConfig)
	if err != nil {
		log.Fatalf("Failed to provision server: %v", err)
	}

	fmt.Printf("Server provisioned successfully:\n")
	fmt.Printf("  ID: %d\n", serverWithVolume.Server.ID)
	fmt.Printf("  Name: %s\n", serverWithVolume.Server.Name)
	fmt.Printf("  IP: %s\n", serverWithVolume.Server.IPAddress)
	fmt.Printf("  Status: %s\n", serverWithVolume.Server.Status)

	if serverWithVolume.Volume != nil {
		fmt.Printf("  Volume ID: %d\n", serverWithVolume.Volume.ID)
		fmt.Printf("  Volume Size: %d GB\n", serverWithVolume.Volume.Size)
	}

	// Example: List all servers
	fmt.Println("\nListing all servers...")
	servers, err := client.ListServers(ctx)
	if err != nil {
		log.Printf("Failed to list servers: %v", err)
	} else {
		for _, server := range servers {
			fmt.Printf("  Server: %s (ID: %d, Status: %s)\n", server.Name, server.ID, server.Status)
		}
	}

	// Example: Clean up (uncomment to actually destroy the server)
	/*
		fmt.Println("\nDestroying server with volume preservation...")
		err = lifecycleManager.DestroyServerWithCleanup(ctx, fmt.Sprintf("%d", serverWithVolume.Server.ID), true)
		if err != nil {
			log.Printf("Failed to destroy server: %v", err)
		} else {
			fmt.Println("Server destroyed successfully, volume preserved")
		}
	*/
}

// ExampleCloudInitGeneration demonstrates cloud-init script generation
func ExampleCloudInitGeneration() {
	fmt.Println("Generating cloud-init script...")

	config := &CloudInitConfig{
		DockerVersion: "latest",
		SSHPublicKey:  "ssh-rsa AAAAB3NzaC1yc2E... your-public-key-here",
		VolumeMount:   "/var/lib/docker",
		KeepAlivePort: 8080,
		DockerAPIPort: 2376,
		Packages: []string{
			"htop",
			"vim",
			"curl",
			"wget",
		},
		RunCommands: []string{
			"echo 'Custom setup complete'",
		},
	}

	script := GenerateCloudInitScript(config)
	fmt.Println("Generated cloud-init script:")
	fmt.Println("---")
	fmt.Println(script)
	fmt.Println("---")
}

// ExampleBasicOperations demonstrates basic CRUD operations
func ExampleBasicOperations() {
	apiToken := os.Getenv("HETZNER_API_TOKEN")
	if apiToken == "" {
		log.Fatal("HETZNER_API_TOKEN environment variable is required")
	}

	config := &Config{
		APIToken:   apiToken,
		ServerType: "cpx21",
		Location:   "fsn1",
		VolumeSize: 10,
	}

	client, err := NewClient(config)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	ctx := context.Background()

	// Create a volume
	fmt.Println("Creating volume...")
	volume, err := client.CreateVolume(ctx, 10, "fsn1")
	if err != nil {
		log.Fatalf("Failed to create volume: %v", err)
	}
	fmt.Printf("Volume created: %s (ID: %d)\n", volume.Name, volume.ID)

	// Upload SSH key
	fmt.Println("Uploading SSH key...")
	sshKey, err := client.ManageSSHKeys(ctx, "ssh-rsa AAAAB3NzaC1yc2E... your-public-key-here")
	if err != nil {
		log.Fatalf("Failed to upload SSH key: %v", err)
	}
	fmt.Printf("SSH key uploaded: %s (ID: %d)\n", sshKey.Name, sshKey.ID)

	// Generate cloud-init script
	cloudInitConfig := GetDefaultCloudInitConfig()
	cloudInitConfig.SSHPublicKey = "ssh-rsa AAAAB3NzaC1yc2E... your-public-key-here"
	userDataScript := GenerateCloudInitScript(cloudInitConfig)

	// Create server
	fmt.Println("Creating server...")
	serverConfig := &ServerConfig{
		Name:       "dockbridge-example",
		ServerType: "cpx21",
		Location:   "fsn1",
		SSHKeyID:   sshKey.ID,
		VolumeID:   fmt.Sprintf("%d", volume.ID),
		UserData:   userDataScript,
	}

	server, err := client.ProvisionServer(ctx, serverConfig)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}
	fmt.Printf("Server created: %s (ID: %d, IP: %s)\n", server.Name, server.ID, server.IPAddress)

	fmt.Println("Example completed successfully!")
}
