package main

import (
	"fmt"
	"net/http"
	"os"
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

func runDoctor() int {
	checks := []doctorCheck{
		checkAutocrmDir(),
		checkInstalledBinary(),
		checkMessagesDB(),
		checkCallHistoryDB(),
		checkLaunchAgent(),
		checkNotion(),
	}

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

func checkInstalledBinary() doctorCheck {
	expected, err := installedBinaryPath()
	if err != nil {
		return doctorCheck{name: "installed binary location", err: err}
	}
	actual, err := os.Executable()
	if err != nil {
		return doctorCheck{name: "installed binary location", err: err}
	}
	actual, err = filepath.EvalSymlinks(actual)
	if err != nil {
		return doctorCheck{name: "installed binary location", err: err}
	}
	expected, err = filepath.EvalSymlinks(expected)
	if err != nil {
		return doctorCheck{name: "installed binary location", err: err}
	}
	if actual != expected {
		return doctorCheck{
			name: "installed binary location",
			err:  fmt.Errorf("running %s; expected %s", actual, expected),
		}
	}
	return doctorCheck{name: "installed binary location"}
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

func launchAgentPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", launchAgentLabel+".plist"), nil
}
