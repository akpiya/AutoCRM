from __future__ import annotations

import sqlite3
from pathlib import Path

from autocrm import outbox
from autocrm.common import DIRECTION_INBOUND, DIRECTION_OUTBOUND, PLATFORM_TEXT
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

    assert result.enqueued == 8
    assert result.cursor_before == 0
    assert result.cursor_after == 7

    rows = _outbox_rows(collector.outbox_db_path)
    assert len(rows) == 8
    assert all(r[0] == PLATFORM_TEXT for r in rows)

    parties = {r[1] for r in rows}
    assert parties == {"+15551234567", "+15559876543", "friend@example.com"}

    directions = {r[2] for r in rows}
    assert directions == {DIRECTION_INBOUND, DIRECTION_OUTBOUND}

    inbound = [r for r in rows if r[2] == DIRECTION_INBOUND]
    assert len(inbound) == 3
    assert {r[1] for r in inbound} == {
        "+15551234567",
        "+15559876543",
        "friend@example.com",
    }

    outbound = [r for r in rows if r[2] == DIRECTION_OUTBOUND]
    assert len(outbound) == 5

    assert outbox.get_cursor("imessage", db_path=collector.outbox_db_path) == 7.0


def test_collect_second_run_empty(collector: IMessageCollector) -> None:
    collector.collect()
    result = collector.collect()

    assert result.enqueued == 0
    assert result.cursor_before == 7
    assert result.cursor_after == 7
    assert len(_outbox_rows(collector.outbox_db_path)) == 8


def test_collect_advances_past_bad_dates(collector: IMessageCollector) -> None:
    collector.collect()

    assert outbox.get_cursor("imessage", db_path=collector.outbox_db_path) == 7.0
    assert len(_outbox_rows(collector.outbox_db_path)) == 8


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
