"""Collector interface."""


from abc import ABC, abstractmethod
from dataclasses import dataclass

@dataclass(frozen=True)
class CollectResult:
    source: str
    enqueued: int
    cursor_before: str | int | float | None = None
    cursor_after: str | int | float | None = None

class Collector(ABC):
    """All outbox ingestors must implement this interface."""

    app: str

    @abstractmethod
    def collect(self) -> CollectResult: ...

