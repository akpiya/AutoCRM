package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func runUninstall() int {
	plistPath, err := launchAgentPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to resolve LaunchAgent path: %v\n", err)
		return 1
	}

	if err := unloadLaunchAgent(plistPath); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not unload LaunchAgent: %v\n", err)
	}
	if err := removeIfExists(plistPath); err != nil {
		fmt.Fprintf(os.Stderr, "Unable to remove LaunchAgent: %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stdout, "Removed LaunchAgent: %s\n", plistPath)

	binaryPath, err := installedBinaryPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to resolve installed binary path: %v\n", err)
		return 1
	}
	if err := removeIfExists(binaryPath); err != nil {
		fmt.Fprintf(os.Stderr, "Unable to remove installed binary: %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stdout, "Removed installed binary: %s\n", binaryPath)

	dataDir, err := autocrmDataDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to resolve AutoCRM data directory: %v\n", err)
		return 1
	}
	if err := removeAllIfExists(dataDir); err != nil {
		fmt.Fprintf(os.Stderr, "Unable to remove AutoCRM data directory: %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stdout, "Removed AutoCRM data directory: %s\n", dataDir)

	for _, logPath := range []string{"/tmp/autocrm.log", "/tmp/autocrm.err"} {
		if err := removeIfExists(logPath); err != nil {
			fmt.Fprintf(os.Stderr, "Unable to remove log file %s: %v\n", logPath, err)
			return 1
		}
		fmt.Fprintf(os.Stdout, "Removed log file: %s\n", logPath)
	}

	fmt.Fprintln(os.Stdout, "Removed AutoCRM.")
	return 0
}

func unloadLaunchAgent(plistPath string) error {
	target := fmt.Sprintf("gui/%d", os.Getuid())
	return exec.Command("launchctl", "bootout", target, plistPath).Run()
}

func removeIfExists(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func removeAllIfExists(path string) error {
	if err := os.RemoveAll(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func autocrmDataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".autocrm"), nil
}
