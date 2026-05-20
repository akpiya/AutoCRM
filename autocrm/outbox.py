"""SQLite outbox for collector events and per-source cursors."""

from __future__ import annotations

import sqlite3
import time
from pathlib import Path
from typing import Sequence

from autocrm.common import OUTBOX_DB_PATH

EventTuple = tuple[str, str, int, float]

_SCHEMA = """
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
"""


def init_db(db_path: Path = OUTBOX_DB_PATH) -> None:
    db_path.parent.mkdir(parents=True, exist_ok=True)
    conn = sqlite3.connect(db_path)
    try:
        conn.executescript(_SCHEMA)
        conn.commit()
    finally:
        conn.close()


def get_cursor(source: str, *, db_path: Path = OUTBOX_DB_PATH) -> float | None:
    init_db(db_path)
    conn = sqlite3.connect(db_path)
    try:
        row = conn.execute(
            "SELECT cursor_value FROM cursor WHERE source = ?",
            (source,),
        ).fetchone()
    finally:
        conn.close()
    if row is None or row[0] is None:
        return None
    return float(row[0])


def ingest_outbox_batch(
    source: str,
    events: Sequence[EventTuple],
    cursor_value: float | None,
    *,
    db_path: Path = OUTBOX_DB_PATH,
) -> int:
    """Insert outbox rows and update the source cursor in one transaction."""
    init_db(db_path)
    conn = sqlite3.connect(db_path)
    try:
        for platform, party_id, direction, created_at in events:
            conn.execute(
                "INSERT INTO outbox (platform, party_id, direction, created_at) "
                "VALUES (?, ?, ?, ?)",
                (platform, party_id, direction, created_at),
            )
        conn.execute(
            "INSERT INTO cursor (source, cursor_value, updated_at) "
            "VALUES (?, ?, ?) "
            "ON CONFLICT(source) DO UPDATE SET "
            "cursor_value = excluded.cursor_value, "
            "updated_at = excluded.updated_at",
            (source, cursor_value, time.time()),
        )
        conn.commit()
        return len(events)
    finally:
        conn.close()
