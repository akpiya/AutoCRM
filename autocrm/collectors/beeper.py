"""Beeper Desktop outbound collector → outbox."""

from __future__ import annotations

import json
import os
import urllib.error
import urllib.parse
import urllib.request
from pathlib import Path
from typing import Any

from autocrm import outbox
from autocrm.collectors.base import Collector, CollectResult
from autocrm.common import (
    beeper_sync_path,
    load_json_state,
    outbox_db,
    save_json_state,
)
from autocrm.models import ContactEvent, PersonHints


class BeeperCollector(Collector):
    app = "beeper"
    outbox_db_path: Path = outbox_db()
    # Beeper uses an HTTP API + a local cursor/watermark JSON file.
    beeper_db_path: Path = beeper_sync_path()

    def collect(self) -> CollectResult:
        cursor_path = self.beeper_db_path
        state = load_json_state(
            cursor_path, default={"last_beeper_sync_at": "2000-01-01T00:00:00Z"}
        )
        date_after = str(state.get("last_beeper_sync_at") or "2000-01-01T00:00:00Z")

        ob = self.outbox_db_path
        outbox.init_db(ob)

        max_ts = date_after
        enq = 0
        cursor: str | None = None
        direction: str | None = None

        while True:
            data = search_messages(
                date_after_iso=date_after,
                sender="me",
                limit=50,
                cursor=cursor,
                direction=direction,
            )
            chats = data.get("chats") or {}
            items = data.get("items") or []
            for msg in items:
                mid = msg.get("id")
                ts = msg.get("timestamp")
                chat_id = msg.get("chatID")
                if not mid or not ts or not chat_id:
                    continue

                ts_s = _norm_ts(str(ts))
                if ts_s > max_ts:
                    max_ts = ts_s

                hints = _hints_from_chat(chats, chat_id)
                chat = chats.get(chat_id) or {}
                channel = _guess_channel(chat)

                ev = ContactEvent(
                    source="beeper",
                    channel=channel,
                    person_hints=hints,
                    occurred_at=ts_s,
                    dedupe_id=f"beeper:{mid}",
                    direction="outbound",
                )
                payload = ev.to_json_bytes().decode("utf-8")
                ins = outbox.enqueue(ob, payload, ev.dedupe_id)
                if ins is not None:
                    enq += 1

            if not data.get("hasMore"):
                break
            nc = data.get("newestCursor")
            oc = data.get("oldestCursor")
            if nc:
                cursor = nc
                direction = "after"
            elif oc:
                cursor = oc
                direction = "before"
            else:
                break

        if max_ts > date_after:
            save_json_state(cursor_path, {"last_beeper_sync_at": max_ts})

        return CollectResult(
            source="beeper",
            enqueued=enq,
            cursor_before=date_after,
            cursor_after=max_ts,
        )


def _base_url() -> str:
    return os.environ.get("BEEPER_DESKTOP_URL", "http://127.0.0.1:23373").rstrip("/")


def _token() -> str:
    t = os.environ.get("BEEPER_DESKTOP_TOKEN", "")
    if not t:
        raise SystemExit("Set BEEPER_DESKTOP_TOKEN to your Beeper Desktop API bearer token.")
    return t


def search_messages(
    date_after_iso: str | None = None,
    sender: str | None = "me",
    limit: int = 50,
    cursor: str | None = None,
    direction: str | None = None,
) -> dict[str, Any]:
    params: list[tuple[str, str]] = []
    if date_after_iso:
        params.append(("dateAfter", date_after_iso))
    if sender:
        params.append(("sender", sender))
    params.append(("limit", str(limit)))
    if cursor:
        params.append(("cursor", cursor))
    if direction:
        params.append(("direction", direction))
    q = urllib.parse.urlencode(params)
    url = f"{_base_url()}/v1/messages/search?{q}"
    req = urllib.request.Request(
        url,
        headers={
            "Authorization": f"Bearer {_token()}",
            "Accept": "application/json",
        },
        method="GET",
    )
    try:
        with urllib.request.urlopen(req, timeout=60) as resp:
            return json.loads(resp.read().decode("utf-8"))
    except urllib.error.HTTPError as e:
        body = e.read().decode("utf-8", errors="replace")
        raise SystemExit(f"Beeper API HTTP {e.code}: {body}") from e
    except urllib.error.URLError as e:
        raise SystemExit(
            f"Beeper connection failed (is Beeper Desktop running?): {e}"
        ) from e


def _hints_from_chat(chats: dict, chat_id: str) -> PersonHints:
    chat = chats.get(chat_id) or {}
    hints = PersonHints(beeper_chat_id=chat_id)
    parts = (chat.get("participants") or {}).get("items") or []
    for u in parts:
        if u.get("isSelf"):
            continue
        if u.get("phoneNumber"):
            hints.phones.append(str(u["phoneNumber"]))
        if u.get("email"):
            hints.emails.append(str(u["email"]))
        if u.get("username"):
            hints.handles.append(str(u["username"]))
        if u.get("id"):
            hints.handles.append(str(u["id"]))
    return hints


def _guess_channel(chat: dict) -> str:
    title = (chat.get("title") or "").lower()
    aid = (chat.get("accountID") or "").lower()
    blob = title + aid
    for name in ("discord", "slack", "instagram", "messenger", "linkedin", "whatsapp"):
        if name in blob:
            return name
    return "unknown"


def _norm_ts(ts: str) -> str:
    s = str(ts).strip()
    if s.endswith("Z"):
        return s
    if "+" in s:
        return s.replace("+00:00", "Z")
    if "T" in s and not s.endswith("Z"):
        return s + "Z"
    return s

