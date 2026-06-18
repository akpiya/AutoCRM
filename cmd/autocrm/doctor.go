package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/akpiya/autocrm/internal/collectors"
	"github.com/akpiya/autocrm/internal/common"
	"github.com/akpiya/autocrm/internal/notion"
)

type doctorCheck struct {
	name string
	err  error
}

type fdaProbeResult struct {
	MessagesError    string `json:"messages_error,omitempty"`
	CallHistoryError string `json:"call_history_error,omitempty"`
}

func runDoctor() int {
	checks := []doctorCheck{
		checkAutocrmDir(),
		checkInstalledApp(),
		checkInstalledBinary(),
	}
	checks = append(checks, checkFullDiskAccessViaInstalledApp()...)
	checks = append(checks, []doctorCheck{
		checkLaunchAgent(),
		checkNotion(),
	}...)

	failed := 0
	for _, check := range checks {
		if check.err != nil {
			failed++
			fmt.Fprintf(os.Stdout, "FAIL %s: %v\n", check.name, check.err)
			continue
		}
		fmt.Fprintf(os.Stdout, "OK   %s\n", check.name)
	}
	if failed > 0 {
		fmt.Fprintf(os.Stdout, "\n%d check(s) failed.\n", failed)
		return 1
	}
	fmt.Fprintln(os.Stdout, "\nAll checks passed.")
	return 0
}

func checkAutocrmDir() doctorCheck {
	home, err := os.UserHomeDir()
	if err != nil {
		return doctorCheck{name: "~/.autocrm directory", err: err}
	}
	path := filepath.Join(home, ".autocrm")
	info, err := os.Stat(path)
	if err != nil {
		return doctorCheck{name: "~/.autocrm directory", err: err}
	}
	if !info.IsDir() {
		return doctorCheck{name: "~/.autocrm directory", err: fmt.Errorf("%s is not a directory", path)}
	}
	return doctorCheck{name: "~/.autocrm directory", err: err}
}

func checkInstalledApp() doctorCheck {
	path, err := installedAppPath()
	if err != nil {
		return doctorCheck{name: "installed app", err: err}
	}
	info, err := os.Stat(path)
	if err != nil {
		return doctorCheck{name: "installed app", err: err}
	}
	if !info.IsDir() {
		return doctorCheck{name: "installed app", err: fmt.Errorf("%s is not a directory", path)}
	}
	return doctorCheck{name: "installed app"}
}

func checkInstalledBinary() doctorCheck {
	expected, err := installedBinaryPath()
	if err != nil {
		return doctorCheck{name: "installed app binary location", err: err}
	}
	actual, err := os.Executable()
	if err != nil {
		return doctorCheck{name: "installed app binary location", err: err}
	}
	actual, err = filepath.EvalSymlinks(actual)
	if err != nil {
		return doctorCheck{name: "installed app binary location", err: err}
	}
	expected, err = filepath.EvalSymlinks(expected)
	if err != nil {
		return doctorCheck{name: "installed app binary location", err: err}
	}
	if actual != expected {
		return doctorCheck{
			name: "installed app binary location",
			err:  fmt.Errorf("running %s; expected %s", actual, expected),
		}
	}
	return doctorCheck{name: "installed app binary location"}
}

func checkFullDiskAccessViaInstalledApp() []doctorCheck {
	appPath, err := installedAppPath()
	if err != nil {
		return failedFullDiskAccessChecks(err)
	}
	resultPath, err := writeDoctorProbePath()
	if err != nil {
		return failedFullDiskAccessChecks(err)
	}
	defer os.Remove(resultPath)

	cmd := exec.Command("open", "-n", appPath, "--args", "doctor-fda-probe", resultPath)
	if err := cmd.Run(); err != nil {
		return failedFullDiskAccessChecks(fmt.Errorf("launch installed app FDA probe: %w", err))
	}

	var result fdaProbeResult
	deadline := time.Now().Add(15 * time.Second)
	for {
		data, err := os.ReadFile(resultPath)
		if err == nil {
			if err := json.Unmarshal(data, &result); err != nil {
				return failedFullDiskAccessChecks(fmt.Errorf("read installed app FDA probe: %w", err))
			}
			return fullDiskAccessChecksFromProbe(result)
		}
		if !os.IsNotExist(err) {
			return failedFullDiskAccessChecks(err)
		}
		if time.Now().After(deadline) {
			return failedFullDiskAccessChecks(fmt.Errorf("installed app FDA probe timed out"))
		}
		time.Sleep(250 * time.Millisecond)
	}
}

