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

func testPhoneCollector(t *testing.T) *collectors.PhoneCallsCollector {
	t.Helper()
	dir := t.TempDir()
	callDB := filepath.Join(dir, "CallHistory.storedata")
	if err := testfixtures.BuildCallHistoryDB(callDB); err != nil {
		t.Fatal(err)
	}
	outboxDB := filepath.Join(dir, "outbox.db")
	if err := outbox.InitDB(outboxDB); err != nil {
		t.Fatal(err)
	}
	z := 0.0
	if _, err := outbox.IngestBatch("phone_calls", nil, &z, outboxDB); err != nil {
		t.Fatal(err)
	}
	return &collectors.PhoneCallsCollector{
		AppName:      "phone_calls",
		CallDBPath:   callDB,
		OutboxDBPath: outboxDB,
	}
}

func phoneOutboxRows(t *testing.T, dbPath string) [][4]any {
	t.Helper()
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := db.Query(`SELECT platform, party_id, direction, created_at FROM outbox ORDER BY id`)
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

func TestPhoneCollectEnqueuesAndAdvancesCursor(t *testing.T) {
	c := testPhoneCollector(t)
	result, err := c.Collect()
	if err != nil {
		t.Fatal(err)
	}
	if result.Enqueued != 4 {
		t.Fatalf("enqueued=%d", result.Enqueued)
	}
	if result.CursorBefore == nil || *result.CursorBefore != 0 {
		t.Fatalf("before=%v", result.CursorBefore)
	}
	if result.CursorAfter == nil || *result.CursorAfter != 7 {
		t.Fatalf("after=%v", result.CursorAfter)
	}

	rows := phoneOutboxRows(t, c.OutboxDBPath)
	if len(rows) != 4 {
		t.Fatalf("rows=%d", len(rows))
	}
	parties := map[string]struct{}{}
	directions := map[int]struct{}{}
	platforms := map[string]struct{}{}
	for _, r := range rows {
		platforms[r[0].(string)] = struct{}{}
		parties[r[1].(string)] = struct{}{}
		directions[r[2].(int)] = struct{}{}
	}
	wantParties := map[string]struct{}{
		"+15551234567": {}, "+15559876543": {}, "friend@example.com": {}, "+15557654321": {},
	}
	if len(parties) != len(wantParties) {
		t.Fatalf("parties=%v", parties)
	}
	if len(directions) != 2 {
		t.Fatalf("directions=%v", directions)
	}
	wantPlatforms := map[string]struct{}{
		common.PlatformPhoneCall:     {},
		common.PlatformFaceTimeVideo: {},
		common.PlatformFaceTimeAudio: {},
	}
	for p := range wantPlatforms {
		if _, ok := platforms[p]; !ok {
			t.Fatalf("platforms=%v", platforms)
		}
	}
	cur, err := outbox.GetCursor("phone_calls", c.OutboxDBPath)
	if err != nil || cur == nil || *cur != 7 {
		t.Fatalf("cursor=%v err=%v", cur, err)
	}
}

func TestPhoneCollectSecondRunEmpty(t *testing.T) {
	c := testPhoneCollector(t)
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
	if len(phoneOutboxRows(t, c.OutboxDBPath)) != 4 {
		t.Fatal("expected 4 rows")
	}
}

func TestPhoneMissingCallDBRaises(t *testing.T) {
	dir := t.TempDir()
	c := &collectors.PhoneCallsCollector{
		CallDBPath:   filepath.Join(dir, "missing.storedata"),
		OutboxDBPath: filepath.Join(dir, "outbox.db"),
	}
	_, err := c.Collect()
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, os.ErrNotExist) && err.Error() == "" {
		if _, statErr := os.Stat(c.CallDBPath); !os.IsNotExist(statErr) {
			t.Fatalf("err=%v", err)
		}
	}
}

func TestPhoneMissingCursorSkipsIngestAndSetsMaxRowid(t *testing.T) {
	dir := t.TempDir()
	callDB := filepath.Join(dir, "CallHistory.storedata")
	if err := testfixtures.BuildCallHistoryDB(callDB); err != nil {
		t.Fatal(err)
	}
	outboxDB := filepath.Join(dir, "outbox.db")
	c := &collectors.PhoneCallsCollector{
		CallDBPath:   callDB,
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
	cur, _ := outbox.GetCursor("phone_calls", outboxDB)
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
