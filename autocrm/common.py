"""Shared paths, time helpers, and Notion tuning constants."""

from __future__ import annotations

from datetime import datetime, timezone
from pathlib import Path

APPLE_EPOCH_UTC = datetime(2001, 1, 1, 0, 0, 0, tzinfo=timezone.utc)

DEFAULT_CHAT_DB_PATH = Path.home() / "Library" / "Messages" / "chat.db"

DIRECTION_INBOUND = 0
DIRECTION_OUTBOUND = 1
PLATFORM_TEXT = "text"

# Notion people database property names (must match your Notion schema).
NOTION_NAME_PROP = "Name"
NOTION_PHONES_PROP = "Phones"
NOTION_EMAILS_PROP = "Emails"
NOTION_LAST_CONTACTED_PROP = "Last Contacted"
NOTION_LAST_CHANNEL_PROP = "Last Channel"

# Notion sync tuning.
NOTION_MIN_INTERVAL = 0.35  # seconds between PATCH request starts (rate limit)
NOTION_PATCH_WORKERS = 2


def notion_property_config() -> dict[str, str]:
    """Property name map for Notion API calls (not from environment)."""
    return {
        "PHONES_PROP": NOTION_PHONES_PROP,
        "EMAILS_PROP": NOTION_EMAILS_PROP,
        "LAST_CONTACTED_PROP": NOTION_LAST_CONTACTED_PROP,
        "LAST_CHANNEL_PROP": NOTION_LAST_CHANNEL_PROP,
    }


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
