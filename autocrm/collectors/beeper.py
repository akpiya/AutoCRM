"""Beeper Desktop collector → outbox."""

from autocrm.collectors.collector import Collector, CollectResult


class BeeperCollector(Collector):
    app = "beeper"

    def collect(self) -> CollectResult:
        # TODO: call Beeper Desktop local API, enqueue events, advance cursor.
        return CollectResult(source=self.app, enqueued=0)
