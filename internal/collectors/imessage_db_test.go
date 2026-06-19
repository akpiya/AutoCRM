package collectors_test

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/akpiya/autocrm/internal/collectors"
	"github.com/akpiya/autocrm/internal/testfixtures"

	_ "github.com/mattn/go-sqlite3"
)

func TestFetchMessagesAfterRowid(t *testing.T) {
	dir := t.TempDir()
	chatDB := filepath.Join(dir, "chat.db")
	if err := testfixtures.BuildChatDB(chatDB); err != nil {
		t.Fatal(err)
	}

	rows, err := collectors.FetchMessagesAfterRowid(chatDB, 0)
	if err != nil {
		t.Fatal(err)
	}
	rowids := make([]int, len(rows))
	for i, r := range rows {
		rowids[i] = r.RowID
	}
	want := []int{1, 2, 3, 4, 5, 6, 7}
	if len(rowids) != len(want) {
		t.Fatalf("rowids=%v", rowids)
	}
	for i, w := range want {
		if rowids[i] != w {
			t.Fatalf("rowids=%v", rowids)
		}
	}

	directIn := rows[0]
	if directIn.IsGroup || directIn.IsFromMe || directIn.SenderHandle != "+15551234567" {
		t.Fatalf("directIn=%+v", directIn)
	}

	groupOut := rows[4]
	if !groupOut.IsGroup || !groupOut.IsFromMe || groupOut.SenderHandle != "" {
		t.Fatalf("groupOut=%+v", groupOut)
	}
	wantMembers := []string{"+15551234567", "+15559876543", "friend@example.com"}
	if len(groupOut.MemberHandles) != len(wantMembers) {
		t.Fatalf("members=%v", groupOut.MemberHandles)
	}
	for i, m := range wantMembers {
		if groupOut.MemberHandles[i] != m {
			t.Fatalf("members=%v", groupOut.MemberHandles)
		}
	}

	groupIn := rows[6]
	if !groupIn.IsGroup || groupIn.IsFromMe || groupIn.SenderHandle != "+15559876543" {
		t.Fatalf("groupIn=%+v", groupIn)
	}

	after2, err := collectors.FetchMessagesAfterRowid(chatDB, 2)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]int, len(after2))
	for i, r := range after2 {
		got[i] = r.RowID
	}
	if len(got) != 5 || got[0] != 3 {
		t.Fatalf("after2=%v", got)
	}
}

func TestMaxMessageRowidBeforeAppleNS(t *testing.T) {
	dir := t.TempDir()
	chatDB := filepath.Join(dir, "chat.db")
	if err := testfixtures.BuildChatDB(chatDB); err != nil {
		t.Fatal(err)
	}

	rowid, err := collectors.MaxMessageRowidBeforeAppleNS(chatDB, testfixtures.AppleNS20010102+2500_000_000)
	if err != nil {
		t.Fatal(err)
	}
	if rowid != 3 {
		t.Fatalf("rowid=%d", rowid)
	}

	rowid, err = collectors.MaxMessageRowidBeforeAppleNS(chatDB, testfixtures.AppleNS20010102)
	if err != nil {
		t.Fatal(err)
	}
	if rowid != 0 {
		t.Fatalf("rowid=%d", rowid)
	}
}

func TestFetchMessagesSkipsOrphanWithoutChatJoin(t *testing.T) {
	dir := t.TempDir()
	chatDB := filepath.Join(dir, "chat.db")
	if err := testfixtures.BuildChatDB(chatDB); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", chatDB)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(
		`INSERT INTO message (ROWID, date, is_from_me, service, handle_id) VALUES (8, ?, 1, 'iMessage', 1)`,
		testfixtures.AppleNS20010102+6_000_000_000,
	)
	db.Close()
	if err != nil {
		t.Fatal(err)
	}

	rows, err := collectors.FetchMessagesAfterRowid(chatDB, 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("rows=%v", rows)
	}
}
