"""
Drain the local outbox into Notion (phone/email match, monotonic Last contacted).
Uses stdlib HTTP and env-based config.
"""

from __future__ import annotations

import json
import logging
import os
import time
import urllib.error
import urllib.request
from collections import defaultdict
from collections.abc import Mapping
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

from autocrm import outbox
from autocrm.common import outbox_db

LOG = logging.getLogger(__name__)


@dataclass
class EventPayload:
    schema_version: int
    source: str
    channel: str
    person_hints: dict[str, Any]
    occurred_at: str
    dedupe_id: str | None
    direction: str | None = None

    @classmethod
    def from_body(cls, raw: str) -> EventPayload:
        d = json.loads(raw)
        return cls(
            schema_version=int(d.get("schema_version", 1)),
            source=str(d["source"]),
            channel=str(d["channel"]),
            person_hints=dict(d.get("person_hints") or {}),
            occurred_at=str(d["occurred_at"]),
            dedupe_id=d.get("dedupe_id"),
            direction=d.get("direction"),
        )


def load_notion_config_from_env() -> dict[str, str]:
    token = os.environ.get("AUTOCRM_NOTION_TOKEN", "").strip()
    db_id = os.environ.get("AUTOCRM_NOTION_DATABASE_ID", "").strip()
    if not token or not db_id:
        raise SystemExit(
            "Set AUTOCRM_NOTION_TOKEN and AUTOCRM_NOTION_DATABASE_ID "
            "(Notion integration token and people database ID)."
        )
    return {
        "NOTION_TOKEN": token,
        "NOTION_DATABASE_ID": db_id,
        "PHONES_PROP": os.environ.get("AUTOCRM_NOTION_PHONES_PROP", "Phones"),
        "EMAILS_PROP": os.environ.get("AUTOCRM_NOTION_EMAILS_PROP", "Emails"),
        "LAST_CONTACTED_PROP": os.environ.get(
            "AUTOCRM_NOTION_LAST_CONTACTED_PROP", "Last contacted"
        ),
        "LAST_CHANNEL_PROP": os.environ.get(
            "AUTOCRM_NOTION_LAST_CHANNEL_PROP", "Last channel"
        ),
    }


def _notion_headers(token: str) -> dict[str, str]:
    return {
        "Authorization": f"Bearer {token}",
        "Notion-Version": "2022-06-28",
        "Content-Type": "application/json",
    }


def _notion_request(
    method: str,
    url: str,
    token: str,
    body: dict | None = None,
    timeout: int = 30,
) -> dict[str, Any]:
    data = None
    if body is not None:
        data = json.dumps(body).encode("utf-8")
    req = urllib.request.Request(
        url,
        data=data,
        method=method,
        headers=_notion_headers(token),
    )
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return json.loads(resp.read().decode("utf-8"))
    except urllib.error.HTTPError as e:
        err_body = e.read().decode("utf-8", errors="replace")
        LOG.error("Notion HTTP %s: %s", e.code, err_body)
        raise


def _parse_notion_datetime(val: dict | None) -> datetime | None:
    if not val or val.get("type") != "date":
        return None
    d = val.get("date")
    if not d or not d.get("start"):
        return None
    start = d["start"]
    if len(start) == 10:
        start = start + "T00:00:00+00:00"
    dt = datetime.fromisoformat(start.replace("Z", "+00:00"))
    if dt.tzinfo is None:
        dt = dt.replace(tzinfo=timezone.utc)
    return dt.astimezone(timezone.utc)


def _event_datetime(iso_s: str) -> datetime:
    s = iso_s.replace("Z", "+00:00")
    dt = datetime.fromisoformat(s)
    if dt.tzinfo is None:
        dt = dt.replace(tzinfo=timezone.utc)
    return dt.astimezone(timezone.utc)


def _normalize_phone(p: str) -> str:
    return "".join(c for c in p if c.isdigit() or c == "+")


def _normalize_email(e: str) -> str:
    return e.strip().lower()


def _page_phones_emails(
    page: dict,
    phones_prop: str,
    emails_prop: str,
) -> tuple[set[str], set[str]]:
    props = page.get("properties") or {}
    phones: set[str] = set()
    emails: set[str] = set()
    p = props.get(phones_prop) or {}
    if p.get("type") == "multi_select":
        for opt in p.get("multi_select") or []:
            phones.add(_normalize_phone(opt.get("name", "")))
    elif p.get("type") == "rich_text":
        for t in p.get("rich_text") or []:
            phones.add(_normalize_phone(t.get("plain_text", "")))
    elif p.get("type") == "phone_number":
        if p.get("phone_number"):
            phones.add(_normalize_phone(str(p["phone_number"])))

    e = props.get(emails_prop) or {}
    if e.get("type") == "email":
        if e.get("email"):
            emails.add(_normalize_email(str(e["email"])))
    elif e.get("type") == "multi_select":
        for opt in e.get("multi_select") or []:
            emails.add(_normalize_email(opt.get("name", "")))
    elif e.get("type") == "rich_text":
        for t in e.get("rich_text") or []:
            emails.add(_normalize_email(t.get("plain_text", "")))
    return phones, emails


def _fetch_all_pages(database_id: str, token: str) -> list[dict]:
    pages: list[dict] = []
    cursor = None
    base = "https://api.notion.com/v1/databases/" + database_id + "/query"
    while True:
        body: dict[str, Any] = {"page_size": 100}
        if cursor:
            body["start_cursor"] = cursor
        r = _notion_request("POST", base, token, body)
        pages.extend(r.get("results") or [])
        if not r.get("has_more"):
            break
        cursor = r.get("next_cursor")
    return pages


