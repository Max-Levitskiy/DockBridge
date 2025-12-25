package portforward

import (
	"fmt"
	"net"

	"github.com/dockbridge/dockbridge/shared/config"
)

// PortConflictResolver defines the interface for resolving port conflicts
type PortConflictResolver interface {
	ResolvePortConflict(requestedPort int, strategy config.ConflictStrategy) (int, error)
	IsPortAvailable(port int) bool
	GetNextAvailablePort(startPort int) (int, error)
}

// DockerAPIError represents a Docker-compatible error response
type DockerAPIError struct {
	Message string `json:"message"`
	Code    string `json:"code"`
}

// Error implements the error interface for DockerAPIError
func (e *DockerAPIError) Error() string {
	return e.Message
}

// portConflictResolverImpl implements PortConflictResolver
type portConflictResolverImpl struct {
	// Configuration for port resolution
	maxPortScanRange int
}

// NewPortConflictResolver creates a new port conflict resolver
func NewPortConflictResolver() PortConflictResolver {
	return &portConflictResolverImpl{
		maxPortScanRange: 1000, // Scan up to 1000 ports when looking for available port
	}
}

// ResolvePortConflict resolves a port conflict using the specified strategy
func (pcr *portConflictResolverImpl) ResolvePortConflict(requestedPort int, strategy config.ConflictStrategy) (int, error) {
	// First check if the requested port is available
	if pcr.IsPortAvailable(requestedPort) {
		return requestedPort, nil
	}

	// Port is not available, apply strategy
	switch strategy {
	case config.ConflictStrategyIncrement:
		return pcr.resolveWithIncrementStrategy(requestedPort)
	case config.ConflictStrategyFail:
		return 0, pcr.createDockerCompatibleError(requestedPort)
	default:
		return 0, fmt.Errorf("unknown conflict strategy: %s", strategy)
	}
}

// IsPortAvailable checks if a port is available for binding
func (pcr *portConflictResolverImpl) IsPortAvailable(port int) bool {
	// Validate port range
	if port < 1 || port > 65535 {
		return false
	}

	// Try to bind to the port
	listener, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		return false
	}

	// Port is available, close the listener
	listener.Close()
	return true
}

// GetNextAvailablePort finds the next available port starting from startPort
func (pcr *portConflictResolverImpl) GetNextAvailablePort(startPort int) (int, error) {
	// Validate starting port
	if startPort < 1 || startPort > 65535 {
		return 0, fmt.Errorf("invalid start port: %d", startPort)
	}

	// Scan for available port
	for port := startPort; port <= startPort+pcr.maxPortScanRange && port <= 65535; port++ {
		if pcr.IsPortAvailable(port) {
			return port, nil
		}
	}

	return 0, fmt.Errorf("no available port found in range %d-%d", startPort, startPort+pcr.maxPortScanRange)
}

// resolveWithIncrementStrategy implements the increment strategy for port conflict resolution
func (pcr *portConflictResolverImpl) resolveWithIncrementStrategy(requestedPort int) (int, error) {
	// Strategy: try same port first, then increment
	// For common ports like 80, try 8080 next, then 8081, etc.
	// For other ports, just increment: 3000 -> 3001 -> 3002, etc.

	var candidatePorts []int

	// Special handling for common web ports
	switch requestedPort {
	case 80:
		candidatePorts = []int{8080, 8081, 8082, 8083, 8084}
	case 443:
		candidatePorts = []int{8443, 8444, 8445, 8446, 8447}
	case 3000:
		candidatePorts = []int{3001, 3002, 3003, 3004, 3005}
	case 5432: // PostgreSQL
		candidatePorts = []int{5433, 5434, 5435, 5436, 5437}
	case 3306: // MySQL
		candidatePorts = []int{3307, 3308, 3309, 3310, 3311}
	case 6379: // Redis
		candidatePorts = []int{6380, 6381, 6382, 6383, 6384}
	case 27017: // MongoDB
		candidatePorts = []int{27018, 27019, 27020, 27021, 27022}
	default:
		// For other ports, just increment sequentially
		for i := 1; i <= 10; i++ {
			candidatePorts = append(candidatePorts, requestedPort+i)
		}
	}

	// Try each candidate port
	for _, port := range candidatePorts {
		if port > 65535 {
			break // Skip invalid ports
		}
		if pcr.IsPortAvailable(port) {
			return port, nil
		}
	}

	// If none of the candidate ports work, use the general algorithm
	return pcr.GetNextAvailablePort(requestedPort + 1)
}

// createDockerCompatibleError creates a Docker-compatible error for port conflicts
func (pcr *portConflictResolverImpl) createDockerCompatibleError(port int) error {
	return &DockerAPIError{
		Message: fmt.Sprintf("driver failed programming external connectivity on endpoint: Error starting userland proxy: listen tcp 0.0.0.0:%d: bind: address already in use (local machine)", port),
		Code:    "port_already_allocated",
	}
}

// Ensure portConflictResolverImpl implements PortConflictResolver
var _ PortConflictResolver = (*portConflictResolverImpl)(nil)
