"""Phone / FaceTime call-history collector → outbox."""

from autocrm.collectors.collector import Collector, CollectResult


class PhoneCallsCollector(Collector):
    app = "phone_calls"

    def collect(self) -> CollectResult:
        # TODO: read CallHistory.storedata, enqueue outbound calls, advance cursor.
        return CollectResult(source=self.app, enqueued=0)
