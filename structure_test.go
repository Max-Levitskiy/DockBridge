package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestProjectStructure validates that the new Go-standard project structure exists
func TestProjectStructure(t *testing.T) {
	// Define expected directories according to Go best practices
	expectedDirs := []string{
		"cmd/dockbridge",
		"client/config",
		"client/hetzner",
		"client/lifecycle",
		"client/proxy",
		"client/ssh",
		"client/cli",
		"client/docker",
		"server/config",
		"server/cli",
		"server/docker",
		"shared/config",
		"shared/logging",
		"pkg",
		"configs",
	}

	for _, dir := range expectedDirs {
		t.Run("Directory_"+dir, func(t *testing.T) {
			_, err := os.Stat(dir)
			assert.NoError(t, err, "Directory %s should exist", dir)
		})
	}
}

// TestMainEntryPoint validates that the unified main entry point exists
func TestMainEntryPoint(t *testing.T) {
	mainFile := "cmd/dockbridge/main.go"
	_, err := os.Stat(mainFile)
	assert.NoError(t, err, "Main entry point %s should exist", mainFile)
}

// TestPackageFiles validates that key package files exist in the new structure
func TestPackageFiles(t *testing.T) {
	expectedFiles := []string{
		"client/config/config.go",
		"client/hetzner/client.go",
		"client/lifecycle/manager.go",
		"client/proxy/manager.go",
		"client/ssh/client.go",
		"client/cli/root.go",
		"client/docker/daemon.go",
		"server/config/config.go",
		"server/cli/root.go",
		"shared/config/types.go",
		"shared/logging/logger.go",
	}

	for _, file := range expectedFiles {
		t.Run("File_"+filepath.Base(file), func(t *testing.T) {
			_, err := os.Stat(file)
			assert.NoError(t, err, "File %s should exist", file)
		})
	}
}

// TestNoInternalImports validates that no files in the new structure import from internal/ paths
func TestNoInternalImports(t *testing.T) {
	// Directories to check for internal imports
	checkDirs := []string{
		"client",
		"server",
		"shared",
		"cmd/dockbridge",
	}

	for _, dir := range checkDirs {
		t.Run("Directory_"+dir, func(t *testing.T) {
			err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}

				// Only check Go files
				if !strings.HasSuffix(path, ".go") {
					return nil
				}

				// Skip test files for this check
				if strings.HasSuffix(path, "_test.go") {
					return nil
				}

				// Read file content and check for internal imports
				content, err := os.ReadFile(path)
				if err != nil {
					t.Errorf("Failed to read %s: %v", path, err)
					return nil
				}

				// Check for internal import patterns
				contentStr := string(content)
				if strings.Contains(contentStr, "github.com/dockbridge/dockbridge/internal") {
					t.Errorf("File %s contains internal import", path)
				}

				return nil
			})
			assert.NoError(t, err)
		})
	}
}