def _match_page(
    hints: dict[str, Any],
    pages: list[dict],
    cfg_map: Mapping[str, str],
) -> str | None:
    hint_phones = {_normalize_phone(p) for p in (hints.get("phones") or []) if p}
    hint_emails = {_normalize_email(e) for e in (hints.get("emails") or []) if e}
    if not hint_phones and not hint_emails:
        return None
    phones_prop = cfg_map["PHONES_PROP"]
    emails_prop = cfg_map["EMAILS_PROP"]
    for page in pages:
        pp, ee = _page_phones_emails(page, phones_prop, emails_prop)
        if hint_phones & pp or hint_emails & ee:
            return page["id"]
    return None


def _patch_page_dates(
    page_id: str,
    occurred_at: datetime,
    channel: str,
    token: str,
    cfg_map: Mapping[str, str],
) -> None:
    last_prop = cfg_map["LAST_CONTACTED_PROP"]
    chan_prop = cfg_map["LAST_CHANNEL_PROP"]
    url = f"https://api.notion.com/v1/pages/{page_id}"
    page = _notion_request("GET", url, token)
    props = page.get("properties") or {}
    current = _parse_notion_datetime(props.get(last_prop))
    occ = occurred_at.astimezone(timezone.utc)
    if current is not None and occ < current:
        LOG.info("Skip page %s: event older than stored", page_id)
        return
    body = {
        "properties": {
            last_prop: {
                "type": "date",
                "date": {"start": occ.strftime("%Y-%m-%dT%H:%M:%S.000Z")},
            },
            chan_prop: {
                "type": "rich_text",
                "rich_text": [
                    {
                        "type": "text",
                        "text": {"content": channel[:2000]},
                    }
                ],
            },
        }
    }
    _notion_request("PATCH", url, token, body)


def sync_pending_to_notion(
    db_path: Path | None = None,
    *,
    cfg: dict[str, str] | None = None,
    min_interval: float | None = None,
    max_rows: int | None = None,
) -> dict[str, int]:
    """
    Process pending outbox rows: match Notion pages, patch dates, delete rows on success.
    Rows that cannot be parsed or matched are removed (poison / skip semantics).
    On Notion API errors, affected rows stay pending for retry.

    Returns counts: applied_page_updates, dropped_unmatched, dropped_bad, errors
    """
    if min_interval is None:
        min_interval = float(os.environ.get("AUTOCRM_NOTION_MIN_INTERVAL", "0.35"))
    cfg = cfg or load_notion_config_from_env()
    token = cfg["NOTION_TOKEN"]
    db_id = cfg["NOTION_DATABASE_ID"]
    cfg_map = {
        "PHONES_PROP": cfg["PHONES_PROP"],
        "EMAILS_PROP": cfg["EMAILS_PROP"],
        "LAST_CONTACTED_PROP": cfg["LAST_CONTACTED_PROP"],
        "LAST_CHANNEL_PROP": cfg["LAST_CHANNEL_PROP"],
    }

    db = db_path or outbox_db()
    outbox.init_db(db)

    all_pages = _fetch_all_pages(db_id, token)
    LOG.info("Loaded %d Notion pages for matching", len(all_pages))

    applied = 0
    dropped_unmatched = 0
    dropped_bad = 0
    errors = 0
    rows_touched = 0

    while True:
        if max_rows is not None and rows_touched >= max_rows:
            break
        batch_limit = 10
        if max_rows is not None:
            remaining = max_rows - rows_touched
            if remaining <= 0:
                break
            batch_limit = min(10, remaining)
        batch = outbox.fetch_pending_batch(db, batch_limit)
        if not batch:
            break

        parsed: list[tuple[int, EventPayload]] = []
        delete_ids: list[int] = []

        for row_id, payload_json, _dedupe in batch:
            rows_touched += 1
            try:
                payload = EventPayload.from_body(payload_json)
                parsed.append((row_id, payload))
            except Exception:
                LOG.exception("Parse error for outbox row %s, removing", row_id)
                dropped_bad += 1
                delete_ids.append(row_id)

        by_page: dict[str, list[tuple[int, EventPayload]]] = defaultdict(list)

        for row_id, payload in parsed:
            if payload.direction is not None and payload.direction != "outbound":
                delete_ids.append(row_id)
                continue
            page_id = _match_page(payload.person_hints, all_pages, cfg_map)
            if not page_id:
                LOG.warning("No Notion match for hints=%s", payload.person_hints)
                dropped_unmatched += 1
                delete_ids.append(row_id)
                continue
            by_page[page_id].append((row_id, payload))

        for page_id, items in by_page.items():
            best = max(items, key=lambda it: _event_datetime(it[1].occurred_at))
            best_row_id, best_payload = best[0], best[1]
            row_ids_for_page = [r for r, _ in items]
            try:
                _patch_page_dates(
                    page_id,
                    _event_datetime(best_payload.occurred_at),
                    best_payload.channel,
                    token,
                    cfg_map,
                )
                delete_ids.extend(row_ids_for_page)
                applied += 1
                time.sleep(min_interval)
            except Exception:
                LOG.exception("Notion update failed for page %s", page_id)
                errors += 1

        for rid in delete_ids:
            outbox.delete_row(db, rid)

    return {
        "applied_page_updates": applied,
        "dropped_unmatched": dropped_unmatched,
        "dropped_bad": dropped_bad,
        "errors": errors,
    }
