package portforward

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/dockbridge/dockbridge/internal/client/monitor"
	"github.com/dockbridge/dockbridge/internal/shared/config"
	"github.com/dockbridge/dockbridge/pkg/logger"
)

// PortForwardManager manages the lifecycle of port forwards
type PortForwardManager interface {
	Start(ctx context.Context) error
	Stop() error

	// Container lifecycle events (implements monitor.ContainerEventHandler)
	OnContainerCreated(container *monitor.ContainerInfo) error
	OnContainerStopped(containerID string) error
	OnContainerRemoved(containerID string) error

	// Manual port management
	AddPortForward(containerID string, localPort, remotePort int) error
	RemovePortForward(containerID string, localPort int) error

	// Status and information
	ListPortForwards() ([]*PortForward, error)
	GetPortForward(containerID string, remotePort int) (*PortForward, error)

	// Configuration
	SetConfig(config *config.PortForwardConfig) error
}

// PortForward represents an active port forward
type PortForward struct {
	ID               string        `json:"id"`
	ContainerID      string        `json:"container_id"`
	ContainerName    string        `json:"container_name"`
	LocalPort        int           `json:"local_port"`
	RemotePort       int           `json:"remote_port"`
	Status           ForwardStatus `json:"status"`
	CreatedAt        time.Time     `json:"created_at"`
	LastUsed         time.Time     `json:"last_used"`
	BytesTransferred int64         `json:"bytes_transferred"`
}

// ForwardStatus represents the status of a port forward
type ForwardStatus string

const (
	ForwardStatusActive   ForwardStatus = "active"
	ForwardStatusInactive ForwardStatus = "inactive"
	ForwardStatusError    ForwardStatus = "error"
)

