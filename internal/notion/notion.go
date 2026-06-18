// Package notion drains the local outbox into Notion.
package notion

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/akpiya/autocrm/internal/common"
	"github.com/akpiya/autocrm/internal/outbox"
)

// PageUpdate is a planned Notion PATCH for one page.
type PageUpdate struct {
	PageID     string
	Platform   string
	OccurredAt time.Time
	RowIDs     []int64
}

// Config holds Notion credentials and property names.
type Config struct {
	Token      string
	DatabaseID string
	Props      map[string]string
}

// ValidationResult describes whether a Notion database is usable by AutoCRM.
type ValidationResult struct {
	MissingProperties []string
}

// Configured reports whether Notion env vars are set.
func Configured() bool {
	return strings.TrimSpace(os.Getenv("NOTION_TOKEN")) != "" &&
		strings.TrimSpace(os.Getenv("NOTION_DATABASE_ID")) != ""
}

// LoadConfigFromEnv loads Notion config or returns an error.
func LoadConfigFromEnv() (Config, error) {
	token := strings.TrimSpace(os.Getenv("NOTION_TOKEN"))
	dbID := strings.TrimSpace(os.Getenv("NOTION_DATABASE_ID"))
	if token == "" || dbID == "" {
		return Config{}, fmt.Errorf("set NOTION_TOKEN and NOTION_DATABASE_ID")
	}
	return Config{
		Token:      token,
		DatabaseID: dbID,
		Props:      common.NotionPropertyConfig(),
	}, nil
}

// ValidateDatabase checks that the configured Notion database is reachable and
// exposes the properties AutoCRM needs.
func ValidateDatabase(client *http.Client, cfg Config) (ValidationResult, error) {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	props := cfg.Props
	if props == nil {
		props = common.NotionPropertyConfig()
	}
	url := "https://api.notion.com/v1/databases/" + cfg.DatabaseID
	db, err := notionRequest(client, http.MethodGet, url, cfg.Token, nil)
	if err != nil {
		return ValidationResult{}, err
	}
	rawProps, _ := db["properties"].(map[string]any)
	required := []string{
		props["PHONES_PROP"],
		props["EMAILS_PROP"],
		props["LAST_CONTACTED_PROP"],
		props["LAST_CHANNEL_PROP"],
	}
	var missing []string
	for _, name := range required {
		if _, ok := rawProps[name]; !ok {
			missing = append(missing, name)
		}
	}
	return ValidationResult{MissingProperties: missing}, nil
}

// PhoneDigits strips non-digits from a phone string.
func PhoneDigits(p string) string {
	var b strings.Builder
	for _, c := range p {
		if c >= '0' && c <= '9' {
			b.WriteRune(c)
		}
	}
	return b.String()
}

// PhoneMatchVariants returns digit forms used for phone comparison.
func PhoneMatchVariants(p string) map[string]struct{} {
	digits := PhoneDigits(p)
	out := map[string]struct{}{}
	if digits == "" {
		return out
	}
	out[digits] = struct{}{}
	if len(digits) == 11 && digits[0] == '1' {
		out[digits[1:]] = struct{}{}
	} else if len(digits) == 10 {
		out["1"+digits] = struct{}{}
	}
	return out
}

// NormalizeEmail lowercases and trims an email.
func NormalizeEmail(e string) string {
	return strings.ToLower(strings.TrimSpace(e))
}

// PlatformToChannelOption maps outbox platform to a Notion select name.
func PlatformToChannelOption(platform string) string {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case "text":
		return "Text"
	case "phone_call":
		return "Phone"
	case "facetime_audio", "facetime_video":
		return "Facetime"
	default:
		return strings.TrimSpace(platform)
	}
}

