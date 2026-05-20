from __future__ import annotations

from pathlib import Path

from autocrm.collectors.imessage_db import fetch_outbound_messages
from autocrm.common import APPLE_EPOCH_UTC, apple_ns_to_unix
from tests.fixtures.build_chat_db import APPLE_NS_2001_01_02


def test_apple_ns_to_unix() -> None:
    assert apple_ns_to_unix(0) is None
    assert apple_ns_to_unix(None) is None
    ns = APPLE_NS_2001_01_02 + 1_000_000_000
    expected = APPLE_EPOCH_UTC.timestamp() + ns / 1e9
    assert apple_ns_to_unix(ns) == expected


def test_fetch_outbound_after_rowid(chat_db_path: Path) -> None:
    rows = fetch_outbound_messages(chat_db_path, 0)
    assert [r.rowid for r in rows] == [2, 3, 4, 5]

    rows_after_2 = fetch_outbound_messages(chat_db_path, 2)
    assert [r.rowid for r in rows_after_2] == [3, 4, 5]
