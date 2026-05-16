"""Shared local state paths."""

from __future__ import annotations

from pathlib import Path


def autocrm_dir() -> Path:
    d = Path.home() / ".autocrm"
    d.mkdir(parents=True, exist_ok=True)
    return d


OUTBOX_DB_PATH = autocrm_dir() / "outbox.db"
