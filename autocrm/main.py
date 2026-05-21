"""AutoCRM entrypoint — run collectors to ingest into the local SQLite outbox."""

from __future__ import annotations

import logging

from autocrm import notion
from autocrm.collectors.beeper import BeeperCollector
from autocrm.collectors.imessage import IMessageCollector
from autocrm.collectors.phone import PhoneCallsCollector


def run() -> None:
    logging.basicConfig(level=logging.INFO, format="%(levelname)s %(message)s")
    log = logging.getLogger(__name__)

    failures: list[str] = []
    collectors = [IMessageCollector(), PhoneCallsCollector(), BeeperCollector()]

    for c in collectors:
        try:
            log.info("Collector start: %s", c.app)
            result = c.collect()
            log.info(
                "Collector done: %s (enqueued=%s cursor_before=%s cursor_after=%s)",
                c.app,
                result.enqueued,
                result.cursor_before,
                result.cursor_after,
            )
        except Exception:
            failures.append(c.app)
            log.exception("Collector failed: %s", c.app)

    if notion.notion_configured():
        try:
            log.info("Notion sync start")
            stats = notion.sync_outbox()
            log.info(
                "Notion sync done: applied=%s errors=%s",
                stats["applied"],
                stats["errors"],
            )
            if stats["errors"]:
                failures.append("notion")
        except Exception:
            failures.append("notion")
            log.exception("Notion sync failed")

    if failures:
        raise SystemExit(
            "pipeline completed with failures: " + ", ".join(sorted(set(failures)))
        )


if __name__ == "__main__":
    run()
