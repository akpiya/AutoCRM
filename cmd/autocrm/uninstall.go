package main

import (
	"fmt"
	"os"
	"os/exec"
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

	appPath, err := installedAppPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to resolve installed app path: %v\n", err)
		return 1
	}
	if err := removeAllIfExists(appPath); err != nil {
		fmt.Fprintf(os.Stderr, "Unable to remove installed app: %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stdout, "Removed installed app: %s\n", appPath)

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
