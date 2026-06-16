package collectors_test

import (
	"path/filepath"
	"testing"

	"github.com/akpiya/autocrm/internal/collectors"
	"github.com/akpiya/autocrm/internal/testfixtures"
)

func TestFetchCallsAfterRowid(t *testing.T) {
	dir := t.TempDir()
	callDB := filepath.Join(dir, "CallHistory.storedata")
	if err := testfixtures.BuildCallHistoryDB(callDB); err != nil {
		t.Fatal(err)
	}

	rows, err := collectors.FetchCallsAfterRowid(callDB, 0)
	if err != nil {
		t.Fatal(err)
	}
	rowids := make([]int, len(rows))
	for i, r := range rows {
		rowids[i] = r.RowID
	}
	want := []int{1, 2, 5, 6, 7}
	if len(rowids) != len(want) {
		t.Fatalf("rowids=%v", rowids)
	}
	for i, w := range want {
		if rowids[i] != w {
			t.Fatalf("rowids=%v", rowids)
		}
	}

	if rows[0].IsOriginated || rows[0].CallType != 1 || rows[0].Address != "+15551234567" {
		t.Fatalf("row0=%+v", rows[0])
	}
	if !rows[1].IsOriginated || rows[1].CallType != 1 {
		t.Fatalf("row1=%+v", rows[1])
	}
	if rows[2].CallType != 8 || rows[2].Address != "friend@example.com" {
		t.Fatalf("row2=%+v", rows[2])
	}
	if rows[3].CallType != 16 {
		t.Fatalf("row3=%+v", rows[3])
	}

	after2, err := collectors.FetchCallsAfterRowid(callDB, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(after2) != 3 || after2[0].RowID != 5 {
		t.Fatalf("after2=%v", after2)
	}
}

func TestMaxCallRowid(t *testing.T) {
	dir := t.TempDir()
	callDB := filepath.Join(dir, "CallHistory.storedata")
	if err := testfixtures.BuildCallHistoryDB(callDB); err != nil {
		t.Fatal(err)
	}
	max, err := collectors.MaxCallRowid(callDB)
	if err != nil {
		t.Fatal(err)
	}
	if max != 7 {
		t.Fatalf("max=%d", max)
	}
}
