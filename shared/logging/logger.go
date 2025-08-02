package config

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/dockbridge/dockbridge/pkg/logger"
	"gopkg.in/yaml.v3"
)

// LoadLoggerConfig loads the logger configuration from the specified file
func LoadLoggerConfig(configPath string) (*logger.Config, error) {
	// Default configuration
	cfg := &logger.Config{
		Level:      "info",
		UseColors:  true,
		TimeFormat: "2006-01-02T15:04:05.000Z07:00",
	}

	// If no config path is provided, return default config
	if configPath == "" {
		return cfg, nil
	}

	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return cfg, nil
	}

	// Read config file
	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	// Parse YAML
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// InitLogger initializes the global logger with the specified configuration
func InitLogger(configPath string) error {
	cfg, err := LoadLoggerConfig(configPath)
	if err != nil {
		return err
	}

	// Create logger
	log, err := logger.New(cfg)
	if err != nil {
		return err
	}

	// Set as default logger
	logger.SetDefaultLogger(log)
	return nil
}

// GetDefaultLoggerConfigPath returns the default path for the logger configuration
func GetDefaultLoggerConfigPath() string {
	// Check for config in current directory
	if _, err := os.Stat("configs/logger.yaml"); err == nil {
		return "configs/logger.yaml"
	}

	// Check for config in user's home directory
	home, err := os.UserHomeDir()
	if err == nil {
		homePath := filepath.Join(home, ".dockbridge", "configs", "logger.yaml")
		if _, err := os.Stat(homePath); err == nil {
			return homePath
		}
	}

	// Return default path even if it doesn't exist
	return "configs/logger.yaml"
}
