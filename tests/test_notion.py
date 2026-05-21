from __future__ import annotations

from autocrm import notion
from autocrm.common import DIRECTION_INBOUND, DIRECTION_OUTBOUND, PLATFORM_TEXT
from autocrm.outbox import OutboxRow


def _page(page_id: str, phones: list[str], emails: list[str]) -> dict:
    return {
        "id": page_id,
        "properties": {
            "Phones": {
                "type": "multi_select",
                "multi_select": [{"name": p} for p in phones],
            },
            "Emails": {
                "type": "email",
                "email": emails[0] if emails else None,
            },
        },
    }


CFG = {
    "PHONES_PROP": "Phones",
    "EMAILS_PROP": "Emails",
    "LAST_CONTACTED_PROP": "Last Contacted",
    "LAST_CHANNEL_PROP": "Last Channel",
}


def test_match_page_for_party_phone() -> None:
    pages = [_page("p1", ["+15551234567"], [])]
    assert notion.match_page_for_party("+15551234567", pages, CFG) == "p1"


def test_match_page_phone_format_variants() -> None:
    pages = [_page("p1", ["(555) 123-4567"], [])]
    assert notion.match_page_for_party("+15551234567", pages, CFG) == "p1"
    assert notion.match_page_for_party("5551234567", pages, CFG) == "p1"


def test_platform_to_channel_option() -> None:
    assert notion.platform_to_channel_option("text") == "Text"
    assert notion.platform_to_channel_option("TEXT") == "Text"


def test_channel_property_payload_select() -> None:
    payload = notion.channel_property_payload("text", {"type": "select"})
    assert payload == {"select": {"name": "Text"}}


def test_phone_match_variants_us_ten_and_eleven() -> None:
    assert notion.phone_match_variants("+17033955764") & notion.phone_match_variants(
        "7033955764"
    )


def test_match_page_for_party_email() -> None:
    pages = [_page("p2", [], ["friend@example.com"])]
    assert notion.match_page_for_party("friend@example.com", pages, CFG) == "p2"


def test_plan_page_updates_includes_inbound() -> None:
    pages = [_page("p1", ["+15551234567"], [])]
    rows = [
        OutboxRow(1, PLATFORM_TEXT, "+15551234567", DIRECTION_INBOUND, 100.0),
        OutboxRow(2, PLATFORM_TEXT, "+15551234567", DIRECTION_OUTBOUND, 50.0),
    ]
    updates, delete_ids = notion.plan_page_updates(rows, pages, CFG)

    assert delete_ids == []
    assert len(updates) == 1
    assert updates[0].page_id == "p1"
    assert updates[0].occurred_at.timestamp() == 100.0


def test_plan_page_updates_groups_and_drops_unmatched() -> None:
    pages = [_page("p1", ["+15551234567"], [])]
    rows = [
        OutboxRow(1, PLATFORM_TEXT, "+15551234567", DIRECTION_OUTBOUND, 100.0),
        OutboxRow(2, PLATFORM_TEXT, "+15551234567", DIRECTION_OUTBOUND, 200.0),
        OutboxRow(3, PLATFORM_TEXT, "unknown@example.com", DIRECTION_OUTBOUND, 150.0),
        OutboxRow(4, PLATFORM_TEXT, "+1999", DIRECTION_OUTBOUND, 50.0),
    ]
    updates, delete_ids = notion.plan_page_updates(rows, pages, CFG)

    assert len(updates) == 1
    assert updates[0].page_id == "p1"
    assert updates[0].occurred_at.timestamp() == 200.0
    assert set(updates[0].row_ids) == {1, 2}
    assert set(delete_ids) == {3, 4}
