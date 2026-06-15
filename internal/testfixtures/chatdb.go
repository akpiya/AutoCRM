// Package testfixtures builds a minimal chat.db for iMessage collector tests.
package testfixtures

import (
	"database/sql"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

// AppleNS20010102 is 2001-01-02 00:00:00 UTC in Apple nanoseconds.
const AppleNS20010102 = 86_400_000_000_000

// BuildChatDB creates a minimal Messages-style chat.db at path.
func BuildChatDB(path string) error {
	if err := ensureParent(path); err != nil {
		return err
	}
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return err
	}
	defer db.Close()

	if _, err := db.Exec(`
CREATE TABLE handle (ROWID INTEGER PRIMARY KEY, id TEXT);
CREATE TABLE chat (
  ROWID INTEGER PRIMARY KEY,
  chat_identifier TEXT,
  guid TEXT
);
CREATE TABLE chat_handle_join (
  chat_id INTEGER,
  handle_id INTEGER
);
CREATE TABLE chat_message_join (
  chat_id INTEGER,
  message_id INTEGER
);
CREATE TABLE message (
  ROWID INTEGER PRIMARY KEY,
  date INTEGER,
  is_from_me INTEGER,
  service TEXT,
  handle_id INTEGER,
  FOREIGN KEY (handle_id) REFERENCES handle(ROWID)
);
`); err != nil {
		return err
	}

	if _, err := db.Exec(`
INSERT INTO handle (ROWID, id) VALUES
  (1, '+15551234567'),
  (2, 'friend@example.com'),
  (3, '+15559876543');
INSERT INTO chat (ROWID, chat_identifier, guid) VALUES
  (1, 'iMessage;-;+15551234567', 'iMessage;-;+15551234567'),
  (2, 'iMessage;-;friend@example.com', 'iMessage;-;friend@example.com'),
  (3, 'iMessage;+;chat-test-group', 'iMessage;+;chat-test-group');
INSERT INTO chat_handle_join (chat_id, handle_id) VALUES
  (1, 1), (2, 2), (3, 1), (3, 2), (3, 3);
`); err != nil {
		return err
	}

	rows := []struct {
		rowid      int
		date       int64
		isFromMe   int
		service    string
		handleID   interface{}
	}{
		{1, AppleNS20010102, 0, "iMessage", 1},
		{2, AppleNS20010102 + 1_000_000_000, 1, "iMessage", 1},
		{3, AppleNS20010102 + 2_000_000_000, 1, "SMS", 2},
		{4, 0, 1, "iMessage", 1},
		{5, AppleNS20010102 + 3_000_000_000, 1, "iMessage", nil},
		{6, AppleNS20010102 + 4_000_000_000, 0, "SMS", 2},
		{7, AppleNS20010102 + 5_000_000_000, 0, "iMessage", 3},
	}
	for _, r := range rows {
		if _, err := db.Exec(
			`INSERT INTO message (ROWID, date, is_from_me, service, handle_id) VALUES (?, ?, ?, ?, ?)`,
			r.rowid, r.date, r.isFromMe, r.service, r.handleID,
		); err != nil {
			return err
		}
	}

	_, err = db.Exec(`
INSERT INTO chat_message_join (chat_id, message_id) VALUES
  (1, 1), (1, 2), (1, 4), (2, 3), (2, 6), (3, 5), (3, 7);
`)
	return err
}

func ensureParent(path string) error {
	return os.MkdirAll(filepath.Dir(path), 0o755)
}
