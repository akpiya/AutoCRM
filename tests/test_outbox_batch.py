from __future__ import annotations

from pathlib import Path

from autocrm import outbox
from autocrm.common import DIRECTION_OUTBOUND, PLATFORM_TEXT


def test_ingest_batch_writes_events_and_cursor(outbox_db_path: Path) -> None:
    events = [
        (PLATFORM_TEXT, "+1", DIRECTION_OUTBOUND, 1_700_000_000.0),
        (PLATFORM_TEXT, "a@b.com", DIRECTION_OUTBOUND, 1_700_000_001.0),
    ]
    n = outbox.ingest_outbox_batch("imessage", events, 42.0, db_path=outbox_db_path)

    assert n == 2
    assert outbox.get_cursor("imessage", db_path=outbox_db_path) == 42.0

    import sqlite3

    conn = sqlite3.connect(outbox_db_path)
    try:
        count = conn.execute("SELECT COUNT(*) FROM outbox").fetchone()[0]
    finally:
        conn.close()
    assert count == 2


def test_ingest_batch_empty_events_still_updates_cursor(outbox_db_path: Path) -> None:
    outbox.ingest_outbox_batch("imessage", [], 10.0, db_path=outbox_db_path)

    assert outbox.get_cursor("imessage", db_path=outbox_db_path) == 10.0
