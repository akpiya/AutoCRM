"""Read outbound rows from macOS Messages chat.db."""

from __future__ import annotations

import sqlite3
from dataclasses import dataclass
from pathlib import Path


@dataclass(frozen=True)
class MessageRow:
    rowid: int
    mdate: int
    handle_id: str | None


def max_message_rowid(chat_db: Path) -> int:
    uri = f"file:{chat_db}?mode=ro"
    conn = sqlite3.connect(uri, uri=True)
    try:
        row = conn.execute("SELECT MAX(ROWID) FROM message").fetchone()
    finally:
        conn.close()
    if row is None or row[0] is None:
        return 0
    return int(row[0])


def fetch_outbound_messages(chat_db: Path, after_rowid: int) -> list[MessageRow]:
    uri = f"file:{chat_db}?mode=ro"
    conn = sqlite3.connect(uri, uri=True)
    conn.row_factory = sqlite3.Row
    try:
        rows = conn.execute(
            """
            SELECT m.ROWID AS rowid, m.date AS mdate, h.id AS handle_id
            FROM message m
            LEFT JOIN handle h ON m.handle_id = h.ROWID
            WHERE m.ROWID > ? AND m.is_from_me = 1
            ORDER BY m.ROWID ASC
            """,
            (after_rowid,),
        ).fetchall()
    finally:
        conn.close()

    return [
        MessageRow(
            rowid=int(r["rowid"]),
            mdate=int(r["mdate"]),
            handle_id=(r["handle_id"] or "").strip() or None,
        )
        for r in rows
    ]
