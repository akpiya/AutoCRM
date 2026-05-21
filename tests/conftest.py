from __future__ import annotations

from pathlib import Path

import pytest

from autocrm.collectors.imessage import IMessageCollector
from tests.fixtures.build_chat_db import build_chat_db


@pytest.fixture
def chat_db_path(tmp_path: Path) -> Path:
    path = tmp_path / "chat.db"
    build_chat_db(path)
    return path


@pytest.fixture
def outbox_db_path(tmp_path: Path) -> Path:
    return tmp_path / "outbox.db"


@pytest.fixture
def collector(chat_db_path: Path, outbox_db_path: Path) -> IMessageCollector:
    from autocrm import outbox

    outbox.init_db(outbox_db_path)
    outbox.ingest_outbox_batch("imessage", [], 0.0, db_path=outbox_db_path)
    return IMessageCollector(
        chat_db_path=chat_db_path,
        outbox_db_path=outbox_db_path,
    )
