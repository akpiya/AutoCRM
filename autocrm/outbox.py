"""SQLite outbox for offline / retry."""

from __future__ import annotations

import sqlite3
import time
from pathlib import Path


def init_db(db_path: Path) -> None:
    db_path.parent.mkdir(parents=True, exist_ok=True)
    conn = sqlite3.connect(db_path)
    try:
        conn.execute(
            """
            CREATE TABLE IF NOT EXISTS outbox (
              id INTEGER PRIMARY KEY AUTOINCREMENT,
              payload_json TEXT NOT NULL,
              created_at REAL NOT NULL,
              status TEXT NOT NULL DEFAULT 'pending',
              attempts INTEGER NOT NULL DEFAULT 0,
              last_error TEXT,
              dedupe_id TEXT UNIQUE
            )
            """
        )
        conn.execute(
            "CREATE INDEX IF NOT EXISTS idx_outbox_status ON outbox(status, id)"
        )
        conn.commit()
    finally:
        conn.close()


def enqueue(
    db_path: Path,
    payload_json: str,
    dedupe_id: str | None,
) -> int | None:
    """Insert row. Returns row id, or None if dedupe_id conflict."""
    init_db(db_path)
    conn = sqlite3.connect(db_path)
    try:
        now = time.time()
        try:
            cur = conn.execute(
                "INSERT INTO outbox (payload_json, created_at, status, dedupe_id) VALUES (?, ?, 'pending', ?)",
                (payload_json, now, dedupe_id),
            )
            conn.commit()
            return int(cur.lastrowid)
        except sqlite3.IntegrityError:
            conn.rollback()
            return None
    finally:
        conn.close()


def fetch_pending_batch(db_path: Path, limit: int = 10) -> list[tuple[int, str, str | None]]:
    init_db(db_path)
    conn = sqlite3.connect(db_path)
    try:
        cur = conn.execute(
            "SELECT id, payload_json, dedupe_id FROM outbox WHERE status = 'pending' ORDER BY id LIMIT ?",
            (limit,),
        )
        return [(int(r[0]), str(r[1]), r[2]) for r in cur.fetchall()]
    finally:
        conn.close()


def mark_sent(db_path: Path, row_id: int) -> None:
    conn = sqlite3.connect(db_path)
    try:
        conn.execute("UPDATE outbox SET status = 'sent' WHERE id = ?", (row_id,))
        conn.commit()
    finally:
        conn.close()


def mark_failed(db_path: Path, row_id: int, err: str) -> None:
    conn = sqlite3.connect(db_path)
    try:
        conn.execute(
            "UPDATE outbox SET status = 'failed', attempts = attempts + 1, last_error = ? WHERE id = ?",
            (err[:2000], row_id),
        )
        conn.commit()
    finally:
        conn.close()


def delete_row(db_path: Path, row_id: int) -> None:
    conn = sqlite3.connect(db_path)
    try:
        conn.execute("DELETE FROM outbox WHERE id = ?", (row_id,))
        conn.commit()
    finally:
        conn.close()


def pending_count(db_path: Path) -> int:
    init_db(db_path)
    conn = sqlite3.connect(db_path)
    try:
        cur = conn.execute(
            "SELECT COUNT(*) FROM outbox WHERE status = 'pending'"
        )
        return int(cur.fetchone()[0])
    finally:
        conn.close()
