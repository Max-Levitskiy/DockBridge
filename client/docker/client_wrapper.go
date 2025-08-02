package docker

import (
	"context"

	"github.com/dockbridge/dockbridge/pkg/logger"
	"github.com/docker/docker/client"
)

// ClientWrapper wraps Docker's official client to redirect to remote server via SSH
type ClientWrapper struct {
	clientManager DockerClientManager
	logger        logger.LoggerInterface
}

// NewClientWrapper creates a new Docker client wrapper using the client manager
func NewClientWrapper(clientManager DockerClientManager, logger logger.LoggerInterface) (*ClientWrapper, error) {
	return &ClientWrapper{
		clientManager: clientManager,
		logger:        logger,
	}, nil
}

// GetDockerClient returns the underlying Docker client for direct use
func (cw *ClientWrapper) GetDockerClient(ctx context.Context) (*client.Client, error) {
	return cw.clientManager.GetClient(ctx)
}

// Close closes the Docker client
func (cw *ClientWrapper) Close() error {
	if cw.clientManager != nil {
		return cw.clientManager.Close()
	}
	return nil
}
