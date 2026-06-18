package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
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
	if askYesNo("Remove installed binary at " + binaryPath + "? [y/N]: ") {
		if err := removeIfExists(binaryPath); err != nil {
			fmt.Fprintf(os.Stderr, "Unable to remove installed binary: %v\n", err)
			return 1
		}
		fmt.Fprintf(os.Stdout, "Removed installed binary: %s\n", binaryPath)
	}

	fmt.Fprintln(os.Stdout, "Kept ~/.autocrm data directory.")
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

func askYesNo(prompt string) bool {
	fmt.Fprint(os.Stdout, prompt)
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes"
}
