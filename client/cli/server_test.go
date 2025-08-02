package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServerCommand(t *testing.T) {
	// Check that the command has the expected properties
	assert.Equal(t, "server", serverCmd.Name())
	assert.Equal(t, "Manage Hetzner Cloud servers", serverCmd.Short)

	// Check that the command has the expected subcommands
	var hasCreate, hasDestroy, hasStatus bool
	for _, cmd := range serverCmd.Commands() {
		switch cmd.Name() {
		case "create":
			hasCreate = true
		case "destroy":
			hasDestroy = true
		case "status":
			hasStatus = true
		}
	}

	assert.True(t, hasCreate, "server command should have 'create' subcommand")
	assert.True(t, hasDestroy, "server command should have 'destroy' subcommand")
	assert.True(t, hasStatus, "server command should have 'status' subcommand")
}

func TestServerCreateCommand(t *testing.T) {
	// Check that the command has the expected properties
	assert.Equal(t, "create", serverCreateCmd.Name())
	assert.Equal(t, "Initialize and validate server configuration", serverCreateCmd.Short)

	// Check that the command has the expected flags
	assert.NotNil(t, serverCreateCmd.Flags().Lookup("config"))
}

func TestServerDestroyCommand(t *testing.T) {
	// Check that the command has the expected properties
	assert.Equal(t, "destroy", serverDestroyCmd.Name())
	assert.Equal(t, "Destroy DockBridge servers", serverDestroyCmd.Short)

	// Check that the command has the expected flags
	assert.NotNil(t, serverDestroyCmd.Flags().Lookup("config"))
	assert.NotNil(t, serverDestroyCmd.Flags().Lookup("force"))
}

func TestServerStatusCommand(t *testing.T) {
	// Check that the command has the expected properties
	assert.Equal(t, "status", serverStatusCmd.Name())
	assert.Equal(t, "Check DockBridge server status", serverStatusCmd.Short)

	// Check that the command has the expected flags
	assert.NotNil(t, serverStatusCmd.Flags().Lookup("config"))
}
