//go:build windows

package main

import (
	"os"
	"path/filepath"
)

func defaultConfigPath() string {
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
