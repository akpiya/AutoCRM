package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
)

const (
	launchAgentLabel = "com.user.autocrm"
	defaultInterval  = 300
)

type launchAgentConfig struct {
	BinaryPath       string
	NotionToken      string
	NotionDatabaseID string
	StartInterval    int
}

func installedBinaryPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "bin", "autocrm"), nil
}

func launchAgentPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", launchAgentLabel+".plist"), nil
}

func renderLaunchAgentPlist(cfg launchAgentConfig) string {
	interval := cfg.StartInterval
	if interval <= 0 {
		interval = defaultInterval
	}
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>%s</string>

  <key>RunAtLoad</key>
  <true/>

  <key>StartInterval</key>
  <integer>%d</integer>

  <key>EnvironmentVariables</key>
  <dict>
    <key>PATH</key>
    <string>/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin</string>
    <key>NOTION_TOKEN</key>
    <string>%s</string>
    <key>NOTION_DATABASE_ID</key>
    <string>%s</string>
  </dict>

  <key>ProgramArguments</key>
  <array>
    <string>%s</string>
    <string>run</string>
  </array>

  <key>StandardOutPath</key>
  <string>/tmp/autocrm.log</string>
  <key>StandardErrorPath</key>
  <string>/tmp/autocrm.err</string>
</dict>
</plist>
`,
		escapeXML(launchAgentLabel),
		interval,
		escapeXML(cfg.NotionToken),
		escapeXML(cfg.NotionDatabaseID),
		escapeXML(cfg.BinaryPath),
	)
}

func escapeXML(s string) string {
	var b bytes.Buffer
	if err := xml.EscapeText(&b, []byte(s)); err != nil {
		return s
	}
	return b.String()
}
