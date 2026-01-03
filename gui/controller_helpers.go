package gui

import (
	"context"

	"github.com/Max-Levitskiy/DockBridge/client/hetzner"
)

// updateServerIP fetches and updates the server IP in state.
func (c *Controller) updateServerIP() {
	c.mu.Lock()
	hetznerClient := c.hetznerClient
	c.mu.Unlock()

	if hetznerClient == nil {
		return
	}

	ctx := context.Background()
	servers, err := hetznerClient.ListServers(ctx)
	if err != nil {
		c.log.WithFields(map[string]any{"error": err.Error()}).Error("Failed to list servers")
		return
	}

	// Find running DockBridge server
	// We iterate through all servers and find the one that is running and has an IP
	var targetServer *hetzner.Server
	for _, srv := range servers {
		if len(srv.Name) >= 10 && srv.Name[:10] == "dockbridge" {
			// Prioritize running servers
			if srv.Status == "running" {
				targetServer = srv
				break
			}
			// Fallback to any dockbridge server if none running found yet
			if targetServer == nil {
				targetServer = srv
			}
		}
	}

	if targetServer != nil {
		c.state.SetServerIP(targetServer.IPAddress)
		c.log.WithFields(map[string]any{
			"server_id": targetServer.ID,
			"server_ip": targetServer.IPAddress,
			"status":    targetServer.Status,
		}).Info("Found DockBridge server")
	} else {
		// No server found
		c.state.SetServerIP("")
	}
}
