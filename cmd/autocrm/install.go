package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/akpiya/autocrm/internal/common"
	"github.com/akpiya/autocrm/internal/notion"
)

func runInstall() int {
	reader := bufio.NewReader(os.Stdin)
	fmt.Fprintln(os.Stdout, "AutoCRM installer")
	fmt.Fprintln(os.Stdout)
	printNotionSetupInstructions(os.Stdout)

	token := promptRequired(reader, "Notion integration token")
	databaseID := promptRequired(reader, "Notion database ID")
	interval := promptInterval(reader)

	cfg := notion.Config{
		Token:      token,
		DatabaseID: databaseID,
		Props:      common.NotionPropertyConfig(),
	}
	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, "Validating Notion database...")
	result, err := notion.ValidateDatabase(&http.Client{Timeout: 30 * time.Second}, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Notion validation failed: %v\n", err)
		return 1
	}
	if len(result.MissingProperties) > 0 {
		fmt.Fprintf(os.Stderr, "Notion database is missing required properties: %s\n", strings.Join(result.MissingProperties, ", "))
		return 1
	}

	installedPath, err := installBinary()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Install failed: %v\n", err)
		return 1
	}
	plistPath, err := writeLaunchAgent(launchAgentConfig{
		BinaryPath:       installedPath,
		NotionToken:      token,
		NotionDatabaseID: databaseID,
		StartInterval:    interval,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "LaunchAgent setup failed: %v\n", err)
		return 1
	}

	fmt.Fprintln(os.Stdout)
	printFullDiskAccessInstructions(os.Stdout, installedPath)
	openFullDiskAccessSettings()
	waitForEnter(reader, "Press Enter after Full Disk Access is enabled for AutoCRM.")

	if err := reloadLaunchAgent(plistPath); err != nil {
		fmt.Fprintf(os.Stderr, "LaunchAgent reload failed: %v\n", err)
		return 1
	}

	fmt.Fprintln(os.Stdout)
	fmt.Fprintf(os.Stdout, "Installed binary: %s\n", installedPath)
	fmt.Fprintf(os.Stdout, "Installed LaunchAgent: %s\n", plistPath)
	fmt.Fprintln(os.Stdout, "AutoCRM is loaded and will run in the background.")
	return 0
}

func printNotionSetupInstructions(w io.Writer) {
	fmt.Fprintln(w, "Before continuing, make sure you have:")
	fmt.Fprintf(w, "1. Created a Notion integration at %s\n", "https://www.notion.so/my-integrations")
	fmt.Fprintln(w, "2. Created a Notion people database with these exact properties:")
	fmt.Fprintf(w, "   - %s\n", common.NotionPhonesProp)
	fmt.Fprintf(w, "   - %s\n", common.NotionEmailsProp)
	fmt.Fprintf(w, "   - %s\n", common.NotionLastContactedProp)
	fmt.Fprintf(w, "   - %s\n", common.NotionLastChannelProp)
	fmt.Fprintln(w, "3. Shared that database with your Notion integration")
	fmt.Fprintln(w)
}

func promptRequired(reader *bufio.Reader, label string) string {
	for {
		fmt.Fprintf(os.Stdout, "%s: ", label)
		value, _ := reader.ReadString('\n')
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
		fmt.Fprintln(os.Stdout, "Value is required.")
	}
}

func promptInterval(reader *bufio.Reader) int {
	for {
		fmt.Fprint(os.Stdout, "Sync interval in minutes [5]: ")
		value, _ := reader.ReadString('\n')
		value = strings.TrimSpace(value)
		if value == "" {
			return defaultInterval
		}
		minutes, err := strconv.Atoi(value)
		if err == nil && minutes > 0 {
			return minutes * 60
		}
		fmt.Fprintln(os.Stdout, "Enter a positive whole number of minutes.")
	}
}

func waitForEnter(reader *bufio.Reader, message string) {
	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, message)
	_, _ = reader.ReadString('\n')
}

func installBinary() (string, error) {
	src, err := os.Executable()
	if err != nil {
		return "", err
	}
	src, err = filepath.EvalSymlinks(src)
	if err != nil {
		return "", err
	}
	dst, err := installedBinaryPath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return "", err
	}
	if err := copyFile(src, dst, 0o755); err != nil {
		return "", err
	}
	return dst, nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Chmod(dst, mode)
}

func writeLaunchAgent(cfg launchAgentConfig) (string, error) {
	path, err := launchAgentPath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(renderLaunchAgentPlist(cfg)), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func reloadLaunchAgent(plistPath string) error {
	target := fmt.Sprintf("gui/%d", os.Getuid())
	_ = exec.Command("launchctl", "bootout", target, plistPath).Run()
	return exec.Command("launchctl", "bootstrap", target, plistPath).Run()
}

func printFullDiskAccessInstructions(w io.Writer, binaryPath string) {
	fmt.Fprintln(w, "Full Disk Access is required before AutoCRM can read Messages and call history.")
	fmt.Fprintln(w, "Open System Settings > Privacy & Security > Full Disk Access.")
	fmt.Fprintf(w, "Add and enable this file: %s\n", binaryPath)
	fmt.Fprintln(w, "After enabling access, AutoCRM will run in the background on the configured interval.")
	fmt.Fprintln(w, "Logs:")
	fmt.Fprintln(w, "  /tmp/autocrm.log")
	fmt.Fprintln(w, "  /tmp/autocrm.err")
}

func openFullDiskAccessSettings() {
	_ = exec.Command("open", "x-apple.systempreferences:com.apple.preference.security?Privacy_AllFiles").Run()
}
