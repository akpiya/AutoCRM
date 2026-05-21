"""iMessage/SMS collector → outbox."""

from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path

from autocrm import outbox
from autocrm.common import (
    DEFAULT_CHAT_DB_PATH,
    DIRECTION_OUTBOUND,
    OUTBOX_DB_PATH,
    PLATFORM_TEXT,
    apple_ns_to_unix,
)
from autocrm.collectors.collector import Collector, CollectResult
from autocrm.collectors.imessage_db import fetch_outbound_messages, max_message_rowid


@dataclass
class IMessageCollector(Collector):
    app: str = "imessage"
    chat_db_path: Path = DEFAULT_CHAT_DB_PATH
    outbox_db_path: Path = OUTBOX_DB_PATH

    def collect(self) -> CollectResult:
        if not self.chat_db_path.exists():
            raise FileNotFoundError(f"chat.db not found: {self.chat_db_path}")

        cursor_before = outbox.get_cursor(self.app, db_path=self.outbox_db_path)
        if cursor_before is None:
            max_row = max_message_rowid(self.chat_db_path)
            outbox.ingest_outbox_batch(
                self.app, [], float(max_row), db_path=self.outbox_db_path
            )
            return CollectResult(
                source=self.app,
                enqueued=0,
                cursor_before=None,
                cursor_after=max_row,
            )

        last_row = int(cursor_before)
        rows = fetch_outbound_messages(self.chat_db_path, last_row)
        max_row = last_row
        events: list[outbox.EventTuple] = []

        for row in rows:
            max_row = max(max_row, row.rowid)
            if not row.handle_id:
                continue
            created_at = apple_ns_to_unix(row.mdate)
            if created_at is None:
                continue
            events.append(
                (PLATFORM_TEXT, row.handle_id, DIRECTION_OUTBOUND, created_at)
            )

        enqueued = 0
        if max_row > last_row or events:
            enqueued = outbox.ingest_outbox_batch(
                self.app,
                events,
                float(max_row),
                db_path=self.outbox_db_path,
            )

        return CollectResult(
            source=self.app,
            enqueued=enqueued,
            cursor_before=last_row,
            cursor_after=max_row,
        )