// ChannelPropertyPayload builds Notion property value for Last Channel.
func ChannelPropertyPayload(platform string, existingProp map[string]any) map[string]any {
	label := PlatformToChannelOption(platform)
	propType, _ := existingProp["type"].(string)
	if propType == "rich_text" {
		return map[string]any{
			"rich_text": []map[string]any{
				{"type": "text", "text": map[string]any{"content": truncate(label, 2000)}},
			},
		}
	}
	return map[string]any{"select": map[string]any{"name": label}}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// PartyContactKeys returns phone and email match keys for a party_id.
func PartyContactKeys(partyID string) (phones map[string]struct{}, emails map[string]struct{}) {
	p := strings.TrimSpace(partyID)
	phones = map[string]struct{}{}
	emails = map[string]struct{}{}
	if p == "" {
		return phones, emails
	}
	if strings.Contains(p, "@") {
		emails[NormalizeEmail(p)] = struct{}{}
		return phones, emails
	}
	return PhoneMatchVariants(p), emails
}

// MatchPageForParty finds a Notion page id for party_id.
func MatchPageForParty(partyID string, pages []map[string]any, cfg map[string]string) string {
	hp, he := PartyContactKeys(partyID)
	if len(hp) == 0 && len(he) == 0 {
		return ""
	}
	phonesProp := cfg["PHONES_PROP"]
	emailsProp := cfg["EMAILS_PROP"]
	for _, page := range pages {
		pp, ee := pagePhonesEmails(page, phonesProp, emailsProp)
		if intersects(hp, pp) || intersects(he, ee) {
			if id, ok := page["id"].(string); ok {
				return id
			}
		}
	}
	return ""
}

func intersects(a, b map[string]struct{}) bool {
	for k := range a {
		if _, ok := b[k]; ok {
			return true
		}
	}
	return false
}

func pagePhonesEmails(page map[string]any, phonesProp, emailsProp string) (phones, emails map[string]struct{}) {
	phones = map[string]struct{}{}
	emails = map[string]struct{}{}
	props, _ := page["properties"].(map[string]any)
	if props == nil {
		return phones, emails
	}
	if p, ok := props[phonesProp].(map[string]any); ok {
		mergePhones(p, phones)
	}
	if e, ok := props[emailsProp].(map[string]any); ok {
		mergeEmails(e, emails)
	}
	return phones, emails
}

func mergePhones(p map[string]any, phones map[string]struct{}) {
	switch p["type"] {
	case "multi_select":
		if opts, ok := p["multi_select"].([]any); ok {
			for _, o := range opts {
				if m, ok := o.(map[string]any); ok {
					for k := range PhoneMatchVariants(fmt.Sprint(m["name"])) {
						phones[k] = struct{}{}
					}
				}
			}
		}
	case "rich_text":
		if opts, ok := p["rich_text"].([]any); ok {
			for _, o := range opts {
				if m, ok := o.(map[string]any); ok {
					for k := range PhoneMatchVariants(fmt.Sprint(m["plain_text"])) {
						phones[k] = struct{}{}
					}
				}
			}
		}
	case "phone_number":
		if n := p["phone_number"]; n != nil {
			for k := range PhoneMatchVariants(fmt.Sprint(n)) {
				phones[k] = struct{}{}
			}
		}
	}
}

func mergeEmails(e map[string]any, emails map[string]struct{}) {
	switch e["type"] {
	case "email":
		if em := e["email"]; em != nil {
			emails[NormalizeEmail(fmt.Sprint(em))] = struct{}{}
		}
	case "multi_select":
		if opts, ok := e["multi_select"].([]any); ok {
			for _, o := range opts {
				if m, ok := o.(map[string]any); ok {
					emails[NormalizeEmail(fmt.Sprint(m["name"]))] = struct{}{}
				}
			}
		}
	case "rich_text":
		if opts, ok := e["rich_text"].([]any); ok {
			for _, o := range opts {
				if m, ok := o.(map[string]any); ok {
					emails[NormalizeEmail(fmt.Sprint(m["plain_text"]))] = struct{}{}
				}
			}
		}
	}
}

// PlanPageUpdates groups outbox rows into per-page updates and unmatched delete ids.
func PlanPageUpdates(rows []outbox.Row, pages []map[string]any, cfg map[string]string) ([]PageUpdate, []int64) {
	var deleteIDs []int64
	byPage := map[string][]outbox.Row{}

	for _, row := range rows {
		if row.Direction != common.DirectionInbound && row.Direction != common.DirectionOutbound {
			deleteIDs = append(deleteIDs, row.ID)
			continue
		}
		pageID := MatchPageForParty(row.PartyID, pages, cfg)
		if pageID == "" {
			deleteIDs = append(deleteIDs, row.ID)
			continue
		}
		byPage[pageID] = append(byPage[pageID], row)
	}

	var updates []PageUpdate
	for pageID, pageRows := range byPage {
		best := pageRows[0]
		for _, r := range pageRows[1:] {
			if r.CreatedAt > best.CreatedAt {
				best = r
			}
		}
		var rowIDs []int64
		for _, r := range pageRows {
			rowIDs = append(rowIDs, r.ID)
		}
		updates = append(updates, PageUpdate{
			PageID:     pageID,
			Platform:   best.Platform,
			OccurredAt: unixToUTC(best.CreatedAt),
			RowIDs:     rowIDs,
		})
	}
	return updates, deleteIDs
}

func unixToUTC(ts float64) time.Time {
	sec := int64(ts)
	nsec := int64((ts - float64(sec)) * 1e9)
	return time.Unix(sec, nsec).UTC()
}

func parseNotionDatetime(val map[string]any) *time.Time {
	if val == nil || val["type"] != "date" {
		return nil
	}
	d, _ := val["date"].(map[string]any)
	if d == nil {
		return nil
	}
	start, _ := d["start"].(string)
	if start == "" {
		return nil
	}
	if len(start) == 10 {
		start += "T00:00:00+00:00"
	}
	start = strings.Replace(start, "Z", "+00:00", 1)
	t, err := time.Parse(time.RFC3339, start)
	if err != nil {
		// try without timezone
		t, err = time.Parse("2006-01-02T15:04:05.000-07:00", start)
		if err != nil {
			return nil
		}
	}
	utc := t.UTC()
	return &utc
}

func notionRequest(client *http.Client, method, url, token string, body map[string]any) (map[string]any, error) {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, url, r)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Notion-Version", "2022-06-28")
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		log.Printf("Notion HTTP %d: %s", resp.StatusCode, string(data))
		return nil, fmt.Errorf("notion http %d", resp.StatusCode)
	}
	var out map[string]any
	if len(data) > 0 {
		if err := json.Unmarshal(data, &out); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func fetchAllPages(client *http.Client, databaseID, token string) ([]map[string]any, error) {
	var pages []map[string]any
	var cursor string
	base := "https://api.notion.com/v1/databases/" + databaseID + "/query"
	for {
		body := map[string]any{"page_size": 100}
		if cursor != "" {
			body["start_cursor"] = cursor
		}
		r, err := notionRequest(client, http.MethodPost, base, token, body)
		if err != nil {
			return nil, err
		}
		if results, ok := r["results"].([]any); ok {
			for _, p := range results {
				if m, ok := p.(map[string]any); ok {
					pages = append(pages, m)
				}
			}
		}
		if more, _ := r["has_more"].(bool); !more {
			break
		}
		if c, ok := r["next_cursor"].(string); ok {
			cursor = c
		} else {
			break
		}
	}
	return pages, nil
}

func patchPage(client *http.Client, pageID string, occurredAt time.Time, platform, token string, cfg map[string]string, cachedPage map[string]any) (bool, error) {
	lastProp := cfg["LAST_CONTACTED_PROP"]
	chanProp := cfg["LAST_CHANNEL_PROP"]
	url := "https://api.notion.com/v1/pages/" + pageID
	props := map[string]any{}
	if cachedPage != nil {
		if p, ok := cachedPage["properties"].(map[string]any); ok {
			props = p
		}
	}
	if len(props) == 0 {
		page, err := notionRequest(client, http.MethodGet, url, token, nil)
		if err != nil {
			return false, err
		}
		if p, ok := page["properties"].(map[string]any); ok {
			props = p
		}
	}
	var existingChan map[string]any
	if c, ok := props[chanProp].(map[string]any); ok {
		existingChan = c
	}
	current := parseNotionDatetime(asMap(props[lastProp]))
	occ := occurredAt.UTC()
	if current != nil && occ.Before(*current) {
		return false, nil
	}
	body := map[string]any{
		"properties": map[string]any{
			lastProp: map[string]any{
				"date": map[string]any{"start": occ.Format("2006-01-02T15:04:05.000Z")},
			},
			chanProp: ChannelPropertyPayload(platform, existingChan),
		},
	}
	if _, err := notionRequest(client, http.MethodPatch, url, token, body); err != nil {
		return false, err
	}
	return true, nil
}

func asMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return nil
}

