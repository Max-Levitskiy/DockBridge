package config

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// LoadWithoutValidation loads configuration from file without validation
func (m *Manager) LoadWithoutValidation(configPath string) error {
	// Set up Viper configuration
	m.setupViper(configPath)

	// Set defaults
	m.setDefaults()

	// Read configuration
	if err := m.viper.ReadInConfig(); err != nil {
		// Check if it's a config file not found error
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found is OK, we'll use defaults and env vars
		} else {
			// Check if it's a path error (file doesn't exist)
			if os.IsNotExist(err) {
				// File doesn't exist is OK, we'll use defaults and env vars
			} else {
				return fmt.Errorf("failed to read config file: %w", err)
			}
		}
	}

	// Unmarshal into struct
	if err := m.viper.Unmarshal(m.config); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return nil
}

// SaveConfig saves the configuration to a file
func (m *Manager) SaveConfig(configPath string) error {
	// Convert config to YAML
	yamlData, err := yaml.Marshal(m.config)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration to YAML: %w", err)
	}

	// Write to file
	if err := os.WriteFile(configPath, yamlData, 0644); err != nil {
		return fmt.Errorf("failed to write configuration to %s: %w", configPath, err)
	}

	return nil
}
