"""iMessage/SMS collector → outbox."""

from __future__ import annotations

import logging
import time
from dataclasses import dataclass
from pathlib import Path

from autocrm import outbox
from autocrm.common import (
    DEFAULT_CHAT_DB_PATH,
    DIRECTION_INBOUND,
    DIRECTION_OUTBOUND,
    OUTBOX_DB_PATH,
    PLATFORM_TEXT,
    apple_ns_to_unix,
)
from autocrm.collectors.collector import Collector, CollectResult
from autocrm.collectors.imessage_db import (
    MessageRow,
    fetch_messages_after_rowid,
    max_message_rowid,
)

LOG = logging.getLogger(__name__)


def _events_for_message(row: MessageRow) -> list[outbox.EventTuple]:
    created_at = apple_ns_to_unix(row.mdate)
    if created_at is None:
        return []

    if row.is_group:
        if row.is_from_me:
            return [
                (PLATFORM_TEXT, handle, DIRECTION_OUTBOUND, created_at)
                for handle in row.member_handles
            ]
        if row.sender_handle:
            return [
                (PLATFORM_TEXT, row.sender_handle, DIRECTION_INBOUND, created_at),
            ]
        return []

    if not row.sender_handle:
        return []
    direction = DIRECTION_OUTBOUND if row.is_from_me else DIRECTION_INBOUND
    return [(PLATFORM_TEXT, row.sender_handle, direction, created_at)]


@dataclass
class IMessageCollector(Collector):
    app: str = "imessage"
    chat_db_path: Path = DEFAULT_CHAT_DB_PATH
    outbox_db_path: Path = OUTBOX_DB_PATH

    def collect(self) -> CollectResult:
        if not self.chat_db_path.exists():
            raise FileNotFoundError(f"chat.db not found: {self.chat_db_path}")

        t0 = time.perf_counter()
        cursor_before = outbox.get_cursor(self.app, db_path=self.outbox_db_path)
        if cursor_before is None:
            max_row = max_message_rowid(self.chat_db_path)
            outbox.ingest_outbox_batch(
                self.app, [], float(max_row), db_path=self.outbox_db_path
            )
            LOG.info(
                "imessage bootstrap cursor=%s in %.2fs",
                max_row,
                time.perf_counter() - t0,
            )
            return CollectResult(
                source=self.app,
                enqueued=0,
                cursor_before=None,
                cursor_after=max_row,
            )

        last_row = int(cursor_before)
        t_fetch = time.perf_counter()
        rows = fetch_messages_after_rowid(self.chat_db_path, last_row)
        fetch_s = time.perf_counter() - t_fetch
        max_row = last_row
        events: list[outbox.EventTuple] = []

        t_events = time.perf_counter()
        for row in rows:
            max_row = max(max_row, row.rowid)
            events.extend(_events_for_message(row))
        events_s = time.perf_counter() - t_events

        enqueued = 0
        ingest_s = 0.0
        if max_row > last_row or events:
            t_ingest = time.perf_counter()
            enqueued = outbox.ingest_outbox_batch(
                self.app,
                events,
                float(max_row),
                db_path=self.outbox_db_path,
            )
            ingest_s = time.perf_counter() - t_ingest

        LOG.info(
            "imessage ingest: %.2fs total (fetch=%.2fs %d msgs, events=%.2fs %d rows, "
            "outbox_write=%.2fs)",
            time.perf_counter() - t0,
            fetch_s,
            len(rows),
            events_s,
            len(events),
            ingest_s,
        )

        return CollectResult(
            source=self.app,
            enqueued=enqueued,
            cursor_before=last_row,
            cursor_after=max_row,
        )
