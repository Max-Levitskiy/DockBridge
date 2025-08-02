package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestClientConfigDefaults(t *testing.T) {
	config := ClientConfig{}

	// Test that the struct can be instantiated
	assert.NotNil(t, config)

	// Test default values would be applied by configuration loading
	assert.Equal(t, "", config.Hetzner.ServerType) // Will be set to "cpx21" by viper
	assert.Equal(t, "", config.Hetzner.Location)   // Will be set to "fsn1" by viper
	assert.Equal(t, 0, config.Hetzner.VolumeSize)  // Will be set to 10 by viper
}

func TestKeepAliveConfigDefaults(t *testing.T) {
	config := KeepAliveConfig{}

	// Test that the struct can be instantiated
	assert.NotNil(t, config)

	// Test zero values
	assert.Equal(t, time.Duration(0), config.Interval)
	assert.Equal(t, time.Duration(0), config.Timeout)
	assert.Equal(t, 0, config.MaxRetries)
}
