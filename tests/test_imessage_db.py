from __future__ import annotations

from pathlib import Path

from autocrm.collectors.imessage_db import fetch_messages_after_rowid
from autocrm.common import APPLE_EPOCH_UTC, apple_ns_to_unix
from tests.fixtures.build_chat_db import APPLE_NS_2001_01_02


def test_apple_ns_to_unix() -> None:
    assert apple_ns_to_unix(0) is None
    assert apple_ns_to_unix(None) is None
    ns = APPLE_NS_2001_01_02 + 1_000_000_000
    expected = APPLE_EPOCH_UTC.timestamp() + ns / 1e9
    assert apple_ns_to_unix(ns) == expected


def test_fetch_messages_after_rowid(chat_db_path: Path) -> None:
    rows = fetch_messages_after_rowid(chat_db_path, 0)
    assert [r.rowid for r in rows] == [1, 2, 3, 4, 5, 6, 7]

    direct_in = rows[0]
    assert not direct_in.is_group
    assert direct_in.sender_handle == "+15551234567"
    assert not direct_in.is_from_me

    group_out = rows[4]
    assert group_out.is_group
    assert group_out.is_from_me
    assert group_out.sender_handle is None
    assert group_out.member_handles == (
        "+15551234567",
        "+15559876543",
        "friend@example.com",
    )

    group_in = rows[6]
    assert group_in.is_group
    assert not group_in.is_from_me
    assert group_in.sender_handle == "+15559876543"

    rows_after_2 = fetch_messages_after_rowid(chat_db_path, 2)
    assert [r.rowid for r in rows_after_2] == [3, 4, 5, 6, 7]


def test_fetch_messages_skips_orphan_without_chat_join(tmp_path: Path) -> None:
    from tests.fixtures.build_chat_db import build_chat_db

    path = tmp_path / "chat.db"
    build_chat_db(path)
    conn = __import__("sqlite3").connect(path)
    try:
        conn.execute(
            "INSERT INTO message (ROWID, date, is_from_me, service, handle_id) "
            "VALUES (8, ?, 1, 'iMessage', 1)",
            (APPLE_NS_2001_01_02 + 6_000_000_000,),
        )
        conn.commit()
    finally:
        conn.close()

    rows = fetch_messages_after_rowid(path, 7)
    assert [r.rowid for r in rows] == []
