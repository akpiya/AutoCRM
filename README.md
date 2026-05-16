# AutoCRM

AutoCRM records outbound communication activity on your Mac into a local SQLite **outbox** (`~/.autocrm/outbox.db`). Collectors for iMessage/SMS, Beeper Desktop, and phone call history are stubbed for now; the first milestone is reliable local ingest.

## Layout

| Path | Purpose |
|------|---------|
| `autocrm/` | Python package: `main`, outbox, collector stubs. |
| `launchd/` | Example agent that runs `python3 -m autocrm.main` every 2 minutes. |
| `pyproject.toml` | Installable project (`pip install -e ".[dev]"`). |

## Usage

```bash
python -m pip install -e ".[dev]"
python3 -m autocrm.main
```

State lives under **`~/.autocrm/`** (outbox DB and per-source cursors).

## Requirements

- **Mac** for future iMessage / call-history collectors.
- **Python 3.11+**.

See **`autocrm/README.md`** for module-level detail.
