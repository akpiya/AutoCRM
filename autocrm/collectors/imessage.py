"""iMessage/SMS outbound collector → outbox."""

from __future__ import annotations

import sqlite3
from pathlib import Path

from autocrm import outbox
from autocrm.collectors.base import Collector, CollectResult
from autocrm.common import (
    default_chat_db,
    imessage_cursor_path,
    load_json_state,
    nanoseconds_to_iso_utc,
    outbox_db,
    save_json_state,
)
from autocrm.models import ContactEvent, PersonHints


class IMessageCollector(Collector):
    app = "imessage"
    imessage_db_path: Path = default_chat_db()
    outbox_db_path: Path = outbox_db()
    state_path: Path = imessage_cursor_path()

    def collect(self) -> CollectResult:
        db_path = self.imessage_db_path
        if not db_path.exists():
            raise SystemExit(f"chat.db not found: {db_path}")

        ob = self.outbox_db_path
        outbox.init_db(ob)

        state = load_json_state(self.state_path, default={"last_message_rowid": 0})
        last_row = int(state.get("last_message_rowid") or 0)

        uri = f"file:{db_path}?mode=ro"
        conn = sqlite3.connect(uri, uri=True)
        conn.row_factory = sqlite3.Row
        try:
            q = """
            SELECT m.ROWID AS rowid, m.date AS mdate, m.is_from_me, m.service,
                   h.id AS handle_id
            FROM message m
            LEFT JOIN handle h ON m.handle_id = h.ROWID
            WHERE m.ROWID > ? AND m.is_from_me = 1
            ORDER BY m.ROWID ASC
            """
            rows = conn.execute(q, (last_row,)).fetchall()
        finally:
            conn.close()

        max_row = last_row
        enq = 0
        for row in rows:
            rid = int(row["rowid"])
            max_row = max(max_row, rid)
            iso = nanoseconds_to_iso_utc(row["mdate"])
            if not iso:
                continue

            handle = (row["handle_id"] or "").strip()
            phones: list[str] = []
            emails: list[str] = []
            handles: list[str] = []
            if handle:
                if "@" in handle:
                    emails.append(handle)
                else:
                    phones.append(handle)
                handles.append(handle)

            chat_id = _lookup_chat_id(db_path, rid)
            hints = PersonHints(phones=phones, emails=emails, handles=handles)
            ev = ContactEvent(
                source="imessage",
                channel=_service_to_channel(row["service"]),
                person_hints=hints,
                occurred_at=iso,
                dedupe_id=f"imessage:{rid}:{chat_id or handle or 'unknown'}",
                direction="outbound",
            )
            payload = ev.to_json_bytes().decode("utf-8")
            ins = outbox.enqueue(ob, payload, ev.dedupe_id)
            if ins is not None:
                enq += 1

        if max_row > last_row:
            save_json_state(self.state_path, {"last_message_rowid": max_row})

        return CollectResult(
            source="imessage",
            enqueued=enq,
            cursor_before=last_row,
            cursor_after=max_row,
        )


def _service_to_channel(service: str | None) -> str:
    if not service:
        return "imessage"
    s = service.lower()
    if "sms" in s:
        return "sms"
    return "imessage"


def _lookup_chat_id(chat_db: Path, message_rowid: int) -> str | None:
    uri = f"file:{chat_db}?mode=ro"
    conn = sqlite3.connect(uri, uri=True)
    try:
        cur = conn.execute(
            "SELECT chat_id FROM chat_message_join WHERE message_id = ? ORDER BY chat_id LIMIT 1",
            (message_rowid,),
        )
        r = cur.fetchone()
        if r:
            return str(r[0])
    except sqlite3.OperationalError:
        return None
    finally:
        conn.close()
    return None

"""Read outbound iMessage/SMS from macOS chat.db → outbox."""

from __future__ import annotations

import sqlite3
from pathlib import Path

from autocrm import outbox
from autocrm.collectors.base import Collector, CollectResult
from autocrm.common import imessage_cursor_path, nanoseconds_to_iso_utc, outbox_db, load_json_state, save_json_state
from autocrm.models import ContactEvent, PersonHints


class IMessageCollector(Collector):
    app = "imessage"

    def collect(self) -> CollectResult:
        return collect()


def _service_to_channel(service: str | None) -> str:
    if not service:
        return "imessage"
    s = service.lower()
    if "sms" in s:
        return "sms"
    return "imessage"


def collect(
    *,
    chat_db: Path | None = None,
    outbox_path: Path | None = None,
    state_path: Path | None = None,
) -> CollectResult:
    """
    Scan new outbound messages since cursor; enqueue to outbox.
    """
    db_path = chat_db or Path.home() / "Library" / "Messages" / "chat.db"
    if not db_path.exists():
        raise SystemExit(f"chat.db not found: {db_path}")

    cursor_path = state_path or imessage_cursor_path()
    state = load_json_state(cursor_path, default={"last_message_rowid": 0})
    last_row = int(state.get("last_message_rowid") or 0)
    ob = outbox_path or outbox_db()
    outbox.init_db(ob)

    uri = f"file:{db_path}?mode=ro"
    conn = sqlite3.connect(uri, uri=True)
    conn.row_factory = sqlite3.Row
    try:
        q = """
        SELECT m.ROWID AS rowid, m.date AS mdate, m.is_from_me, m.service,
               h.id AS handle_id
        FROM message m
        LEFT JOIN handle h ON m.handle_id = h.ROWID
        WHERE m.ROWID > ? AND m.is_from_me = 1
        ORDER BY m.ROWID ASC
        """
        rows = conn.execute(q, (last_row,)).fetchall()
    finally:
        conn.close()

    max_row = last_row
    enq = 0
    for row in rows:
        rid = int(row["rowid"])
        max_row = max(max_row, rid)
        iso = nanoseconds_to_iso_utc(row["mdate"])
        if not iso:
            continue
        handle = (row["handle_id"] or "").strip()
        phones: list[str] = []
        emails: list[str] = []
        handles: list[str] = []
        if handle:
            if "@" in handle:
                emails.append(handle)
            else:
                phones.append(handle)
            handles.append(handle)
        chat_id = _lookup_chat_id(db_path, rid)
        hints = PersonHints(phones=phones, emails=emails, handles=handles)
        ev = ContactEvent(
            source="imessage",
            channel=_service_to_channel(row["service"]),
            person_hints=hints,
            occurred_at=iso,
            dedupe_id=f"imessage:{rid}:{chat_id or handle or 'unknown'}",
            direction="outbound",
        )
        payload = ev.to_json_bytes().decode("utf-8")
        ins = outbox.enqueue(ob, payload, ev.dedupe_id)
        if ins is not None:
            enq += 1

    if max_row > last_row:
        save_json_state(cursor_path, {"last_message_rowid": max_row})

    return CollectResult(
        source="imessage",
        enqueued=enq,
        cursor_before=last_row,
        cursor_after=max_row,
    )


def _lookup_chat_id(chat_db: Path, message_rowid: int) -> str | None:
    uri = f"file:{chat_db}?mode=ro"
    conn = sqlite3.connect(uri, uri=True)
    try:
        cur = conn.execute(
            "SELECT chat_id FROM chat_message_join WHERE message_id = ? ORDER BY chat_id LIMIT 1",
            (message_rowid,),
        )
        r = cur.fetchone()
        if r:
            return str(r[0])
    except sqlite3.OperationalError:
        return None
    finally:
        conn.close()
    return None

