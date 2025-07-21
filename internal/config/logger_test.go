package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadLoggerConfig(t *testing.T) {
	// Test with non-existent file (should return default config)
	cfg, err := LoadLoggerConfig("non_existent_file.yaml")
	assert.NoError(t, err)
	assert.Equal(t, "info", cfg.Level)
	assert.Equal(t, true, cfg.UseColors)
	assert.Equal(t, "2006-01-02T15:04:05.000Z07:00", cfg.TimeFormat)

	// Test with empty path (should return default config)
	cfg, err = LoadLoggerConfig("")
	assert.NoError(t, err)
	assert.Equal(t, "info", cfg.Level)

	// Create a temporary config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "logger.yaml")
	configContent := []byte(`
level: "debug"
use_colors: false
time_format: "2006/01/02 15:04:05"
`)
	err = os.WriteFile(configPath, configContent, 0644)
	assert.NoError(t, err)

	// Test with valid config file
	cfg, err = LoadLoggerConfig(configPath)
	assert.NoError(t, err)
	assert.Equal(t, "debug", cfg.Level)
	assert.Equal(t, false, cfg.UseColors)
	assert.Equal(t, "2006/01/02 15:04:05", cfg.TimeFormat)

	// Test with invalid YAML
	invalidConfigPath := filepath.Join(tempDir, "invalid_logger.yaml")
	invalidContent := []byte(`
level: "debug
use_colors: false
`)
	err = os.WriteFile(invalidConfigPath, invalidContent, 0644)
	assert.NoError(t, err)

	_, err = LoadLoggerConfig(invalidConfigPath)
	assert.Error(t, err)
}

func TestInitLogger(t *testing.T) {
	// Create a temporary config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "logger.yaml")
	configContent := []byte(`
level: "debug"
use_colors: false
time_format: "2006/01/02 15:04:05"
`)
	err := os.WriteFile(configPath, configContent, 0644)
	assert.NoError(t, err)

	// Test initialization with valid config
	err = InitLogger(configPath)
	assert.NoError(t, err)

	// Test initialization with default config
	err = InitLogger("")
	assert.NoError(t, err)

	// Test initialization with invalid config
	invalidConfigPath := filepath.Join(tempDir, "invalid_logger.yaml")
	invalidContent := []byte(`
level: "invalid_level"
use_colors: false
`)
	err = os.WriteFile(invalidConfigPath, invalidContent, 0644)
	assert.NoError(t, err)

	err = InitLogger(invalidConfigPath)
	assert.Error(t, err)
}

func TestGetDefaultLoggerConfigPath(t *testing.T) {
	// The default path should be returned
	path := GetDefaultLoggerConfigPath()
	assert.NotEmpty(t, path)
}
