"""Contact event JSON stored in the local outbox and consumed by notion_sync."""

from __future__ import annotations

import json
from dataclasses import asdict, dataclass, field
from typing import Any


@dataclass
class PersonHints:
    phones: list[str] = field(default_factory=list)
    emails: list[str] = field(default_factory=list)
    handles: list[str] = field(default_factory=list)
    beeper_chat_id: str | None = None

    def to_dict(self) -> dict[str, Any]:
        d: dict[str, Any] = {
            "phones": self.phones,
            "emails": self.emails,
            "handles": self.handles,
        }
        if self.beeper_chat_id:
            d["beeper_chat_id"] = self.beeper_chat_id
        return d


@dataclass
class ContactEvent:
    schema_version: int = 1
    source: str = ""
    channel: str = ""
    person_hints: PersonHints = field(default_factory=PersonHints)
    occurred_at: str = ""  # ISO 8601 UTC
    dedupe_id: str | None = None
    direction: str | None = "outbound"

    def to_json_bytes(self) -> bytes:
        d: dict[str, Any] = {
            "schema_version": self.schema_version,
            "source": self.source,
            "channel": self.channel,
            "person_hints": self.person_hints.to_dict(),
            "occurred_at": self.occurred_at,
        }
        if self.dedupe_id:
            d["dedupe_id"] = self.dedupe_id
        if self.direction:
            d["direction"] = self.direction
        return json.dumps(d, separators=(",", ":")).encode("utf-8")
