from __future__ import annotations

import sqlite3

from autocrm import outbox
from autocrm.collectors.imessage import IMessageCollector


def test_missing_cursor_skips_ingest_and_sets_max_rowid(
    chat_db_path, outbox_db_path
) -> None:
    collector = IMessageCollector(
        chat_db_path=chat_db_path,
        outbox_db_path=outbox_db_path,
    )
    outbox.init_db(outbox_db_path)

    result = collector.collect()

    assert result.enqueued == 0
    assert result.cursor_before is None
    assert result.cursor_after == 7
    assert outbox.get_cursor("imessage", db_path=outbox_db_path) == 7.0

    conn = sqlite3.connect(outbox_db_path)
    try:
        assert conn.execute("SELECT COUNT(*) FROM outbox").fetchone()[0] == 0
    finally:
        conn.close()
