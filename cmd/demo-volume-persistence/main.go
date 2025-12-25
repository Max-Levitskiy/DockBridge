package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/dockbridge/dockbridge/client/hetzner"
	"github.com/dockbridge/dockbridge/server"
	"github.com/dockbridge/dockbridge/shared/config"
)

func main() {
	fmt.Println("DockBridge Volume Persistence Demo")
	fmt.Println("==================================")

	// Check for API token
	apiToken := os.Getenv("HETZNER_API_TOKEN")
	if apiToken == "" {
		log.Fatal("HETZNER_API_TOKEN environment variable is required")
	}

	// Create Hetzner client configuration
	hetznerConfig := &hetzner.Config{
		APIToken:   apiToken,
		ServerType: "cpx11", // Use smallest server for demo
		Location:   "fsn1",
		VolumeSize: 10,
	}

	clientConfig := &config.HetznerConfig{
		APIToken:   apiToken,
		ServerType: "cpx11",
		Location:   "fsn1",
		VolumeSize: 10,
	}

	// Create Hetzner client
	hetznerClient, err := hetzner.NewClient(hetznerConfig)
	if err != nil {
		log.Fatalf("Failed to create Hetzner client: %v", err)
	}

	// Create server manager
	serverManager := server.NewManager(hetznerClient, clientConfig)
	ctx := context.Background()

	fmt.Println("\n1. Ensuring Docker data volume exists...")
	volume, err := serverManager.EnsureVolume(ctx)
	if err != nil {
		log.Fatalf("Failed to ensure volume: %v", err)
	}
	fmt.Printf("✓ Docker data volume ready: %s (Size: %dGB, Mount: %s)\n",
		volume.Name, volume.Size, volume.MountPath)

	fmt.Println("\n2. Provisioning server with persistent volume...")
	server, err := serverManager.EnsureServer(ctx)
	if err != nil {
		log.Fatalf("Failed to ensure server: %v", err)
	}
	fmt.Printf("✓ Server provisioned: %s (ID: %s)\n", server.Name, server.ID)
	fmt.Printf("  - Status: %s\n", server.Status)
	fmt.Printf("  - IP Address: %s\n", server.IPAddress)
	fmt.Printf("  - Volume ID: %s\n", server.VolumeID)
	fmt.Printf("  - Docker Data Dir: %s\n", server.Metadata["docker_data_dir"])

	fmt.Println("\n3. Waiting for server to be fully ready...")
	fmt.Println("   (This includes Docker installation and volume mounting)")
	time.Sleep(3 * time.Minute)

	fmt.Println("\n4. Checking server status...")
	status, err := serverManager.GetServerStatus(ctx)
	if err != nil {
		log.Fatalf("Failed to get server status: %v", err)
	}
	fmt.Printf("✓ Server status: %s\n", *status)

	fmt.Println("\n5. Listing all DockBridge servers...")
	servers, err := serverManager.ListServers(ctx)
	if err != nil {
		log.Fatalf("Failed to list servers: %v", err)
	}
	fmt.Printf("✓ Found %d DockBridge server(s):\n", len(servers))
	for i, srv := range servers {
		fmt.Printf("  %d. %s (ID: %s, Status: %s, Volume: %s)\n",
			i+1, srv.Name, srv.ID, srv.Status, srv.VolumeID)
	}

	fmt.Println("\n6. Demonstrating volume persistence...")
	fmt.Printf("   Server ID to destroy: %s\n", server.ID)
	fmt.Println("   This will destroy the server but preserve the volume for Docker state persistence")

	// Ask for confirmation
	fmt.Print("\nProceed with server destruction demo? (y/N): ")
	var response string
	fmt.Scanln(&response)

	if response == "y" || response == "Y" {
		fmt.Println("\n7. Destroying server (preserving volume)...")
		err = serverManager.DestroyServer(ctx, server.ID)
		if err != nil {
			log.Fatalf("Failed to destroy server: %v", err)
		}
		fmt.Printf("✓ Server %s destroyed, volume %s preserved\n", server.ID, server.VolumeID)

		fmt.Println("\n8. Waiting for server destruction to complete...")
		time.Sleep(30 * time.Second)

		fmt.Println("\n9. Recreating server with same volume...")
		newServer, err := serverManager.EnsureServer(ctx)
		if err != nil {
			log.Fatalf("Failed to recreate server: %v", err)
		}
		fmt.Printf("✓ New server created: %s (ID: %s)\n", newServer.Name, newServer.ID)
		fmt.Printf("  - Volume ID: %s (should be same as before)\n", newServer.VolumeID)
		fmt.Printf("  - Docker Data Dir: %s\n", newServer.Metadata["docker_data_dir"])

		if newServer.VolumeID == server.VolumeID {
			fmt.Println("✓ SUCCESS: Same volume reused - Docker state will be preserved!")
		} else {
			fmt.Println("⚠ WARNING: Different volume used - this shouldn't happen")
		}

		fmt.Println("\n10. Final cleanup...")
		fmt.Printf("    New server ID: %s\n", newServer.ID)
		fmt.Printf("    Volume ID: %s\n", newServer.VolumeID)
		fmt.Println("    Remember to clean up resources to avoid ongoing costs!")

		// Optionally destroy the new server too
		fmt.Print("\nDestroy the new server as well? (y/N): ")
		fmt.Scanln(&response)
		if response == "y" || response == "Y" {
			err = serverManager.DestroyServer(ctx, newServer.ID)
			if err != nil {
				log.Printf("Failed to destroy new server: %v", err)
			} else {
				fmt.Printf("✓ New server %s destroyed\n", newServer.ID)
			}
		}
	} else {
		fmt.Println("\nDemo cancelled. Server remains running.")
		fmt.Printf("Server ID: %s\n", server.ID)
		fmt.Printf("Volume ID: %s\n", server.VolumeID)
		fmt.Println("Remember to clean up resources to avoid ongoing costs!")
	}

	fmt.Println("\nDemo completed!")
	fmt.Println("\nKey features demonstrated:")
	fmt.Println("- Automatic Docker data volume creation")
	fmt.Println("- Volume mounting at /var/lib/docker for complete Docker state persistence")
	fmt.Println("- Server destruction while preserving volumes")
	fmt.Println("- Volume reuse when recreating servers")
	fmt.Println("- Enhanced cloud-init script with robust volume management")
}
