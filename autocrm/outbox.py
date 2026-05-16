"""SQLite outbox for collector events and per-source cursors."""

from __future__ import annotations

import sqlite3
import time
from contextlib import contextmanager
from pathlib import Path
from typing import Iterator

from autocrm.common import OUTBOX_DB_PATH

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


@contextmanager
def _connect(db_path: Path, *, write: bool = False) -> Iterator[sqlite3.Connection]:
    init_db(db_path)
    conn = sqlite3.connect(db_path)
    try:
        yield conn
        if write:
            conn.commit()
    finally:
        conn.close()


def add_row(
    platform: str,
    party_id: str,
    direction: int,
    timestamp: float,
    *,
    db_path: Path = OUTBOX_DB_PATH,
) -> int:
    with _connect(db_path, write=True) as conn:
        cur = conn.execute(
            "INSERT INTO outbox (platform, party_id, direction, created_at) "
            "VALUES (?, ?, ?, ?)",
            (platform, party_id, direction, timestamp),
        )
        return int(cur.lastrowid)


def get_cursor(source: str, *, db_path: Path = OUTBOX_DB_PATH) -> float | None:
    with _connect(db_path) as conn:
        row = conn.execute(
            "SELECT cursor_value FROM cursor WHERE source = ?",
            (source,),
        ).fetchone()
    if row is None or row[0] is None:
        return None
    return float(row[0])


def set_cursor(
    source: str,
    cursor_value: float | None,
    *,
    db_path: Path = OUTBOX_DB_PATH,
) -> None:
    with _connect(db_path, write=True) as conn:
        conn.execute(
            "INSERT INTO cursor (source, cursor_value, updated_at) "
            "VALUES (?, ?, ?) "
            "ON CONFLICT(source) DO UPDATE SET "
            "cursor_value = excluded.cursor_value, "
            "updated_at = excluded.updated_at",
            (source, cursor_value, time.time()),
        )


def delete_row(row_id: int, *, db_path: Path = OUTBOX_DB_PATH) -> None:
    with _connect(db_path, write=True) as conn:
        conn.execute("DELETE FROM outbox WHERE id = ?", (row_id,))
