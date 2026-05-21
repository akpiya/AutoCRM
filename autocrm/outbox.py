"""SQLite outbox for collector events and per-source cursors."""

from __future__ import annotations

import sqlite3
import time
from dataclasses import dataclass
from pathlib import Path
from typing import Sequence

from autocrm.common import OUTBOX_DB_PATH

EventTuple = tuple[str, str, int, float]


@dataclass(frozen=True)
class OutboxRow:
    id: int
    platform: str
    party_id: str
    direction: int
    created_at: float

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


def fetch_all_outbox_rows(*, db_path: Path = OUTBOX_DB_PATH) -> list[OutboxRow]:
    init_db(db_path)
    conn = sqlite3.connect(db_path)
    try:
        rows = conn.execute(
            "SELECT id, platform, party_id, direction, created_at "
            "FROM outbox ORDER BY id"
        ).fetchall()
    finally:
        conn.close()
    return [
        OutboxRow(
            id=int(r[0]),
            platform=str(r[1]),
            party_id=str(r[2]),
            direction=int(r[3]),
            created_at=float(r[4]),
        )
        for r in rows
    ]


def fetch_outbox_batch(
    limit: int,
    *,
    db_path: Path = OUTBOX_DB_PATH,
) -> list[OutboxRow]:
    init_db(db_path)
    conn = sqlite3.connect(db_path)
    try:
        rows = conn.execute(
            "SELECT id, platform, party_id, direction, created_at "
            "FROM outbox ORDER BY id LIMIT ?",
            (limit,),
        ).fetchall()
    finally:
        conn.close()
    return [
        OutboxRow(
            id=int(r[0]),
            platform=str(r[1]),
            party_id=str(r[2]),
            direction=int(r[3]),
            created_at=float(r[4]),
        )
        for r in rows
    ]


def delete_outbox_rows(
    row_ids: Sequence[int],
    *,
    db_path: Path = OUTBOX_DB_PATH,
) -> None:
    if not row_ids:
        return
    init_db(db_path)
    conn = sqlite3.connect(db_path)
    try:
        placeholders = ",".join("?" * len(row_ids))
        conn.execute(
            f"DELETE FROM outbox WHERE id IN ({placeholders})",
            list(row_ids),
        )
        conn.commit()
    finally:
        conn.close()
