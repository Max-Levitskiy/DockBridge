package gui

import (
	"context"
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

	// Find DockBridge server
	for _, srv := range servers {
		if len(srv.Name) >= 10 && srv.Name[:10] == "dockbridge" {
			c.state.SetServerIP(srv.IPAddress)
			c.log.WithFields(map[string]any{
				"server_id": srv.ID,
				"server_ip": srv.IPAddress,
			}).Info("Found DockBridge server")
			return
		}
	}
}
