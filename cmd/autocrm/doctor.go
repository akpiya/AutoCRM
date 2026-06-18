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
	"github.com/akpiya/autocrm/internal/outbox"
)

type doctorCheck struct {
	name string
	err  error
}

func runDoctor() int {
	checks := []doctorCheck{
		checkAutocrmDir(),
		checkOutboxPath(),
		checkMessagesDB(),
		checkCallHistoryDB(),
		checkLaunchAgent(),
	}
	checks = append(checks, checkNotion())

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
	_, err := common.AutocrmDir()
	return doctorCheck{name: "~/.autocrm directory", err: err}
}

func checkOutboxPath() doctorCheck {
	path, err := common.OutboxDBPath()
	if err != nil {
		return doctorCheck{name: "outbox path", err: err}
	}
	if err := outbox.InitDB(path); err != nil {
		return doctorCheck{name: "outbox database", err: err}
	}
	return doctorCheck{name: "outbox database"}
}

func checkMessagesDB() doctorCheck {
	path := common.DefaultChatDBPath()
	if _, err := os.Stat(path); err != nil {
		return doctorCheck{name: "Messages database", err: err}
	}
	if _, err := collectors.MaxMessageRowid(path); err != nil {
		return doctorCheck{name: "Messages database", err: err}
	}
	return doctorCheck{name: "Messages database"}
}

func checkCallHistoryDB() doctorCheck {
	path := common.DefaultCallHistoryDBPath()
	if _, err := os.Stat(path); err != nil {
		return doctorCheck{name: "Call history database", err: err}
	}
	if _, err := collectors.MaxCallRowid(path); err != nil {
		return doctorCheck{name: "Call history database", err: err}
	}
	return doctorCheck{name: "Call history database"}
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
