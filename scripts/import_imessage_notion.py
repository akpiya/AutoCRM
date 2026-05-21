#!/usr/bin/env python3
"""
Import contacts from a vCard (.vcf) into the Notion people database.

Requires AUTOCRM_NOTION_TOKEN and AUTOCRM_NOTION_DATABASE_ID (same as autocrm.main).

Usage (from repo root, package installed):

  python scripts/import_imessage_notion.py path/to/contacts.vcf
"""

from __future__ import annotations

import argparse
import json
import os
import sys
import urllib.error
import urllib.request
from dataclasses import dataclass
from pathlib import Path

from autocrm.notion import load_notion_config_from_env


@dataclass(frozen=True)
class VCardContact:
    name: str
    phones: tuple[str, ...]
    emails: tuple[str, ...]


def ensure_notion_env() -> dict[str, str]:
    missing: list[str] = []
    if not os.environ.get("AUTOCRM_NOTION_TOKEN", "").strip():
        missing.append("AUTOCRM_NOTION_TOKEN")
    if not os.environ.get("AUTOCRM_NOTION_DATABASE_ID", "").strip():
        missing.append("AUTOCRM_NOTION_DATABASE_ID")
    if missing:
        print(
            "Missing required environment variables: " + ", ".join(missing),
            file=sys.stderr,
        )
        print(
            "Export them in your shell or add them to your launchd plist, then retry.",
            file=sys.stderr,
        )
        raise SystemExit(1)
    return load_notion_config_from_env()


def _notion_request(
    method: str,
    url: str,
    token: str,
    body: dict | None = None,
) -> dict:
    data = None
    if body is not None:
        data = json.dumps(body).encode("utf-8")
    req = urllib.request.Request(
        url,
        data=data,
        method=method,
        headers={
            "Authorization": f"Bearer {token}",
            "Notion-Version": "2022-06-28",
            "Content-Type": "application/json",
        },
    )
    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            return json.loads(resp.read().decode("utf-8"))
    except urllib.error.HTTPError as e:
        err_body = e.read().decode("utf-8", errors="replace")
        print(f"Notion HTTP {e.code}: {err_body}", file=sys.stderr)
        raise


def create_people_page(
    name: str,
    phones: list[str],
    emails: list[str],
    *,
    cfg: dict[str, str],
) -> str:
    name_prop = os.environ.get("AUTOCRM_NOTION_NAME_PROP", "Name")
    phones_prop = cfg["PHONES_PROP"]
    emails_prop = cfg["EMAILS_PROP"]
    token = cfg["NOTION_TOKEN"]
    db_id = cfg["NOTION_DATABASE_ID"]

    properties: dict = {
        name_prop: {
            "title": [{"type": "text", "text": {"content": name[:2000]}}],
        },
    }
    if phones:
        properties[phones_prop] = {
            "multi_select": [{"name": p[:2000]} for p in phones],
        }
    if emails:
        properties[emails_prop] = {
            "multi_select": [{"name": e[:2000]} for e in emails],
        }

    body = {"parent": {"database_id": db_id}, "properties": properties}
    result = _notion_request(
        "POST",
        "https://api.notion.com/v1/pages",
        token,
        body,
    )
    return str(result["id"])


def _format_n(value: str) -> str:
    parts = value.split(";")
    family = parts[0].strip() if len(parts) > 0 else ""
    given = parts[1].strip() if len(parts) > 1 else ""
    if given and family:
        return f"{given} {family}"
    return given or family or ""


def _dedupe(values: list[str]) -> list[str]:
    seen: set[str] = set()
    out: list[str] = []
    for v in values:
        key = v.strip()
        if not key or key in seen:
            continue
        seen.add(key)
        out.append(key)
    return out


def parse_vcf(path: Path) -> list[VCardContact]:
    text = path.read_text(encoding="utf-8", errors="replace")
    unfolded: list[str] = []
    for raw in text.splitlines():
        if unfolded and raw.startswith((" ", "\t")):
            unfolded[-1] += raw[1:]
        else:
            unfolded.append(raw.strip())

    contacts: list[VCardContact] = []
    fields: dict[str, list[str]] | None = None

    def flush() -> None:
        nonlocal fields
        if fields is None:
            return
        name = fields["fn"][0] if fields.get("fn") else ""
        if not name and fields.get("n"):
            name = _format_n(fields["n"][0])
        phones = _dedupe(fields.get("tel", []))
        emails = _dedupe([e.lower() for e in fields.get("email", [])])
        if name or phones or emails:
            contacts.append(
                VCardContact(
                    name=name or "Unknown",
                    phones=tuple(phones),
                    emails=tuple(emails),
                )
            )
        fields = None

    for line in unfolded:
        if line == "BEGIN:VCARD":
            flush()
            fields = {}
            continue
        if line == "END:VCARD":
            flush()
            continue
        if fields is None or ":" not in line:
            continue
        key, _, value = line.partition(":")
        tag = key.split(";", 1)[0].upper()
        value = value.strip()
        if not value:
            continue
        if tag == "FN":
            fields.setdefault("fn", []).append(value)
        elif tag == "N":
            fields.setdefault("n", []).append(value)
        elif tag == "TEL":
            fields.setdefault("tel", []).append(value)
        elif tag == "EMAIL":
            fields.setdefault("email", []).append(value)
    flush()
    return contacts


def _print_contact(contact: VCardContact, index: int, total: int) -> None:
    print()
    print(f"Contact {index + 1} of {total}")
    print(f"  Name:   {contact.name}")
    if contact.phones:
        print("  Phones: " + ", ".join(contact.phones))
    else:
        print("  Phones: (none)")
    if contact.emails:
        print("  Emails: " + ", ".join(contact.emails))
    else:
        print("  Emails: (none)")


def _prompt_track() -> bool:
    while True:
        answer = input("Track this contact? [y/N]: ").strip().lower()
        if answer in ("y", "yes"):
            return True
        if answer in ("n", "no", ""):
            return False
        print("Please enter y or n.")


def main() -> None:
    parser = argparse.ArgumentParser(
        description="Import selected vCard contacts into your Notion people database."
    )
    parser.add_argument(
        "vcf",
        type=Path,
        help="Path to a .vcf / vCard file",
    )
    args = parser.parse_args()

    vcf_path: Path = args.vcf
    if not vcf_path.is_file():
        print(f"File not found: {vcf_path}", file=sys.stderr)
        raise SystemExit(1)

    cfg = ensure_notion_env()
    contacts = parse_vcf(vcf_path)
    if not contacts:
        print(f"No contacts found in {vcf_path}", file=sys.stderr)
        raise SystemExit(1)

    tracked = 0
    skipped = 0

    for i, contact in enumerate(contacts):
        _print_contact(contact, i, len(contacts))
        if not contact.phones and not contact.emails:
            print("  (No phone or email — AutoCRM cannot match this person for sync.)")
        if not _prompt_track():
            skipped += 1
            continue
        try:
            page_id = create_people_page(
                contact.name,
                list(contact.phones),
                list(contact.emails),
                cfg=cfg,
            )
        except Exception as exc:
            print(f"Failed to create Notion page: {exc}", file=sys.stderr)
            raise SystemExit(1) from exc
        tracked += 1
        print(f"  Added to Notion (page {page_id}).")

    print()
    print(f"Done. Tracked {tracked}, skipped {skipped}, {len(contacts)} total.")


if __name__ == "__main__":
    main()
