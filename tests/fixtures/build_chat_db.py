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
            """
        )
        conn.executemany(
            "INSERT INTO handle (ROWID, id) VALUES (?, ?)",
            [
                (1, "+15551234567"),
                (2, "friend@example.com"),
                (3, "+15559876543"),
            ],
        )
        conn.executemany(
            "INSERT INTO chat (ROWID, chat_identifier, guid) VALUES (?, ?, ?)",
            [
                (1, "iMessage;-;+15551234567", "iMessage;-;+15551234567"),
                (2, "iMessage;-;friend@example.com", "iMessage;-;friend@example.com"),
                (3, "iMessage;+;chat-test-group", "iMessage;+;chat-test-group"),
            ],
        )
        conn.executemany(
            "INSERT INTO chat_handle_join (chat_id, handle_id) VALUES (?, ?)",
            [
                (1, 1),
                (2, 2),
                (3, 1),
                (3, 2),
                (3, 3),
            ],
        )
        rows = [
            (1, APPLE_NS_2001_01_02, 0, "iMessage", 1),
            (2, APPLE_NS_2001_01_02 + 1_000_000_000, 1, "iMessage", 1),
            (3, APPLE_NS_2001_01_02 + 2_000_000_000, 1, "SMS", 2),
            (4, 0, 1, "iMessage", 1),
            (5, APPLE_NS_2001_01_02 + 3_000_000_000, 1, "iMessage", None),
            (6, APPLE_NS_2001_01_02 + 4_000_000_000, 0, "SMS", 2),
            (7, APPLE_NS_2001_01_02 + 5_000_000_000, 0, "iMessage", 3),
        ]
        conn.executemany(
            "INSERT INTO message (ROWID, date, is_from_me, service, handle_id) "
            "VALUES (?, ?, ?, ?, ?)",
            rows,
        )
        conn.executemany(
            "INSERT INTO chat_message_join (chat_id, message_id) VALUES (?, ?)",
            [
                (1, 1),
                (1, 2),
                (1, 4),
                (2, 3),
                (2, 6),
                (3, 5),
                (3, 7),
            ],
        )
        conn.commit()
    finally:
        conn.close()
