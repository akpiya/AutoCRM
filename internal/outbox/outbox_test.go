package outbox_test

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/akpiya/autocrm/internal/common"
	"github.com/akpiya/autocrm/internal/outbox"

	_ "github.com/mattn/go-sqlite3"
)

func TestIngestBatchWritesEventsAndCursor(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "outbox.db")
	events := []outbox.Event{
		{common.PlatformText, "+1", common.DirectionOutbound, 1_700_000_000},
		{common.PlatformText, "a@b.com", common.DirectionOutbound, 1_700_000_001},
	}
	cv := 42.0
	n, err := outbox.IngestBatch("imessage", events, &cv, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("n=%d", n)
	}
	cur, err := outbox.GetCursor("imessage", dbPath)
	if err != nil || cur == nil || *cur != 42 {
		t.Fatalf("cursor=%v err=%v", cur, err)
	}
	conn, _ := sql.Open("sqlite3", dbPath)
	defer conn.Close()
	var count int
	_ = conn.QueryRow(`SELECT COUNT(*) FROM outbox`).Scan(&count)
	if count != 2 {
		t.Fatalf("count=%d", count)
	}
}

func TestIngestBatchEmptyEventsStillUpdatesCursor(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "outbox.db")
	cv := 10.0
	_, err := outbox.IngestBatch("imessage", nil, &cv, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	cur, err := outbox.GetCursor("imessage", dbPath)
	if err != nil || cur == nil || *cur != 10 {
		t.Fatalf("cursor=%v", cur)
	}
}
