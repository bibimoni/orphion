//go:build !windows

package cli

import (
	"os"
	"path/filepath"
)

// DefaultConfigPath returns the default configuration file path on Unix systems.
func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "orphion", "config.yaml")
}
