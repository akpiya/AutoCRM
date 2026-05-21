"""Read messages from macOS Messages chat.db (1:1 and group)."""

from __future__ import annotations

import sqlite3
from dataclasses import dataclass
from pathlib import Path


@dataclass(frozen=True)
class MessageRow:
    rowid: int
    mdate: int
    is_from_me: bool
    sender_handle: str | None
    is_group: bool
    member_handles: tuple[str, ...]


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


def _fetch_member_handles(conn: sqlite3.Connection, chat_id: int) -> tuple[str, ...]:
    rows = conn.execute(
        """
        SELECT h.id
        FROM chat_handle_join chj
        JOIN handle h ON h.ROWID = chj.handle_id
        WHERE chj.chat_id = ?
        ORDER BY h.id
        """,
        (chat_id,),
    ).fetchall()
    seen: set[str] = set()
    out: list[str] = []
    for (raw,) in rows:
        handle = (raw or "").strip()
        if handle and handle not in seen:
            seen.add(handle)
            out.append(handle)
    return tuple(out)


def fetch_messages_after_rowid(chat_db: Path, after_rowid: int) -> list[MessageRow]:
    uri = f"file:{chat_db}?mode=ro"
    conn = sqlite3.connect(uri, uri=True)
    conn.row_factory = sqlite3.Row
    member_cache: dict[int, tuple[str, ...]] = {}
    try:
        rows = conn.execute(
            """
            SELECT
              m.ROWID AS rowid,
              m.date AS mdate,
              m.is_from_me AS is_from_me,
              h.id AS sender_handle,
              c.ROWID AS chat_id,
              CASE
                WHEN c.chat_identifier LIKE '%;+;%'
                  OR IFNULL(c.guid, '') LIKE '%;+;%'
                THEN 1
                ELSE 0
              END AS is_group
            FROM message m
            INNER JOIN chat_message_join cmj ON cmj.message_id = m.ROWID
            INNER JOIN chat c ON c.ROWID = cmj.chat_id
            LEFT JOIN handle h ON m.handle_id = h.ROWID
            WHERE m.ROWID > ?
            ORDER BY m.ROWID ASC
            """,
            (after_rowid,),
        ).fetchall()

        result: list[MessageRow] = []
        for r in rows:
            chat_id = int(r["chat_id"])
            is_group = bool(r["is_group"])
            members: tuple[str, ...] = ()
            if is_group:
                if chat_id not in member_cache:
                    member_cache[chat_id] = _fetch_member_handles(conn, chat_id)
                members = member_cache[chat_id]
            result.append(
                MessageRow(
                    rowid=int(r["rowid"]),
                    mdate=int(r["mdate"]),
                    is_from_me=bool(r["is_from_me"]),
                    sender_handle=(r["sender_handle"] or "").strip() or None,
                    is_group=is_group,
                    member_handles=members,
                )
            )
        return result
    finally:
        conn.close()
