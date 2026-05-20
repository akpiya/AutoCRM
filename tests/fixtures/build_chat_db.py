"""Build a minimal chat.db fixture for iMessage collector tests."""

from __future__ import annotations

import sqlite3
from pathlib import Path

# 2001-01-02 00:00:00 UTC in Apple ns
APPLE_NS_2001_01_02 = 86_400_000_000_000


def build_chat_db(path: Path) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    conn = sqlite3.connect(path)
    try:
        conn.executescript(
            """
            CREATE TABLE handle (ROWID INTEGER PRIMARY KEY, id TEXT);
            CREATE TABLE message (
              ROWID INTEGER PRIMARY KEY,
              date INTEGER,
              is_from_me INTEGER,
              service TEXT,
              handle_id INTEGER,
              FOREIGN KEY (handle_id) REFERENCES handle(ROWID)
            );
            """
        )
        conn.executemany(
            "INSERT INTO handle (ROWID, id) VALUES (?, ?)",
            [(1, "+15551234567"), (2, "friend@example.com")],
        )
        rows = [
            (1, APPLE_NS_2001_01_02, 0, "iMessage", 1),
            (2, APPLE_NS_2001_01_02 + 1_000_000_000, 1, "iMessage", 1),
            (3, APPLE_NS_2001_01_02 + 2_000_000_000, 1, "SMS", 2),
            (4, 0, 1, "iMessage", 1),
            (5, APPLE_NS_2001_01_02 + 3_000_000_000, 1, "iMessage", None),
            (6, APPLE_NS_2001_01_02 + 4_000_000_000, 0, "SMS", 2),
        ]
        conn.executemany(
            "INSERT INTO message (ROWID, date, is_from_me, service, handle_id) "
            "VALUES (?, ?, ?, ?, ?)",
            rows,
        )
        conn.commit()
    finally:
        conn.close()
