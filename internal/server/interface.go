package server

import "context"

// ServerManager defines the interface for server lifecycle management with enhanced volume support
type ServerManager interface {
	// EnsureServer ensures a server exists with proper Docker state persistence
	EnsureServer(ctx context.Context) (*ServerInfo, error)

	// DestroyServer destroys a server while preserving the volume for future use
	DestroyServer(ctx context.Context, serverID string) error

	// GetServerStatus retrieves the current status of the active server
	GetServerStatus(ctx context.Context) (*ServerStatus, error)

	// ListServers retrieves all DockBridge servers
	ListServers(ctx context.Context) ([]*ServerInfo, error)

	// EnsureVolume ensures a Docker data volume exists and is available
	EnsureVolume(ctx context.Context) (*VolumeInfo, error)
}
