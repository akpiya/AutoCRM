package notion_test

import (
	"testing"
	"time"

	"github.com/akpiya/autocrm/internal/common"
	"github.com/akpiya/autocrm/internal/notion"
	"github.com/akpiya/autocrm/internal/outbox"
)

var testCfg = map[string]string{
	"PHONES_PROP":         "Phones",
	"EMAILS_PROP":         "Emails",
	"LAST_CONTACTED_PROP": "Last Contacted",
	"LAST_CHANNEL_PROP":   "Last Channel",
}

func testPage(id string, phones, emails []string) map[string]any {
	props := map[string]any{
		"Phones": map[string]any{
			"type":         "multi_select",
			"multi_select": multiSelect(phones),
		},
		"Emails": map[string]any{
			"type":  "email",
			"email": nil,
		},
	}
	if len(emails) > 0 {
		props["Emails"] = map[string]any{"type": "email", "email": emails[0]}
	}
	return map[string]any{"id": id, "properties": props}
}

func multiSelect(names []string) []any {
	out := make([]any, len(names))
	for i, n := range names {
		out[i] = map[string]any{"name": n}
	}
	return out
}

func TestMatchPageForPartyPhone(t *testing.T) {
	pages := []map[string]any{testPage("p1", []string{"+15551234567"}, nil)}
	if got := notion.MatchPageForParty("+15551234567", pages, testCfg); got != "p1" {
		t.Fatalf("got %q", got)
	}
}

func TestMatchPagePhoneFormatVariants(t *testing.T) {
	pages := []map[string]any{testPage("p1", []string{"(555) 123-4567"}, nil)}
	if notion.MatchPageForParty("+15551234567", pages, testCfg) != "p1" {
		t.Fatal("variant +1")
	}
	if notion.MatchPageForParty("5551234567", pages, testCfg) != "p1" {
		t.Fatal("variant 10-digit")
	}
}

func TestPlatformToChannelOption(t *testing.T) {
	if notion.PlatformToChannelOption("text") != "Text" {
		t.Fatal()
	}
	if notion.PlatformToChannelOption("TEXT") != "Text" {
		t.Fatal()
	}
	if notion.PlatformToChannelOption("phone_call") != "Phone" {
		t.Fatal()
	}
	if notion.PlatformToChannelOption("facetime_audio") != "Facetime" {
		t.Fatal()
	}
	if notion.PlatformToChannelOption("facetime_video") != "Facetime" {
		t.Fatal()
	}
}

func TestChannelPropertyPayloadSelect(t *testing.T) {
	got := notion.ChannelPropertyPayload("text", map[string]any{"type": "select"})
	want := map[string]any{"select": map[string]any{"name": "Text"}}
	if fmtSelect(got) != fmtSelect(want) {
		t.Fatalf("got %v", got)
	}

	gotPhone := notion.ChannelPropertyPayload("phone_call", map[string]any{"type": "select"})
	wantPhone := map[string]any{"select": map[string]any{"name": "Phone"}}
	if fmtSelect(gotPhone) != fmtSelect(wantPhone) {
		t.Fatalf("got %v", gotPhone)
	}

	gotFacetime := notion.ChannelPropertyPayload("facetime_video", map[string]any{"type": "select"})
	wantFacetime := map[string]any{"select": map[string]any{"name": "Facetime"}}
	if fmtSelect(gotFacetime) != fmtSelect(wantFacetime) {
		t.Fatalf("got %v", gotFacetime)
	}
}

func fmtSelect(m map[string]any) string {
	s, _ := m["select"].(map[string]any)
	return s["name"].(string)
}

func TestPhoneMatchVariantsUSTenAndEleven(t *testing.T) {
	a := notion.PhoneMatchVariants("+17033955764")
	b := notion.PhoneMatchVariants("7033955764")
	for k := range a {
		if _, ok := b[k]; ok {
			return
		}
	}
	t.Fatal("expected overlap")
}

func TestMatchPageForPartyEmail(t *testing.T) {
	pages := []map[string]any{testPage("p2", nil, []string{"friend@example.com"})}
	if notion.MatchPageForParty("friend@example.com", pages, testCfg) != "p2" {
		t.Fatal()
	}
}

func TestPlanPageUpdatesIncludesInbound(t *testing.T) {
	pages := []map[string]any{testPage("p1", []string{"+15551234567"}, nil)}
	rows := []outbox.Row{
		{ID: 1, Platform: common.PlatformText, PartyID: "+15551234567", Direction: common.DirectionInbound, CreatedAt: 100},
		{ID: 2, Platform: common.PlatformText, PartyID: "+15551234567", Direction: common.DirectionOutbound, CreatedAt: 50},
	}
	updates, deleteIDs := notion.PlanPageUpdates(rows, pages, testCfg)
	if len(deleteIDs) != 0 || len(updates) != 1 {
		t.Fatalf("updates=%v delete=%v", updates, deleteIDs)
	}
	if updates[0].PageID != "p1" || updates[0].OccurredAt.Unix() != 100 {
		t.Fatalf("update=%+v", updates[0])
	}
}

func TestPlanPageUpdatesGroupsAndDropsUnmatched(t *testing.T) {
	pages := []map[string]any{testPage("p1", []string{"+15551234567"}, nil)}
	rows := []outbox.Row{
		{ID: 1, Platform: common.PlatformText, PartyID: "+15551234567", Direction: common.DirectionOutbound, CreatedAt: 100},
		{ID: 2, Platform: common.PlatformText, PartyID: "+15551234567", Direction: common.DirectionOutbound, CreatedAt: 200},
		{ID: 3, Platform: common.PlatformText, PartyID: "unknown@example.com", Direction: common.DirectionOutbound, CreatedAt: 150},
		{ID: 4, Platform: common.PlatformText, PartyID: "+1999", Direction: common.DirectionOutbound, CreatedAt: 50},
	}
	updates, deleteIDs := notion.PlanPageUpdates(rows, pages, testCfg)
	if len(updates) != 1 {
		t.Fatalf("updates=%v", updates)
	}
	if updates[0].OccurredAt.Unix() != 200 {
		t.Fatalf("ts=%d", updates[0].OccurredAt.Unix())
	}
	rowSet := map[int64]bool{}
	for _, id := range updates[0].RowIDs {
		rowSet[id] = true
	}
	if !rowSet[1] || !rowSet[2] {
		t.Fatalf("rowIDs=%v", updates[0].RowIDs)
	}
	delSet := map[int64]bool{}
	for _, id := range deleteIDs {
		delSet[id] = true
	}
	if !delSet[3] || !delSet[4] {
		t.Fatalf("delete=%v", deleteIDs)
	}
	_ = time.UTC
}
