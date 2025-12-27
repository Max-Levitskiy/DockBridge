package monitor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/dockbridge/dockbridge/pkg/logger"
	"github.com/docker/docker/api/types/container"
	"github.com/pkg/errors"
)

// ContainerMonitor defines the interface for monitoring Docker containers
type ContainerMonitor interface {
	Start(ctx context.Context) error
	Stop() error

	// Event registration
	RegisterContainerEventHandler(handler ContainerEventHandler) error

	// Container queries
	ListRunningContainers(ctx context.Context) ([]*ContainerInfo, error)
	GetContainer(ctx context.Context, containerID string) (*ContainerInfo, error)

	// Monitoring configuration
	SetPollingInterval(interval time.Duration) error
}

// ContainerEventHandler defines the interface for handling container events
type ContainerEventHandler interface {
	OnContainerCreated(container *ContainerInfo) error
	OnContainerStopped(containerID string) error
	OnContainerRemoved(containerID string) error
}

// ContainerInfo represents container information for monitoring
type ContainerInfo struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Image   string            `json:"image"`
	Status  string            `json:"status"`
	Ports   []PortMapping     `json:"ports"`
	Labels  map[string]string `json:"labels"`
	Created time.Time         `json:"created"`
}

// PortMapping represents a container port mapping
type PortMapping struct {
	ContainerPort int    `json:"container_port"`
	HostPort      int    `json:"host_port"`
	Protocol      string `json:"protocol"`
	HostIP        string `json:"host_ip"`
}

// ContainerAPIClient defines the minimal Docker API interface needed for container monitoring
type ContainerAPIClient interface {
	ContainerList(ctx context.Context, options container.ListOptions) ([]container.Summary, error)
	ContainerInspect(ctx context.Context, containerID string) (container.InspectResponse, error)
}

// containerMonitorImpl implements ContainerMonitor
type containerMonitorImpl struct {
	dockerClient ContainerAPIClient
	logger       logger.LoggerInterface

	// Configuration
	pollingInterval time.Duration

	// Event handlers
	handlers []ContainerEventHandler
	mu       sync.RWMutex

	// State management
	running         bool
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	knownContainers map[string]*ContainerInfo // containerID -> ContainerInfo
}

// NewContainerMonitor creates a new container monitor
func NewContainerMonitor(dockerClient ContainerAPIClient, logger logger.LoggerInterface) ContainerMonitor {
	return &containerMonitorImpl{
		dockerClient:    dockerClient,
		logger:          logger,
		pollingInterval: 30 * time.Second, // Default polling interval
		handlers:        make([]ContainerEventHandler, 0),
		knownContainers: make(map[string]*ContainerInfo),
	}
}

// Start starts the container monitor
func (cm *containerMonitorImpl) Start(ctx context.Context) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.running {
		return fmt.Errorf("container monitor is already running")
	}

	cm.ctx, cm.cancel = context.WithCancel(ctx)
	cm.running = true

	// Initialize known containers state
	if err := cm.initializeKnownContainers(); err != nil {
		cm.logger.WithFields(map[string]any{
			"error": err.Error(),
		}).Warn("Failed to initialize known containers state")
	}

	// Start monitoring goroutine
	cm.wg.Add(1)
	go func() {
		defer cm.wg.Done()
		cm.monitorContainers()
	}()

	cm.logger.WithFields(map[string]any{
		"polling_interval": cm.pollingInterval,
		"handlers":         len(cm.handlers),
	}).Info("Container monitor started")

	return nil
}

// Stop stops the container monitor
func (cm *containerMonitorImpl) Stop() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if !cm.running {
		return nil
	}

	cm.cancel()
	cm.running = false

	// Wait for monitoring goroutine to finish
	cm.wg.Wait()

	// Clear state
	cm.knownContainers = make(map[string]*ContainerInfo)

	cm.logger.Info("Container monitor stopped")
	return nil
}

