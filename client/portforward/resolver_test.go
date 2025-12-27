package portforward

import (
	"fmt"
	"net"
	"testing"

	"github.com/dockbridge/dockbridge/shared/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPortConflictResolver_IsPortAvailable(t *testing.T) {
	resolver := NewPortConflictResolver()

	// Test with an available port (using 0 to get any available port)
	listener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	availablePort := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	// Port should be available after closing
	assert.True(t, resolver.IsPortAvailable(availablePort))

	// Test with a port that's in use
	listener, err = net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	defer listener.Close()
	usedPort := listener.Addr().(*net.TCPAddr).Port

	// Port should not be available while listener is open
	assert.False(t, resolver.IsPortAvailable(usedPort))

	// Test invalid ports
	assert.False(t, resolver.IsPortAvailable(0))
	assert.False(t, resolver.IsPortAvailable(-1))
	assert.False(t, resolver.IsPortAvailable(65536))
	assert.False(t, resolver.IsPortAvailable(70000))
}

func TestPortConflictResolver_GetNextAvailablePort(t *testing.T) {
	resolver := NewPortConflictResolver()

	// Test with a high port number that should be available
	port, err := resolver.GetNextAvailablePort(50000)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, port, 50000)
	assert.LessOrEqual(t, port, 65535)

	// Verify the returned port is actually available
	assert.True(t, resolver.IsPortAvailable(port))

	// Test with invalid start port
	_, err = resolver.GetNextAvailablePort(0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid start port")

	_, err = resolver.GetNextAvailablePort(-1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid start port")

	_, err = resolver.GetNextAvailablePort(70000)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid start port")
}

func TestPortConflictResolver_ResolvePortConflict_AvailablePort(t *testing.T) {
	resolver := NewPortConflictResolver()

	// Test with an available port - should return the same port
	listener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	availablePort := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	// Test increment strategy
	resolvedPort, err := resolver.ResolvePortConflict(availablePort, config.ConflictStrategyIncrement)
	require.NoError(t, err)
	assert.Equal(t, availablePort, resolvedPort)

	// Test fail strategy
	resolvedPort, err = resolver.ResolvePortConflict(availablePort, config.ConflictStrategyFail)
	require.NoError(t, err)
	assert.Equal(t, availablePort, resolvedPort)
}

func TestPortConflictResolver_ResolvePortConflict_IncrementStrategy(t *testing.T) {
	resolver := NewPortConflictResolver()

	// Create a listener to occupy a port
	listener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	defer listener.Close()
	occupiedPort := listener.Addr().(*net.TCPAddr).Port

	// Test increment strategy with occupied port
	resolvedPort, err := resolver.ResolvePortConflict(occupiedPort, config.ConflictStrategyIncrement)
	require.NoError(t, err)
	assert.NotEqual(t, occupiedPort, resolvedPort)
	assert.Greater(t, resolvedPort, occupiedPort)

	// Verify the resolved port is actually available
	assert.True(t, resolver.IsPortAvailable(resolvedPort))
}

func TestPortConflictResolver_ResolvePortConflict_FailStrategy(t *testing.T) {
	resolver := NewPortConflictResolver()

	// Create a listener to occupy a port
	listener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	defer listener.Close()
	occupiedPort := listener.Addr().(*net.TCPAddr).Port

	// Test fail strategy with occupied port
	_, err = resolver.ResolvePortConflict(occupiedPort, config.ConflictStrategyFail)
	require.Error(t, err)

	// Check that it's a Docker-compatible error
	dockerErr, ok := err.(*DockerAPIError)
	require.True(t, ok, "Error should be a DockerAPIError")
	assert.Equal(t, "port_already_allocated", dockerErr.Code)
	assert.Contains(t, dockerErr.Message, fmt.Sprintf("tcp 0.0.0.0:%d", occupiedPort))
	assert.Contains(t, dockerErr.Message, "bind: address already in use")
	assert.Contains(t, dockerErr.Message, "local machine")
}

func TestPortConflictResolver_ResolvePortConflict_UnknownStrategy(t *testing.T) {
	resolver := NewPortConflictResolver()

	// Create a listener to occupy a port, so we trigger the strategy logic
	listener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	defer listener.Close()
	occupiedPort := listener.Addr().(*net.TCPAddr).Port

	// Test with unknown strategy on occupied port
	_, err = resolver.ResolvePortConflict(occupiedPort, config.ConflictStrategy("unknown"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown conflict strategy")
}

func TestPortConflictResolver_IncrementStrategy_CommonPorts(t *testing.T) {
	resolver := NewPortConflictResolver()

	testCases := []struct {
		name          string
		requestedPort int
		expectedNext  []int
	}{
		{
			name:          "HTTP port 80",
			requestedPort: 80,
			expectedNext:  []int{8080, 8081, 8082, 8083, 8084},
		},
		{
			name:          "HTTPS port 443",
			requestedPort: 443,
			expectedNext:  []int{8443, 8444, 8445, 8446, 8447},
		},
		{
			name:          "Node.js port 3000",
			requestedPort: 3000,
			expectedNext:  []int{3001, 3002, 3003, 3004, 3005},
		},
		{
			name:          "PostgreSQL port 5432",
			requestedPort: 5432,
			expectedNext:  []int{5433, 5434, 5435, 5436, 5437},
		},
		{
			name:          "MySQL port 3306",
			requestedPort: 3306,
			expectedNext:  []int{3307, 3308, 3309, 3310, 3311},
		},
		{
			name:          "Redis port 6379",
			requestedPort: 6379,
			expectedNext:  []int{6380, 6381, 6382, 6383, 6384},
		},
		{
			name:          "MongoDB port 27017",
			requestedPort: 27017,
			expectedNext:  []int{27018, 27019, 27020, 27021, 27022},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create listeners to occupy the requested port and some expected alternatives
			var listeners []net.Listener

			// Occupy the requested port
			listener, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", tc.requestedPort))
			if err == nil {
				listeners = append(listeners, listener)
			}

			// Occupy some of the expected next ports to test the algorithm
			for i := 0; i < len(tc.expectedNext)-1; i++ {
				listener, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", tc.expectedNext[i]))
				if err == nil {
					listeners = append(listeners, listener)
				}
			}

			// Clean up listeners
			defer func() {
				for _, l := range listeners {
					l.Close()
				}
			}()

			// Test resolution
			resolvedPort, err := resolver.ResolvePortConflict(tc.requestedPort, config.ConflictStrategyIncrement)
			require.NoError(t, err)

			// The resolved port should be one of the expected alternatives or higher
			assert.NotEqual(t, tc.requestedPort, resolvedPort)
			assert.True(t, resolver.IsPortAvailable(resolvedPort))

			// For common ports, the resolved port should be in the expected range or follow the pattern
			switch tc.requestedPort {
			case 80:
				assert.GreaterOrEqual(t, resolvedPort, 8080)
			case 443:
				assert.GreaterOrEqual(t, resolvedPort, 8443)
			default:
				assert.Greater(t, resolvedPort, tc.requestedPort)
			}
		})
	}
}

func TestPortConflictResolver_IncrementStrategy_CustomPorts(t *testing.T) {
	resolver := NewPortConflictResolver()

	// Test with a custom port that doesn't have special handling
	customPort := 9999

	// Create a listener to occupy the custom port
	listener, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", customPort))
	require.NoError(t, err)
	defer listener.Close()

	// Test resolution
	resolvedPort, err := resolver.ResolvePortConflict(customPort, config.ConflictStrategyIncrement)
	require.NoError(t, err)

	// Should get the next available port (likely 10000)
	assert.Greater(t, resolvedPort, customPort)
	assert.True(t, resolver.IsPortAvailable(resolvedPort))
}

func TestDockerAPIError_Error(t *testing.T) {
	err := &DockerAPIError{
		Message: "Test error message",
		Code:    "test_code",
	}

	assert.Equal(t, "Test error message", err.Error())
}

func TestPortConflictResolver_EdgeCases(t *testing.T) {
	resolver := NewPortConflictResolver()

	// Test with port near the upper limit
	highPort := 65530
	resolvedPort, err := resolver.ResolvePortConflict(highPort, config.ConflictStrategyIncrement)
	require.NoError(t, err)
	assert.LessOrEqual(t, resolvedPort, 65535)

	// Test with a higher port that should be available (skip port 1 as it may require privileges)
	testPort := 40000
	resolvedPort, err = resolver.ResolvePortConflict(testPort, config.ConflictStrategyIncrement)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, resolvedPort, testPort)
	assert.LessOrEqual(t, resolvedPort, 65535)
}

func TestPortConflictResolver_ConcurrentAccess(t *testing.T) {
	resolver := NewPortConflictResolver()

	// Test that multiple goroutines can use the resolver concurrently
	// This is a basic test - in practice, port availability can change between check and use
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			defer func() { done <- true }()

			// Each goroutine tries to resolve a different port
			basePort := 40000 + id*10
			resolvedPort, err := resolver.ResolvePortConflict(basePort, config.ConflictStrategyIncrement)
			assert.NoError(t, err)
			assert.GreaterOrEqual(t, resolvedPort, basePort)
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}

// Ensure PortConflictResolver interface is implemented
var _ PortConflictResolver = (*portConflictResolverImpl)(nil)
