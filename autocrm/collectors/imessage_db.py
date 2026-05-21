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


def _member_handles_for_chats(
    conn: sqlite3.Connection, chat_ids: set[int]
) -> dict[int, tuple[str, ...]]:
    if not chat_ids:
        return {}
    placeholders = ",".join("?" * len(chat_ids))
    rows = conn.execute(
        f"""
        SELECT chj.chat_id, h.id
        FROM chat_handle_join chj
        JOIN handle h ON h.ROWID = chj.handle_id
        WHERE chj.chat_id IN ({placeholders})
        ORDER BY chj.chat_id, h.id
        """,
        list(chat_ids),
    ).fetchall()
    buckets: dict[int, list[str]] = {}
    seen: dict[int, set[str]] = {}
    for chat_id, raw in rows:
        handle = (raw or "").strip()
        if not handle:
            continue
        cid = int(chat_id)
        if handle in seen.setdefault(cid, set()):
            continue
        seen[cid].add(handle)
        buckets.setdefault(cid, []).append(handle)
    return {cid: tuple(handles) for cid, handles in buckets.items()}


def fetch_messages_after_rowid(chat_db: Path, after_rowid: int) -> list[MessageRow]:
    uri = f"file:{chat_db}?mode=ro"
    conn = sqlite3.connect(uri, uri=True)
    conn.row_factory = sqlite3.Row
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

        group_chat_ids = {
            int(r["chat_id"]) for r in rows if bool(r["is_group"])
        }
        member_cache = _member_handles_for_chats(conn, group_chat_ids)

        result: list[MessageRow] = []
        for r in rows:
            chat_id = int(r["chat_id"])
            is_group = bool(r["is_group"])
            members = member_cache.get(chat_id, ()) if is_group else ()
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
