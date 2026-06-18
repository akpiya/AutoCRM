package main

import (
	"os"
	"path/filepath"
)

const launchAgentLabel = "com.user.autocrm"

func installedBinaryPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "bin", "autocrm"), nil
}