func writeDoctorProbePath() (string, error) {
	dir, err := autocrmDataDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	file, err := os.CreateTemp(dir, "doctor-fda-*.json")
	if err != nil {
		return "", err
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		os.Remove(path)
		return "", err
	}
	if err := os.Remove(path); err != nil {
		return "", err
	}
	return path, nil
}

func fullDiskAccessChecksFromProbe(result fdaProbeResult) []doctorCheck {
	checks := []doctorCheck{
		{name: "Full Disk Access: Messages database"},
		{name: "Full Disk Access: Call history database"},
	}
	if result.MessagesError != "" {
		checks[0].err = fmt.Errorf("%s", result.MessagesError)
	}
	if result.CallHistoryError != "" {
		checks[1].err = fmt.Errorf("%s", result.CallHistoryError)
	}
	return checks
}

func failedFullDiskAccessChecks(err error) []doctorCheck {
	return []doctorCheck{
		{name: "Full Disk Access: Messages database", err: err},
		{name: "Full Disk Access: Call history database", err: err},
	}
}

func runDoctorFDAProbe(args []string) int {
	if len(args) != 1 {
		return 2
	}
	result := fdaProbeResult{}
	if check := checkMessagesDB(); check.err != nil {
		result.MessagesError = check.err.Error()
	}
	if check := checkCallHistoryDB(); check.err != nil {
		result.CallHistoryError = check.err.Error()
	}

	data, err := json.Marshal(result)
	if err != nil {
		return 1
	}
	if err := os.WriteFile(args[0], data, 0o600); err != nil {
		return 1
	}
	return 0
}

func checkMessagesDB() doctorCheck {
	path := common.DefaultChatDBPath()
	if _, err := os.Stat(path); err != nil {
		return doctorCheck{name: "Full Disk Access: Messages database", err: err}
	}
	if _, err := collectors.MaxMessageRowid(path); err != nil {
		return doctorCheck{name: "Full Disk Access: Messages database", err: err}
	}
	return doctorCheck{name: "Full Disk Access: Messages database"}
}

func checkCallHistoryDB() doctorCheck {
	path := common.DefaultCallHistoryDBPath()
	if _, err := os.Stat(path); err != nil {
		return doctorCheck{name: "Full Disk Access: Call history database", err: err}
	}
	if _, err := collectors.MaxCallRowid(path); err != nil {
		return doctorCheck{name: "Full Disk Access: Call history database", err: err}
	}
	return doctorCheck{name: "Full Disk Access: Call history database"}
}

func checkLaunchAgent() doctorCheck {
	path, err := launchAgentPath()
	if err != nil {
		return doctorCheck{name: "LaunchAgent", err: err}
	}
	if _, err := os.Stat(path); err != nil {
		return doctorCheck{name: "LaunchAgent", err: err}
	}
	return doctorCheck{name: "LaunchAgent"}
}

func checkNotion() doctorCheck {
	cfg, err := notion.LoadConfigFromEnv()
	if err != nil {
		return doctorCheck{name: "Notion configuration", err: err}
	}
	client := &http.Client{Timeout: 30 * time.Second}
	result, err := notion.ValidateDatabase(client, cfg)
	if err != nil {
		return doctorCheck{name: "Notion database", err: err}
	}
	if len(result.MissingProperties) > 0 {
		return doctorCheck{
			name: "Notion database",
			err:  fmt.Errorf("missing required properties: %v", result.MissingProperties),
		}
	}
	return doctorCheck{name: "Notion database"}
}
