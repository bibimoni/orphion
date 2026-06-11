package cli

import (
	"os"
	"path/filepath"
)

// DefaultConfigPath returns the default configuration file path on Windows.
func DefaultConfigPath() string {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		appData = filepath.Join(home, "AppData", "Roaming")
	}
	return filepath.Join(appData, "orphion", "config.yaml")
}
