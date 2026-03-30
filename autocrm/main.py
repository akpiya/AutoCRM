"""
AutoCRM entrypoint.

Design:
- This is the only entrypoint launchd runs (at load + every 2 minutes).
- It runs collectors serially (iMessage → Beeper → Phone calls) to update the outbox.
- Then it drains the outbox into Notion.
"""

from __future__ import annotations

import logging

from autocrm import outbox
from autocrm.common import outbox_db
from autocrm.collectors.beeper import BeeperCollector
from autocrm.collectors.imessage import IMessageCollector
from autocrm.collectors.phone import PhoneCallsCollector
from autocrm.notion_sync import sync_pending_to_notion


def run() -> None:
    logging.basicConfig(level=logging.INFO, format="%(levelname)s %(message)s")
    log = logging.getLogger(__name__)

    failures: list[str] = []
    enqueued_by_app: dict[str, int] = {}

    # 1) Ingest serially into outbox
    collectors = [IMessageCollector(), PhoneCallsCollector(), BeeperCollector()]
    for c in collectors:
        try:
            log.info("Collector start: %s", c.app)
            r = c.collect()
            enqueued_by_app[c.app] = int(r.enqueued)
            log.info(
                "Collector done: %s (enqueued=%s cursor_before=%s cursor_after=%s)",
                c.app,
                r.enqueued,
                r.cursor_before,
                r.cursor_after,
            )
        except BaseException as e:
            failures.append(c.app)
            enqueued_by_app.setdefault(c.app, 0)
            log.exception("Collector failed: %s (%s)", c.app, e)

    # 2) Drain outbox to Notion
    stats: dict[str, int] | None = None
    try:
        log.info("Notion sync start")
        stats = sync_pending_to_notion()
        log.info("Notion sync done: %s", stats)
        if stats["errors"] > 0:
            failures.append("notion")
    except BaseException as e:
        failures.append("notion")
        log.exception("Notion sync failed (%s)", e)
    finally:
        pending = outbox.pending_count(outbox_db())
        parts = []
        for c in collectors:
            parts.append(f"{c.app}={enqueued_by_app.get(c.app, 0)}")
        notion_part = "notion=failed"
        if stats is not None:
            notion_part = (
                f"notion_pages_updated={stats.get('applied_page_updates', 0)} "
                f"notion_errors={stats.get('errors', 0)} "
                f"outbox_pending={pending}"
            )
        print("SUMMARY " + " ".join(parts) + " " + notion_part)

    if failures:
        raise SystemExit("pipeline completed with failures: " + ", ".join(sorted(set(failures))))


if __name__ == "__main__":
    run()

