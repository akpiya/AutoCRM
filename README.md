# AutoCRM

AutoCRM records outbound communication activity on your Mac into a local SQLite **outbox** (`~/.autocrm/outbox.db`). The iMessage collector is implemented; phone and Beeper collectors are still stubs. The first milestone is reliable local ingest.

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

State lives under **`~/.autocrm/`** (outbox DB at `outbox.db`, per-source cursors).

## Full Disk Access and deployment

Reading `~/Library/Messages/chat.db` requires **Full Disk Access** on macOS. FDA is granted per **executable**, not per Python package—so granting FDA to Terminal, Cursor, or a shared `python3` applies to everything run from that binary.

**Planned approach:** ship a **frozen single-purpose binary** (e.g. PyInstaller or py2app) used only for ingest, e.g. `autocrm-ingest`, and point launchd at that binary. Full Disk Access would be granted **only** to that executable—not to Python in general or your IDE. Production scheduling would use the frozen binary.

**Interim:** grant FDA to **Terminal** and/or **Cursor** (whichever you use to run `python3 -m autocrm.main`). System Settings → Privacy & Security → Full Disk Access → add the app → quit and reopen it. That is enough for local development and manual runs until the frozen binary exists. If you use the example launchd agent with `/usr/bin/python3`, that interpreter needs FDA separately unless launchd is pointed at the same environment you use interactively.

## Requirements

- **Mac** for future iMessage / call-history collectors.
- **Python 3.11+**.

See **`autocrm/README.md`** for module-level detail.
