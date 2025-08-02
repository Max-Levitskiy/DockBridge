package docker

import (
	"os"
	"path/filepath"
	"strings"
)

// expandPath expands ~ to home directory
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return path // Return original path if we can't get home dir
		}
		return filepath.Join(homeDir, path[2:])
	}
	return path
}
