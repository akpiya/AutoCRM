"""Shared paths and time helpers."""

from __future__ import annotations

from datetime import datetime, timezone
from pathlib import Path

APPLE_EPOCH_UTC = datetime(2001, 1, 1, 0, 0, 0, tzinfo=timezone.utc)

DEFAULT_CHAT_DB_PATH = Path.home() / "Library" / "Messages" / "chat.db"

DIRECTION_INBOUND = 0
DIRECTION_OUTBOUND = 1
PLATFORM_TEXT = "text"

def autocrm_dir() -> Path:
    d = Path.home() / ".autocrm"
    d.mkdir(parents=True, exist_ok=True)
    return d

OUTBOX_DB_PATH = autocrm_dir() / "outbox.db"

def apple_ns_to_unix(ns: int | float | None) -> float | None:
    if ns is None:
        return None
    try:
        n = int(ns)
    except (TypeError, ValueError):
        return None
    if n == 0:
        return None
    return APPLE_EPOCH_UTC.timestamp() + n / 1e9