// RegisterContainerEventHandler registers a handler for container events
func (cm *containerMonitorImpl) RegisterContainerEventHandler(handler ContainerEventHandler) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.handlers = append(cm.handlers, handler)

	cm.logger.WithFields(map[string]any{
		"total_handlers": len(cm.handlers),
	}).Debug("Container event handler registered")

	return nil
}

// ListRunningContainers returns all currently running containers
func (cm *containerMonitorImpl) ListRunningContainers(ctx context.Context) ([]*ContainerInfo, error) {
	containers, err := cm.dockerClient.ContainerList(ctx, container.ListOptions{
		All: false, // Only running containers
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to list containers")
	}

	result := make([]*ContainerInfo, 0, len(containers))
	for _, c := range containers {
		containerInfo, err := cm.convertToContainerInfo(c)
		if err != nil {
			cm.logger.WithFields(map[string]any{
				"container_id": c.ID,
				"error":        err.Error(),
			}).Warn("Failed to convert container info")
			continue
		}
		result = append(result, containerInfo)
	}

	return result, nil
}

// GetContainer returns information about a specific container
func (cm *containerMonitorImpl) GetContainer(ctx context.Context, containerID string) (*ContainerInfo, error) {
	containerJSON, err := cm.dockerClient.ContainerInspect(ctx, containerID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to inspect container %s", containerID)
	}

	return cm.convertInspectToContainerInfo(containerJSON)
}

// SetPollingInterval sets the polling interval for container monitoring
func (cm *containerMonitorImpl) SetPollingInterval(interval time.Duration) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.pollingInterval = interval

	cm.logger.WithFields(map[string]any{
		"polling_interval": interval,
	}).Info("Container monitor polling interval updated")

	return nil
}

// initializeKnownContainers initializes the known containers state
func (cm *containerMonitorImpl) initializeKnownContainers() error {
	containers, err := cm.ListRunningContainers(cm.ctx)
	if err != nil {
		return errors.Wrap(err, "failed to list initial containers")
	}

	for _, container := range containers {
		cm.knownContainers[container.ID] = container
	}

	cm.logger.WithFields(map[string]any{
		"initial_containers": len(containers),
	}).Debug("Initialized known containers state")

	return nil
}

// monitorContainers runs the main monitoring loop
func (cm *containerMonitorImpl) monitorContainers() {
	ticker := time.NewTicker(cm.pollingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-cm.ctx.Done():
			return
		case <-ticker.C:
			if err := cm.checkContainerChanges(); err != nil {
				cm.logger.WithFields(map[string]any{
					"error": err.Error(),
				}).Error("Error checking container changes")
			}
		}
	}
}

