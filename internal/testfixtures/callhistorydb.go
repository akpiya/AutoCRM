// Package testfixtures builds minimal fixture DBs for collector tests.
package testfixtures

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

// AppleSec20010102 is 2001-01-02 00:00:00 UTC in Apple absolute seconds.
const AppleSec20010102 = 86_400

// BuildCallHistoryDB creates a minimal CallHistory.storedata-style DB at path.
func BuildCallHistoryDB(path string) error {
	if err := ensureParent(path); err != nil {
		return err
	}
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return err
	}
	defer db.Close()

	if _, err := db.Exec(`
CREATE TABLE ZCALLRECORD (
  Z_PK INTEGER PRIMARY KEY,
  ZDATE REAL,
  ZORIGINATED INTEGER,
  ZANSWERED INTEGER,
  ZDURATION REAL,
  ZCALLTYPE INTEGER,
  ZADDRESS TEXT
);
`); err != nil {
		return err
	}

	rows := []struct {
		rowid      int
		date       float64
		originated int
		answered   int
		duration   float64
		callType   int
		address    any
	}{
		{1, AppleSec20010102 + 1, 0, 1, 120, 1, "+15551234567"},
		{2, AppleSec20010102 + 2, 1, 1, 40, 1, "+15559876543"},
		{3, AppleSec20010102 + 3, 0, 0, 0, 1, "+15550000000"},
		{4, AppleSec20010102 + 4, 1, 0, 0, 1, "+15550000001"},
		{5, AppleSec20010102 + 5, 0, 1, 15, 8, "friend@example.com"},
		{6, AppleSec20010102 + 6, 1, 1, 25, 16, "+15557654321"},
		{7, AppleSec20010102 + 7, 0, 1, 60, 1, ""},
	}
	for _, r := range rows {
		if _, err := db.Exec(
			`INSERT INTO ZCALLRECORD (Z_PK, ZDATE, ZORIGINATED, ZANSWERED, ZDURATION, ZCALLTYPE, ZADDRESS) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			r.rowid, r.date, r.originated, r.answered, r.duration, r.callType, r.address,
		); err != nil {
			return err
		}
	}
	return nil
}
