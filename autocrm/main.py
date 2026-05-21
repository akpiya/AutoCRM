"""AutoCRM entrypoint — run collectors to ingest into the local SQLite outbox."""

from __future__ import annotations

import logging
import time

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
            t0 = time.perf_counter()
            result = c.collect()
            log.info(
                "Collector done: %s in %.2fs (enqueued=%s cursor_before=%s cursor_after=%s)",
                c.app,
                time.perf_counter() - t0,
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
            t0 = time.perf_counter()
            stats = notion.sync_outbox()
            log.info(
                "Notion sync done in %.2fs (pending=%s applied=%s errors=%s)",
                time.perf_counter() - t0,
                stats.get("pending", 0),
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