// portForwardManagerImpl implements PortForwardManager
type portForwardManagerImpl struct {
	config *config.PortForwardConfig
	logger logger.LoggerInterface

	// State management
	forwards   map[string]*PortForward           // forwardID -> PortForward
	containers map[string]*monitor.ContainerInfo // containerID -> ContainerInfo
	portMap    map[int]string                    // localPort -> forwardID

	// Synchronization
	mu      sync.RWMutex
	running bool
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewPortForwardManager creates a new port forward manager
func NewPortForwardManager(config *config.PortForwardConfig, logger logger.LoggerInterface) PortForwardManager {
	return &portForwardManagerImpl{
		config:     config,
		logger:     logger,
		forwards:   make(map[string]*PortForward),
		containers: make(map[string]*monitor.ContainerInfo),
		portMap:    make(map[int]string),
	}
}

// Start starts the port forward manager
func (pfm *portForwardManagerImpl) Start(ctx context.Context) error {
	pfm.mu.Lock()
	defer pfm.mu.Unlock()

	if pfm.running {
		return fmt.Errorf("port forward manager is already running")
	}

	if !pfm.config.Enabled {
		pfm.logger.Info("Port forwarding is disabled in configuration")
		return nil
	}

	pfm.ctx, pfm.cancel = context.WithCancel(ctx)
	pfm.running = true

	pfm.logger.WithFields(map[string]interface{}{
		"conflict_strategy": pfm.config.ConflictStrategy,
		"monitor_interval":  pfm.config.MonitorInterval,
	}).Info("Port forward manager started")

	return nil
}

// Stop stops the port forward manager
func (pfm *portForwardManagerImpl) Stop() error {
	pfm.mu.Lock()
	defer pfm.mu.Unlock()

	if !pfm.running {
		return nil
	}

	pfm.cancel()
	pfm.running = false

	// Clean up all active forwards
	for _, forward := range pfm.forwards {
		if forward.Status == ForwardStatusActive {
			pfm.logger.WithFields(map[string]interface{}{
				"forward_id":   forward.ID,
				"container_id": forward.ContainerID,
				"local_port":   forward.LocalPort,
				"remote_port":  forward.RemotePort,
			}).Debug("Cleaning up port forward on shutdown")
		}
	}

	// Clear state
	pfm.forwards = make(map[string]*PortForward)
	pfm.containers = make(map[string]*monitor.ContainerInfo)
	pfm.portMap = make(map[int]string)

	pfm.logger.Info("Port forward manager stopped")
	return nil
}

// OnContainerCreated handles container creation events
func (pfm *portForwardManagerImpl) OnContainerCreated(container *monitor.ContainerInfo) error {
	pfm.mu.Lock()
	defer pfm.mu.Unlock()

	if !pfm.running || !pfm.config.Enabled {
		return nil
	}

	pfm.logger.WithFields(map[string]interface{}{
		"container_id":   container.ID,
		"container_name": container.Name,
		"ports":          len(container.Ports),
	}).Debug("Container created event received")

	// Store container info
	pfm.containers[container.ID] = container

	// Create port forwards for exposed ports
	for _, portMapping := range container.Ports {
		if err := pfm.createPortForward(container, portMapping); err != nil {
			pfm.logger.WithFields(map[string]interface{}{
				"container_id": container.ID,
				"port":         portMapping.ContainerPort,
				"error":        err.Error(),
			}).Error("Failed to create port forward")
		}
	}

	return nil
}

// OnContainerStopped handles container stopped events
func (pfm *portForwardManagerImpl) OnContainerStopped(containerID string) error {
	pfm.mu.Lock()
	defer pfm.mu.Unlock()

	if !pfm.running {
		return nil
	}

	pfm.logger.WithFields(map[string]interface{}{
		"container_id": containerID,
	}).Debug("Container stopped event received")

	return pfm.cleanupContainerForwards(containerID)
}

// OnContainerRemoved handles container removed events
func (pfm *portForwardManagerImpl) OnContainerRemoved(containerID string) error {
	pfm.mu.Lock()
	defer pfm.mu.Unlock()

	if !pfm.running {
		return nil
	}

	pfm.logger.WithFields(map[string]interface{}{
		"container_id": containerID,
	}).Debug("Container removed event received")

	// Clean up forwards and remove container info
	if err := pfm.cleanupContainerForwards(containerID); err != nil {
		return err
	}

	delete(pfm.containers, containerID)
	return nil
}

// AddPortForward manually adds a port forward
func (pfm *portForwardManagerImpl) AddPortForward(containerID string, localPort, remotePort int) error {
	pfm.mu.Lock()
	defer pfm.mu.Unlock()

	if !pfm.running {
		return fmt.Errorf("port forward manager is not running")
	}

	container, exists := pfm.containers[containerID]
	if !exists {
		return fmt.Errorf("container %s not found", containerID)
	}

	portMapping := monitor.PortMapping{
		ContainerPort: remotePort,
		HostPort:      localPort,
		Protocol:      "tcp",
		HostIP:        "0.0.0.0",
	}

	return pfm.createPortForward(container, portMapping)
}

// RemovePortForward manually removes a port forward
func (pfm *portForwardManagerImpl) RemovePortForward(containerID string, localPort int) error {
	pfm.mu.Lock()
	defer pfm.mu.Unlock()

	if !pfm.running {
		return fmt.Errorf("port forward manager is not running")
	}

	forwardID, exists := pfm.portMap[localPort]
	if !exists {
		return fmt.Errorf("no port forward found for local port %d", localPort)
	}

	forward, exists := pfm.forwards[forwardID]
	if !exists || forward.ContainerID != containerID {
		return fmt.Errorf("port forward mismatch for container %s and port %d", containerID, localPort)
	}

	return pfm.removePortForward(forwardID)
}

// ListPortForwards returns all active port forwards
func (pfm *portForwardManagerImpl) ListPortForwards() ([]*PortForward, error) {
	pfm.mu.RLock()
	defer pfm.mu.RUnlock()

	forwards := make([]*PortForward, 0, len(pfm.forwards))
	for _, forward := range pfm.forwards {
		forwards = append(forwards, forward)
	}

	return forwards, nil
}

// GetPortForward gets a specific port forward
func (pfm *portForwardManagerImpl) GetPortForward(containerID string, remotePort int) (*PortForward, error) {
	pfm.mu.RLock()
	defer pfm.mu.RUnlock()

	for _, forward := range pfm.forwards {
		if forward.ContainerID == containerID && forward.RemotePort == remotePort {
			return forward, nil
		}
	}

	return nil, fmt.Errorf("port forward not found for container %s port %d", containerID, remotePort)
}

// SetConfig updates the configuration
func (pfm *portForwardManagerImpl) SetConfig(config *config.PortForwardConfig) error {
	pfm.mu.Lock()
	defer pfm.mu.Unlock()

	pfm.config = config

	pfm.logger.WithFields(map[string]interface{}{
		"enabled":           config.Enabled,
		"conflict_strategy": config.ConflictStrategy,
		"monitor_interval":  config.MonitorInterval,
	}).Info("Port forward configuration updated")

	return nil
}

// createPortForward creates a new port forward (must be called with lock held)
func (pfm *portForwardManagerImpl) createPortForward(container *monitor.ContainerInfo, portMapping monitor.PortMapping) error {
	// Use first 12 characters of container ID, or full ID if shorter
	containerIDPrefix := container.ID
	if len(containerIDPrefix) > 12 {
		containerIDPrefix = containerIDPrefix[:12]
	}
	forwardID := fmt.Sprintf("%s-%d", containerIDPrefix, portMapping.ContainerPort)

	// Check if forward already exists
	if _, exists := pfm.forwards[forwardID]; exists {
		pfm.logger.WithFields(map[string]interface{}{
			"forward_id":   forwardID,
			"container_id": container.ID,
			"port":         portMapping.ContainerPort,
		}).Debug("Port forward already exists")
		return nil
	}

	// For now, just create the forward entry without actual proxy server
	// The proxy server will be implemented in subsequent tasks
	forward := &PortForward{
		ID:            forwardID,
		ContainerID:   container.ID,
		ContainerName: container.Name,
		LocalPort:     portMapping.HostPort,
		RemotePort:    portMapping.ContainerPort,
		Status:        ForwardStatusActive,
		CreatedAt:     time.Now(),
		LastUsed:      time.Now(),
	}

	pfm.forwards[forwardID] = forward
	pfm.portMap[forward.LocalPort] = forwardID

	pfm.logger.WithFields(map[string]interface{}{
		"forward_id":     forwardID,
		"container_id":   container.ID,
		"container_name": container.Name,
		"local_port":     forward.LocalPort,
		"remote_port":    forward.RemotePort,
	}).Info("Port forward created")

	return nil
}

// removePortForward removes a port forward (must be called with lock held)
func (pfm *portForwardManagerImpl) removePortForward(forwardID string) error {
	forward, exists := pfm.forwards[forwardID]
	if !exists {
		return fmt.Errorf("port forward %s not found", forwardID)
	}

	// Remove from maps
	delete(pfm.forwards, forwardID)
	delete(pfm.portMap, forward.LocalPort)

	pfm.logger.WithFields(map[string]interface{}{
		"forward_id":   forwardID,
		"container_id": forward.ContainerID,
		"local_port":   forward.LocalPort,
		"remote_port":  forward.RemotePort,
	}).Info("Port forward removed")

	return nil
}

// cleanupContainerForwards removes all forwards for a container (must be called with lock held)
func (pfm *portForwardManagerImpl) cleanupContainerForwards(containerID string) error {
	var forwardsToRemove []string

	// Find all forwards for this container
	for forwardID, forward := range pfm.forwards {
		if forward.ContainerID == containerID {
			forwardsToRemove = append(forwardsToRemove, forwardID)
		}
	}

	// Remove each forward
	for _, forwardID := range forwardsToRemove {
		if err := pfm.removePortForward(forwardID); err != nil {
			pfm.logger.WithFields(map[string]interface{}{
				"forward_id":   forwardID,
				"container_id": containerID,
				"error":        err.Error(),
			}).Error("Failed to remove port forward during cleanup")
		}
	}

	return nil
}
