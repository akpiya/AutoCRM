package collectors_test

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/akpiya/autocrm/internal/collectors"
	"github.com/akpiya/autocrm/internal/common"
	"github.com/akpiya/autocrm/internal/outbox"
	"github.com/akpiya/autocrm/internal/testfixtures"

	_ "github.com/mattn/go-sqlite3"
)

func testCollector(t *testing.T) *collectors.IMessageCollector {
	t.Helper()
	dir := t.TempDir()
	chatDB := filepath.Join(dir, "chat.db")
	if err := testfixtures.BuildChatDB(chatDB); err != nil {
		t.Fatal(err)
	}
	outboxDB := filepath.Join(dir, "outbox.db")
	if err := outbox.InitDB(outboxDB); err != nil {
		t.Fatal(err)
	}
	z := 0.0
	if _, err := outbox.IngestBatch("imessage", nil, &z, outboxDB); err != nil {
		t.Fatal(err)
	}
	return &collectors.IMessageCollector{
		AppName:      "imessage",
		ChatDBPath:   chatDB,
		OutboxDBPath: outboxDB,
	}
}

func outboxRows(t *testing.T, dbPath string) [][4]any {
	t.Helper()
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := db.Query(
		`SELECT platform, party_id, direction, created_at FROM outbox ORDER BY id`,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out [][4]any
	for rows.Next() {
		var platform, party string
		var direction int
		var created float64
		if err := rows.Scan(&platform, &party, &direction, &created); err != nil {
			t.Fatal(err)
		}
		out = append(out, [4]any{platform, party, direction, created})
	}
	return out
}

func TestCollectEnqueuesAndAdvancesCursor(t *testing.T) {
	c := testCollector(t)
	result, err := c.Collect()
	if err != nil {
		t.Fatal(err)
	}
	if result.Enqueued != 8 {
		t.Fatalf("enqueued=%d", result.Enqueued)
	}
	if result.CursorBefore == nil || *result.CursorBefore != 0 {
		t.Fatalf("before=%v", result.CursorBefore)
	}
	if result.CursorAfter == nil || *result.CursorAfter != 7 {
		t.Fatalf("after=%v", result.CursorAfter)
	}

	rows := outboxRows(t, c.OutboxDBPath)
	if len(rows) != 8 {
		t.Fatalf("rows=%d", len(rows))
	}
	parties := map[string]struct{}{}
	directions := map[int]struct{}{}
	inbound := 0
	for _, r := range rows {
		if r[0] != common.PlatformText {
			t.Fatalf("platform=%v", r[0])
		}
		parties[r[1].(string)] = struct{}{}
		directions[r[2].(int)] = struct{}{}
		if r[2].(int) == common.DirectionInbound {
			inbound++
		}
	}
	wantParties := map[string]struct{}{
		"+15551234567": {}, "+15559876543": {}, "friend@example.com": {},
	}
	if len(parties) != len(wantParties) {
		t.Fatalf("parties=%v", parties)
	}
	if len(directions) != 2 {
		t.Fatalf("directions=%v", directions)
	}
	if inbound != 3 {
		t.Fatalf("inbound=%d", inbound)
	}
	cur, err := outbox.GetCursor("imessage", c.OutboxDBPath)
	if err != nil || cur == nil || *cur != 7 {
		t.Fatalf("cursor=%v", cur)
	}
}

func TestCollectSecondRunEmpty(t *testing.T) {
	c := testCollector(t)
	if _, err := c.Collect(); err != nil {
		t.Fatal(err)
	}
	result, err := c.Collect()
	if err != nil {
		t.Fatal(err)
	}
	if result.Enqueued != 0 {
		t.Fatalf("enqueued=%d", result.Enqueued)
	}
	if result.CursorBefore == nil || *result.CursorBefore != 7 {
		t.Fatalf("before=%v", result.CursorBefore)
	}
	if len(outboxRows(t, c.OutboxDBPath)) != 8 {
		t.Fatal("expected 8 rows")
	}
}

func TestMissingChatDBRaises(t *testing.T) {
	dir := t.TempDir()
	c := &collectors.IMessageCollector{
		ChatDBPath:   filepath.Join(dir, "missing.db"),
		OutboxDBPath: filepath.Join(dir, "outbox.db"),
	}
	_, err := c.Collect()
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, os.ErrNotExist) && err.Error() == "" {
		// wrapped message is fine
		if _, statErr := os.Stat(c.ChatDBPath); !os.IsNotExist(statErr) {
			t.Fatalf("err=%v", err)
		}
	}
}

func TestMissingCursorSkipsIngestAndSetsMaxRowid(t *testing.T) {
	dir := t.TempDir()
	chatDB := filepath.Join(dir, "chat.db")
	if err := testfixtures.BuildChatDB(chatDB); err != nil {
		t.Fatal(err)
	}
	outboxDB := filepath.Join(dir, "outbox.db")
	c := &collectors.IMessageCollector{
		ChatDBPath:   chatDB,
		OutboxDBPath: outboxDB,
	}
	result, err := c.Collect()
	if err != nil {
		t.Fatal(err)
	}
	if result.Enqueued != 0 || result.CursorBefore != nil {
		t.Fatalf("result=%+v", result)
	}
	if result.CursorAfter == nil || *result.CursorAfter != 7 {
		t.Fatalf("after=%v", result.CursorAfter)
	}
	cur, _ := outbox.GetCursor("imessage", outboxDB)
	if cur == nil || *cur != 7 {
		t.Fatalf("cursor=%v", cur)
	}
	db, _ := sql.Open("sqlite3", outboxDB)
	defer db.Close()
	var count int
	_ = db.QueryRow(`SELECT COUNT(*) FROM outbox`).Scan(&count)
	if count != 0 {
		t.Fatalf("count=%d", count)
	}
}
