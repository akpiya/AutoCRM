"""Drain the local outbox into Notion (match party_id, monotonic Last contacted)."""

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

from autocrm import outbox
from autocrm.common import DIRECTION_INBOUND, DIRECTION_OUTBOUND, OUTBOX_DB_PATH
from autocrm.outbox import OutboxRow

LOG = logging.getLogger(__name__)


def notion_configured() -> bool:
    token = os.environ.get("NOTION_TOKEN", "").strip()
    db_id = os.environ.get("NOTION_DATABASE_ID", "").strip()
    return bool(token and db_id)


def load_notion_config_from_env() -> dict[str, str]:
    token = os.environ.get("NOTION_TOKEN", "").strip()
    db_id = os.environ.get("NOTION_DATABASE_ID", "").strip()
    if not token or not db_id:
        raise RuntimeError(
            "Set NOTION_TOKEN and NOTION_DATABASE_ID"
        )
    return {
        "NOTION_TOKEN": token,
        "NOTION_DATABASE_ID": db_id,
        "PHONES_PROP": os.environ.get("NOTION_PHONES_PROP", "Phones"),
        "EMAILS_PROP": os.environ.get("NOTION_EMAILS_PROP", "Emails"),
        "LAST_CONTACTED_PROP": os.environ.get(
            "NOTION_LAST_CONTACTED_PROP", "Last Contacted"
        ),
        "LAST_CHANNEL_PROP": os.environ.get(
            "NOTION_LAST_CHANNEL_PROP", "Last Channel"
        ),
    }


def phone_digits(p: str) -> str:
    return "".join(c for c in p if c.isdigit())


def phone_match_variants(p: str) -> set[str]:
    """
    Forms used to compare handles across formatting styles.
    e.g. +1 (703) 395-5764, 7033955764, +17033955764
    """
    digits = phone_digits(p)
    if not digits:
        return set()
    variants = {digits}
    if len(digits) == 11 and digits.startswith("1"):
        variants.add(digits[1:])
    elif len(digits) == 10:
        variants.add("1" + digits)
    return variants


def normalize_email(e: str) -> str:
    return e.strip().lower()


def platform_to_channel_option(platform: str) -> str:
    """Map outbox platform to a Notion select option name."""
    if platform.strip().lower() == "text":
        return "Text"
    return platform.strip()


def channel_property_payload(platform: str, existing_prop: dict | None) -> dict:
    """Build Notion property value for Last Channel (select or legacy rich_text)."""
    label = platform_to_channel_option(platform)
    prop_type = (existing_prop or {}).get("type", "select")
    if prop_type == "rich_text":
        return {
            "rich_text": [
                {"type": "text", "text": {"content": label[:2000]}},
            ],
        }
    return {"select": {"name": label}}


def party_contact_keys(party_id: str) -> tuple[set[str], set[str]]:
    p = party_id.strip()
    if not p:
        return set(), set()
    if "@" in p:
        return set(), {normalize_email(p)}
    return phone_match_variants(p), set()


@dataclass(frozen=True)
class PageUpdate:
    page_id: str
    platform: str
    occurred_at: datetime
    row_ids: tuple[int, ...]


def plan_page_updates(
    rows: list[OutboxRow],
    pages: list[dict],
    cfg_map: Mapping[str, str],
) -> tuple[list[PageUpdate], list[int]]:
    delete_ids: list[int] = []
    by_page_rows: dict[str, list[OutboxRow]] = defaultdict(list)

    for row in rows:
        if row.direction not in (DIRECTION_INBOUND, DIRECTION_OUTBOUND):
            delete_ids.append(row.id)
            continue
        page_id = match_page_for_party(row.party_id, pages, cfg_map)
        if not page_id:
            delete_ids.append(row.id)
            continue
        by_page_rows[page_id].append(row)

    updates: list[PageUpdate] = []
    for page_id, page_rows in by_page_rows.items():
        best = max(page_rows, key=lambda r: r.created_at)
        updates.append(
            PageUpdate(
                page_id=page_id,
                platform=best.platform,
                occurred_at=_unix_to_utc(best.created_at),
                row_ids=tuple(r.id for r in page_rows),
            )
        )
    return updates, delete_ids


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
) -> dict:
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


