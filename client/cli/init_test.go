package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInitCommand(t *testing.T) {
	// Check that the command has the expected properties
	assert.Equal(t, "init", initCmd.Name())
	assert.Equal(t, "Initialize DockBridge client configuration", initCmd.Short)

	// Check that the command has the expected flags
	assert.NotNil(t, initCmd.Flags().Lookup("force"))
	assert.NotNil(t, initCmd.Flags().Lookup("guided"))
}

// Since we can't mock functions directly in Go, we'll test the command structure
func TestInitCommandFlags(t *testing.T) {
	// Check force flag
	forceFlag := initCmd.Flags().Lookup("force")
	assert.NotNil(t, forceFlag)
	assert.Equal(t, "bool", forceFlag.Value.Type())
	assert.Equal(t, "f", forceFlag.Shorthand)

	// Check guided flag
	guidedFlag := initCmd.Flags().Lookup("guided")
	assert.NotNil(t, guidedFlag)
	assert.Equal(t, "bool", guidedFlag.Value.Type())
	assert.Equal(t, "g", guidedFlag.Shorthand)
}
