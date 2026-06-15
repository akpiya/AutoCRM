// Package common provides shared paths, time helpers, and Notion tuning constants.
package common

import (
	"os"
	"path/filepath"
	"time"
)

var AppleEpochUTC = time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)

const (
	DirectionInbound  = 0
	DirectionOutbound = 1
	PlatformText      = "text"

	NotionNameProp           = "Name"
	NotionPhonesProp         = "Phones"
	NotionEmailsProp         = "Emails"
	NotionLastContactedProp  = "Last Contacted"
	NotionLastChannelProp    = "Last Channel"
	NotionMinInterval        = 0.35
	NotionPatchWorkers       = 2
)

// DefaultChatDBPath is the macOS Messages database.
func DefaultChatDBPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "Messages", "chat.db")
}

// NotionPropertyConfig returns property name map for Notion API calls.
func NotionPropertyConfig() map[string]string {
	return map[string]string{
		"PHONES_PROP":          NotionPhonesProp,
		"EMAILS_PROP":          NotionEmailsProp,
		"LAST_CONTACTED_PROP":  NotionLastContactedProp,
		"LAST_CHANNEL_PROP":    NotionLastChannelProp,
	}
}

// AutocrmDir returns ~/.autocrm, creating it if needed.
func AutocrmDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".autocrm")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// OutboxDBPath is ~/.autocrm/outbox.db.
func OutboxDBPath() (string, error) {
	dir, err := AutocrmDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "outbox.db"), nil
}

// AppleNsToUnix converts Apple message.date nanoseconds to Unix seconds.
func AppleNsToUnix(ns int64) (float64, bool) {
	if ns == 0 {
		return 0, false
	}
	base := float64(AppleEpochUTC.Unix()) + float64(AppleEpochUTC.Nanosecond())/1e9
	return base + float64(ns)/1e9, true
}
