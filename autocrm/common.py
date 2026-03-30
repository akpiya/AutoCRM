"""Shared utilities (time + local state paths)."""

from __future__ import annotations

from datetime import datetime, timedelta, timezone
import json
from pathlib import Path
from typing import Any

# macOS / iOS reference date for NSTimeInterval-style storage in chat.db
APPLE_EPOCH_UTC = datetime(2001, 1, 1, 0, 0, 0, tzinfo=timezone.utc)


def nanoseconds_to_iso_utc(ns: int | float | None) -> str | None:
    """Convert Apple `message.date` (ns since 2001-01-01 UTC) to ISO 8601 UTC."""
    if ns is None:
        return None
    try:
        n = int(ns)
    except (TypeError, ValueError):
        return None
    if n == 0:
        return None
    seconds = n / 1e9
    dt = APPLE_EPOCH_UTC + timedelta(seconds=seconds)
    return dt.astimezone(timezone.utc).strftime("%Y-%m-%dT%H:%M:%S.%f")[:-3] + "Z"


def data_dir() -> Path:
    d = Path.home() / ".autocrm"
    d.mkdir(parents=True, exist_ok=True)
    return d


def outbox_db() -> Path:
    return data_dir() / "outbox.db"


def imessage_cursor_path() -> Path:
    return data_dir() / "imessage_cursor.json"


def beeper_sync_path() -> Path:
    return data_dir() / "beeper_sync.json"


def default_chat_db() -> Path:
    return Path.home() / "Library" / "Messages" / "chat.db"

def load_json_state(path: Path, *, default: dict[str, Any]) -> dict[str, Any]:
    if not path.exists():
        return dict(default)
    try:
        data = json.loads(path.read_text(encoding="utf-8"))
        if isinstance(data, dict):
            merged = dict(default)
            merged.update(data)
            return merged
    except Exception:
        pass
    return dict(default)
    
def save_json_state(path: Path, data: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(data, indent=2) + "\n", encoding="utf-8")

