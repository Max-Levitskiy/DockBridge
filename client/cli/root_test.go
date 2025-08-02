package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRootCommand(t *testing.T) {
	cmd := rootCmd.CommandPath()

	// Check that the command name is correct
	assert.Equal(t, "dockbridge", cmd)

	// Check that the command has the expected flags
	assert.NotNil(t, rootCmd.PersistentFlags().Lookup("config"))
	assert.NotNil(t, rootCmd.PersistentFlags().Lookup("verbose"))
}

func TestRootCommandVersion(t *testing.T) {
	// Check that the version is set correctly
	assert.Equal(t, "0.1.0", rootCmd.Version)
}

func TestRootCommandVerbose(t *testing.T) {
	// Reset verbose flag
	verbose = false

	// Execute the command with verbose flag
	rootCmd.SetArgs([]string{"--verbose"})
	err := rootCmd.Execute()

	// Check that there was no error
	assert.NoError(t, err)

	// Check that verbose flag was set
	assert.True(t, verbose)
}