// checkContainerChanges checks for container lifecycle changes
func (cm *containerMonitorImpl) checkContainerChanges() error {
	// Get current running containers
	currentContainers, err := cm.ListRunningContainers(cm.ctx)
	if err != nil {
		return errors.Wrap(err, "failed to list current containers")
	}

	// Create maps for efficient lookup
	currentMap := make(map[string]*ContainerInfo)
	for _, container := range currentContainers {
		currentMap[container.ID] = container
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Check for new containers (created)
	for containerID, container := range currentMap {
		if _, exists := cm.knownContainers[containerID]; !exists {
			cm.logger.WithFields(map[string]any{
				"container_id":   containerID,
				"container_name": container.Name,
				"image":          container.Image,
			}).Debug("New container detected")

			// Notify handlers about container creation
			for _, handler := range cm.handlers {
				if err := handler.OnContainerCreated(container); err != nil {
					cm.logger.WithFields(map[string]any{
						"container_id": containerID,
						"error":        err.Error(),
					}).Error("Handler failed to process container created event")
				}
			}

			cm.knownContainers[containerID] = container
		}
	}

	// Check for removed/stopped containers
	for containerID, container := range cm.knownContainers {
		if _, exists := currentMap[containerID]; !exists {
			cm.logger.WithFields(map[string]any{
				"container_id":   containerID,
				"container_name": container.Name,
			}).Debug("Container stopped/removed detected")

			// Try to determine if container was stopped or removed
			// by attempting to inspect it
			containerJSON, err := cm.dockerClient.ContainerInspect(cm.ctx, containerID)
			if err != nil {
				// Container not found - it was removed
				cm.logger.WithFields(map[string]any{
					"container_id": containerID,
				}).Debug("Container was removed")

				for _, handler := range cm.handlers {
					if err := handler.OnContainerRemoved(containerID); err != nil {
						cm.logger.WithFields(map[string]any{
							"container_id": containerID,
							"error":        err.Error(),
						}).Error("Handler failed to process container removed event")
					}
				}
			} else if !containerJSON.State.Running {
				// Container exists but is not running - it was stopped
				cm.logger.WithFields(map[string]any{
					"container_id": containerID,
					"status":       containerJSON.State.Status,
				}).Debug("Container was stopped")

				for _, handler := range cm.handlers {
					if err := handler.OnContainerStopped(containerID); err != nil {
						cm.logger.WithFields(map[string]any{
							"container_id": containerID,
							"error":        err.Error(),
						}).Error("Handler failed to process container stopped event")
					}
				}
			}

			delete(cm.knownContainers, containerID)
		}
	}

	return nil
}

// convertToContainerInfo converts Docker API container to ContainerInfo
func (cm *containerMonitorImpl) convertToContainerInfo(c container.Summary) (*ContainerInfo, error) {
	// Extract container name (remove leading slash)
	name := ""
	if len(c.Names) > 0 {
		name = c.Names[0]
		if len(name) > 0 && name[0] == '/' {
			name = name[1:]
		}
	}

	// Convert port mappings
	ports := make([]PortMapping, 0, len(c.Ports))
	for _, port := range c.Ports {
		ports = append(ports, PortMapping{
			ContainerPort: int(port.PrivatePort),
			HostPort:      int(port.PublicPort),
			Protocol:      port.Type,
			HostIP:        port.IP,
		})
	}

	return &ContainerInfo{
		ID:      c.ID,
		Name:    name,
		Image:   c.Image,
		Status:  c.Status,
		Ports:   ports,
		Labels:  c.Labels,
		Created: time.Unix(c.Created, 0),
	}, nil
}

// convertInspectToContainerInfo converts Docker inspect result to ContainerInfo
func (cm *containerMonitorImpl) convertInspectToContainerInfo(resp container.InspectResponse) (*ContainerInfo, error) {
	// Extract container name (remove leading slash)
	name := resp.Name
	if len(name) > 0 && name[0] == '/' {
		name = name[1:]
	}

	// Convert port mappings from NetworkSettings
	ports := make([]PortMapping, 0)
	if resp.NetworkSettings != nil && resp.NetworkSettings.Ports != nil {
		for portProto, bindings := range resp.NetworkSettings.Ports {
			// Parse port and protocol
			portStr := string(portProto)
			var containerPort int
			var protocol string
			if n, err := fmt.Sscanf(portStr, "%d/%s", &containerPort, &protocol); n == 2 && err == nil {
				// Add port mappings for each binding
				for _, binding := range bindings {
					hostPort := 0
					if binding.HostPort != "" {
						_, _ = fmt.Sscanf(binding.HostPort, "%d", &hostPort)
					}

					ports = append(ports, PortMapping{
						ContainerPort: containerPort,
						HostPort:      hostPort,
						Protocol:      protocol,
						HostIP:        binding.HostIP,
					})
				}
			}
		}
	}

	// Parse created time
	createdTime, err := time.Parse(time.RFC3339Nano, resp.Created)
	if err != nil {
		createdTime = time.Now() // Fallback to current time
	}

	return &ContainerInfo{
		ID:      resp.ID,
		Name:    name,
		Image:   resp.Config.Image,
		Status:  resp.State.Status,
		Ports:   ports,
		Labels:  resp.Config.Labels,
		Created: createdTime,
	}, nil
}
