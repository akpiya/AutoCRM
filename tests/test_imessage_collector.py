from __future__ import annotations

import sqlite3
from pathlib import Path

from autocrm import outbox
from autocrm.common import PLATFORM_TEXT
from autocrm.collectors.imessage import IMessageCollector


def _outbox_rows(db_path: Path) -> list[tuple]:
    conn = sqlite3.connect(db_path)
    try:
        return conn.execute(
            "SELECT platform, party_id, direction, created_at FROM outbox ORDER BY id"
        ).fetchall()
    finally:
        conn.close()


def test_collect_enqueues_and_advances_cursor(collector: IMessageCollector) -> None:
    result = collector.collect()

    assert result.enqueued == 2
    assert result.cursor_before == 0
    assert result.cursor_after == 5

    rows = _outbox_rows(collector.outbox_db_path)
    assert len(rows) == 2
    assert all(r[0] == PLATFORM_TEXT for r in rows)
    assert {r[1] for r in rows} == {"+15551234567", "friend@example.com"}

    assert outbox.get_cursor("imessage", db_path=collector.outbox_db_path) == 5.0


def test_collect_second_run_empty(collector: IMessageCollector) -> None:
    collector.collect()
    result = collector.collect()

    assert result.enqueued == 0
    assert result.cursor_before == 5
    assert result.cursor_after == 5
    assert len(_outbox_rows(collector.outbox_db_path)) == 2


def test_collect_advances_past_bad_dates(collector: IMessageCollector) -> None:
    collector.collect()

    assert outbox.get_cursor("imessage", db_path=collector.outbox_db_path) == 5.0
    assert len(_outbox_rows(collector.outbox_db_path)) == 2


def test_missing_chat_db_raises(tmp_path: Path, outbox_db_path: Path) -> None:
    collector = IMessageCollector(
        chat_db_path=tmp_path / "missing.db",
        outbox_db_path=outbox_db_path,
    )
    try:
        collector.collect()
        assert False, "expected FileNotFoundError"
    except FileNotFoundError:
        pass
