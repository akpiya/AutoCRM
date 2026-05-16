"""Outbox collectors (ingestors)."""

from autocrm.collectors.collector import Collector, CollectResult
from autocrm.collectors.beeper import BeeperCollector
from autocrm.collectors.imessage import IMessageCollector
from autocrm.collectors.phone import PhoneCallsCollector

__all__ = [
    "Collector",
    "CollectResult",
    "IMessageCollector",
    "BeeperCollector",
    "PhoneCallsCollector",
]