// SyncStats is returned from SyncOutbox.
type SyncStats struct {
	Applied int
	Errors  int
	Pending int
}

// SyncOutbox drains the outbox into Notion.
func SyncOutbox(dbPath string, cfg *Config, minInterval float64) (SyncStats, error) {
	if minInterval == 0 {
		minInterval = common.NotionMinInterval
	}
	c := Config{}
	if cfg != nil {
		c = *cfg
	} else {
		var err error
		c, err = LoadConfigFromEnv()
		if err != nil {
			return SyncStats{}, err
		}
	}
	if dbPath == "" {
		var err error
		dbPath, err = common.OutboxDBPath()
		if err != nil {
			return SyncStats{}, err
		}
	}
	cfgMap := common.NotionPropertyConfig()
	client := &http.Client{Timeout: 30 * time.Second}

	t0 := time.Now()
	pending, err := outbox.FetchAll(dbPath)
	if err != nil {
		return SyncStats{}, err
	}
	if len(pending) == 0 {
		log.Printf("Notion sync: no pending rows (%.2fs)", time.Since(t0).Seconds())
		return SyncStats{}, nil
	}

	allPages, err := fetchAllPages(client, c.DatabaseID, c.Token)
	if err != nil {
		return SyncStats{}, err
	}
	pagesByID := map[string]map[string]any{}
	for _, p := range allPages {
		if id, ok := p["id"].(string); ok {
			pagesByID[id] = p
		}
	}

	updates, deleteIDs := PlanPageUpdates(pending, allPages, cfgMap)

	applied, skipped, errors := 0, 0, 0
	var mu sync.Mutex
	lastStart := time.Time{}
	workers := common.NotionPatchWorkers
	if workers < 1 {
		workers = 1
	}

	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
	var resultMu sync.Mutex
	var toDelete []int64
	toDelete = append(toDelete, deleteIDs...)

	for _, update := range updates {
		update := update
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			mu.Lock()
			if minInterval > 0 && !lastStart.IsZero() {
				wait := minInterval - time.Since(lastStart).Seconds()
				if wait > 0 {
					time.Sleep(time.Duration(wait * float64(time.Second)))
				}
			}
			lastStart = time.Now()
			mu.Unlock()

			patched, err := patchPage(
				client, update.PageID, update.OccurredAt, update.Platform,
				c.Token, cfgMap, pagesByID[update.PageID],
			)
			resultMu.Lock()
			defer resultMu.Unlock()
			if err != nil {
				errors++
				return
			}
			toDelete = append(toDelete, update.RowIDs...)
			if patched {
				applied++
			} else {
				skipped++
			}
		}()
	}
	wg.Wait()

	if err := outbox.DeleteRows(toDelete, dbPath); err != nil {
		return SyncStats{}, err
	}

	log.Printf(
		"Notion sync timing: %.2fs total | pending=%d pages=%d updates=%d applied=%d skipped=%d errors=%d",
		time.Since(t0).Seconds(), len(pending), len(allPages), len(updates), applied, skipped, errors,
	)
	return SyncStats{Applied: applied, Errors: errors, Pending: len(pending)}, nil
}