def _unix_to_utc(ts: float) -> datetime:
    return datetime.fromtimestamp(ts, tz=timezone.utc)


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
            phones |= phone_match_variants(opt.get("name", ""))
    elif p.get("type") == "rich_text":
        for t in p.get("rich_text") or []:
            phones |= phone_match_variants(t.get("plain_text", ""))
    elif p.get("type") == "phone_number":
        if p.get("phone_number"):
            phones |= phone_match_variants(str(p["phone_number"]))

    e = props.get(emails_prop) or {}
    if e.get("type") == "email":
        if e.get("email"):
            emails.add(normalize_email(str(e["email"])))
    elif e.get("type") == "multi_select":
        for opt in e.get("multi_select") or []:
            emails.add(normalize_email(opt.get("name", "")))
    elif e.get("type") == "rich_text":
        for t in e.get("rich_text") or []:
            emails.add(normalize_email(t.get("plain_text", "")))
    return phones, emails


def match_page_for_party(
    party_id: str,
    pages: list[dict],
    cfg_map: Mapping[str, str],
) -> str | None:
    hint_phones, hint_emails = party_contact_keys(party_id)
    if not hint_phones and not hint_emails:
        return None
    phones_prop = cfg_map["PHONES_PROP"]
    emails_prop = cfg_map["EMAILS_PROP"]
    for page in pages:
        pp, ee = _page_phones_emails(page, phones_prop, emails_prop)
        if hint_phones & pp or hint_emails & ee:
            return page["id"]
    return None


def _fetch_all_pages(database_id: str, token: str) -> list[dict]:
    pages: list[dict] = []
    cursor = None
    base = "https://api.notion.com/v1/databases/" + database_id + "/query"
    while True:
        body: dict = {"page_size": 100}
        if cursor:
            body["start_cursor"] = cursor
        r = _notion_request("POST", base, token, body)
        pages.extend(r.get("results") or [])
        if not r.get("has_more"):
            break
        cursor = r.get("next_cursor")
    return pages


def _patch_page(
    page_id: str,
    occurred_at: datetime,
    platform: str,
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
        return
    body = {
        "properties": {
            last_prop: {
                "date": {"start": occ.strftime("%Y-%m-%dT%H:%M:%S.000Z")},
            },
            chan_prop: channel_property_payload(platform, props.get(chan_prop)),
        }
    }
    _notion_request("PATCH", url, token, body)


def sync_outbox(
    db_path: Path | None = None,
    *,
    cfg: dict[str, str] | None = None,
    min_interval: float | None = None,
    batch_size: int = 50,
) -> dict[str, int]:
    if min_interval is None:
        min_interval = float(os.environ.get("NOTION_MIN_INTERVAL", "0.35"))
    cfg = cfg or load_notion_config_from_env()
    token = cfg["NOTION_TOKEN"]
    db_id = cfg["NOTION_DATABASE_ID"]
    cfg_map = {
        "PHONES_PROP": cfg["PHONES_PROP"],
        "EMAILS_PROP": cfg["EMAILS_PROP"],
        "LAST_CONTACTED_PROP": cfg["LAST_CONTACTED_PROP"],
        "LAST_CHANNEL_PROP": cfg["LAST_CHANNEL_PROP"],
    }

    db = db_path or OUTBOX_DB_PATH
    outbox.init_db(db)

    all_pages = _fetch_all_pages(db_id, token)
    LOG.info("Notion sync: loaded %d pages", len(all_pages))

    applied = 0
    errors = 0

    while True:
        batch = outbox.fetch_outbox_batch(batch_size, db_path=db)
        if not batch:
            break

        updates, delete_ids = plan_page_updates(batch, all_pages, cfg_map)

        for update in updates:
            try:
                _patch_page(
                    update.page_id,
                    update.occurred_at,
                    update.platform,
                    token,
                    cfg_map,
                )
                delete_ids.extend(update.row_ids)
                applied += 1
                time.sleep(min_interval)
            except Exception:
                LOG.exception("Notion update failed for page %s", update.page_id)
                errors += 1

        outbox.delete_outbox_rows(delete_ids, db_path=db)

    return {"applied": applied, "errors": errors}
