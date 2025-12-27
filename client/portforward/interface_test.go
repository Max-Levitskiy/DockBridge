package portforward

import (
	"testing"

	"github.com/dockbridge/dockbridge/client/monitor"
	"github.com/dockbridge/dockbridge/pkg/logger"
	"github.com/dockbridge/dockbridge/shared/config"
)

// TestPortForwardManagerImplementsContainerEventHandler verifies that PortForwardManager
// properly implements the monitor.ContainerEventHandler interface
func TestPortForwardManagerImplementsContainerEventHandler(t *testing.T) {
	// Create a port forward manager
	config := &config.PortForwardConfig{
		Enabled:          true,
		ConflictStrategy: config.ConflictStrategyIncrement,
		MonitorInterval:  30,
	}

	logger := logger.NewDefault()
	pfm := NewPortForwardManager(config, logger)

	// Verify it implements ContainerEventHandler interface
	var handler monitor.ContainerEventHandler = pfm

	// This should compile without errors if the interface is properly implemented
	if handler == nil {
		t.Fatal("PortForwardManager should implement ContainerEventHandler interface")
	}

	t.Log("PortForwardManager successfully implements ContainerEventHandler interface")
}

// TestInterfaceMethodSignatures tests that the method signatures match exactly
func TestInterfaceMethodSignatures(t *testing.T) {
	config := &config.PortForwardConfig{
		Enabled:          true,
		ConflictStrategy: config.ConflictStrategyIncrement,
		MonitorInterval:  30,
	}

	logger := logger.NewDefault()
	pfm := NewPortForwardManager(config, logger)

	// Create a mock container info
	containerInfo := &monitor.ContainerInfo{
		ID:   "test-container",
		Name: "test",
		Ports: []monitor.PortMapping{
			{
				ContainerPort: 80,
				HostPort:      8080,
				Protocol:      "tcp",
				HostIP:        "0.0.0.0",
			},
		},
	}

	// Test that we can call the interface methods without compilation errors
	err := pfm.OnContainerCreated(containerInfo)
	if err != nil {
		t.Logf("OnContainerCreated returned error (expected for unstarted manager): %v", err)
	}

	err = pfm.OnContainerStopped("test-container")
	if err != nil {
		t.Logf("OnContainerStopped returned error (expected for unstarted manager): %v", err)
	}

	err = pfm.OnContainerRemoved("test-container")
	if err != nil {
		t.Logf("OnContainerRemoved returned error (expected for unstarted manager): %v", err)
	}

	t.Log("All interface methods have correct signatures")
}
