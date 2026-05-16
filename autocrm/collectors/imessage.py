"""iMessage/SMS collector → outbox."""

from autocrm.collectors.collector import Collector, CollectResult


class IMessageCollector(Collector):
    app = "imessage"

    def collect(self) -> CollectResult:
        # TODO: read ~/Library/Messages/chat.db, enqueue events, advance cursor.
        return CollectResult(source=self.app, enqueued=0)
