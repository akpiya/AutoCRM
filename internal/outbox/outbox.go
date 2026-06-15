// Package outbox provides SQLite storage for collector events and per-source cursors.
package outbox

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Event is one outbox row to ingest.
type Event struct {
	Platform  string
	PartyID   string
	Direction int
	CreatedAt float64
}

// Row is a pending sync queue row.
type Row struct {
	ID        int64
	Platform  string
	PartyID   string
	Direction int
	CreatedAt float64
}

const schema = `
CREATE TABLE IF NOT EXISTS outbox (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  platform TEXT NOT NULL,
  party_id TEXT NOT NULL,
  direction INTEGER NOT NULL,
  created_at REAL NOT NULL
);

CREATE TABLE IF NOT EXISTS cursor (
  source TEXT PRIMARY KEY,
  cursor_value REAL,
  updated_at REAL NOT NULL
);
`

// InitDB creates tables if missing.
func InitDB(dbPath string) error {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return err
	}
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.Exec(schema)
	return err
}

// GetCursor returns the ingest checkpoint for source, or nil if unset.
func GetCursor(source, dbPath string) (*float64, error) {
	if err := InitDB(dbPath); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	var v sql.NullFloat64
	err = db.QueryRow(
		`SELECT cursor_value FROM cursor WHERE source = ?`, source,
	).Scan(&v)
	if err == sql.ErrNoRows || !v.Valid {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	f := v.Float64
	return &f, nil
}

// IngestBatch inserts events and updates the source cursor in one transaction.
func IngestBatch(source string, events []Event, cursorValue *float64, dbPath string) (int, error) {
	if err := InitDB(dbPath); err != nil {
		return 0, err
	}
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return 0, err
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	for _, e := range events {
		if _, err := tx.Exec(
			`INSERT INTO outbox (platform, party_id, direction, created_at) VALUES (?, ?, ?, ?)`,
			e.Platform, e.PartyID, e.Direction, e.CreatedAt,
		); err != nil {
			return 0, err
		}
	}
	if _, err := tx.Exec(
		`INSERT INTO cursor (source, cursor_value, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT(source) DO UPDATE SET
		   cursor_value = excluded.cursor_value,
		   updated_at = excluded.updated_at`,
		source, cursorValue, time.Now().Unix(),
	); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(events), nil
}

// FetchAll returns all pending outbox rows ordered by id.
func FetchAll(dbPath string) ([]Row, error) {
	if err := InitDB(dbPath); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return queryRows(db, `SELECT id, platform, party_id, direction, created_at FROM outbox ORDER BY id`)
}

// FetchBatch returns up to limit pending rows.
func FetchBatch(limit int, dbPath string) ([]Row, error) {
	if err := InitDB(dbPath); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return queryRows(
		db,
		`SELECT id, platform, party_id, direction, created_at FROM outbox ORDER BY id LIMIT ?`,
		limit,
	)
}

func queryRows(db *sql.DB, q string, args ...any) ([]Row, error) {
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Row
	for rows.Next() {
		var r Row
		if err := rows.Scan(&r.ID, &r.Platform, &r.PartyID, &r.Direction, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// DeleteRows removes processed outbox rows by id.
func DeleteRows(rowIDs []int64, dbPath string) error {
	if len(rowIDs) == 0 {
		return nil
	}
	if err := InitDB(dbPath); err != nil {
		return err
	}
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	placeholders := strings.Repeat("?,", len(rowIDs))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, len(rowIDs))
	for i, id := range rowIDs {
		args[i] = id
	}
	_, err = db.Exec(fmt.Sprintf(`DELETE FROM outbox WHERE id IN (%s)`, placeholders), args...)
	return err
}
