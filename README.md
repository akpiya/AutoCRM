# AutoCRM

AutoCRM records outbound communication activity on your Mac into a local SQLite **outbox** (`~/.autocrm/outbox.db`), then syncs matched contacts to a Notion people database. The iMessage collector is implemented; phone and Beeper collectors are still stubs.

## Layout

| Path | Purpose |
|------|---------|
| `autocrm/` | Python package: `main`, outbox, collectors, `notion.py`. |
| `launchd/` | LaunchAgent template (`.plist.example`); copy to gitignored `com.user.autocrm.plist`. |
| `pyproject.toml` | Installable project (`pip install -e ".[dev]"`). |

## Usage

```bash
python -m pip install -e ".[dev]"
export NOTION_TOKEN="secret_..."
export NOTION_DATABASE_ID="your-database-id"
python3 -m autocrm.main
```

`main` runs collectors, then drains the outbox to Notion when both env vars are set.

State lives under **`~/.autocrm/`** (outbox DB at `outbox.db`, per-source cursors). The `outbox` table is a pending sync queue; rows are deleted after processing (matched or unmatched).

### Notion environment variables

| Variable | Required | Default |
|----------|----------|---------|
| `NOTION_TOKEN` | Yes (for sync) | — |
| `NOTION_DATABASE_ID` | Yes (for sync) | — |
| `NOTION_PHONES_PROP` | No | `Phones` |
| `NOTION_EMAILS_PROP` | No | `Emails` |
| `NOTION_LAST_CONTACTED_PROP` | No | `Last Contacted` |
| `NOTION_LAST_CHANNEL_PROP` | No | `Last Channel` |
| `NOTION_MIN_INTERVAL` | No | `0.35` (seconds between page updates) |

Unmatched outbox rows (no Notion page with that phone/email) are removed silently. **Last Contacted** only moves forward when the outbox event is newer than the value on the page.

**Multiple phones or emails per person:** use **Multi-select** on the **Phones** and **Emails** columns and add every number/address you message from. AutoCRM matches if the iMessage handle equals **any** value on that page. Notion’s single **Phone** or **Email** field types only store one value per column.

**Phone formatting:** tags can be `+1 703-395-5764`, `(703) 395-5764`, `7033955764`, etc. Matching strips non-digits and compares US numbers with or without a leading `1`.

**Last Channel:** use a Notion **Select** property; include an option named **Text** (outbox platform `text` is written as **Text**).

## Full Disk Access and deployment

Reading `~/Library/Messages/chat.db` requires **Full Disk Access** on macOS. FDA is granted per **executable**, not per Python package—so granting FDA to Terminal, Cursor, or a shared `python3` applies to everything run from that binary.

**Planned approach:** ship a **frozen single-purpose binary** (e.g. PyInstaller or py2app) used only for ingest, e.g. `autocrm-ingest`, and point launchd at that binary. Full Disk Access would be granted **only** to that executable—not to Python in general or your IDE. Production scheduling would use the frozen binary.

**Interim:** grant FDA to **Terminal** and/or **Cursor** (whichever you use to run `python3 -m autocrm.main`). System Settings → Privacy & Security → Full Disk Access → add the app → quit and reopen it. That is enough for local development and manual runs until the frozen binary exists.

### Scheduled runs (launchd)

```bash
cp launchd/com.user.autocrm.plist.example launchd/com.user.autocrm.plist
# Edit launchd/com.user.autocrm.plist: Python path, NOTION_* (gitignored)
cp launchd/com.user.autocrm.plist ~/Library/LaunchAgents/
launchctl unload ~/Library/LaunchAgents/com.user.autocrm.plist 2>/dev/null || true
launchctl load ~/Library/LaunchAgents/com.user.autocrm.plist
```

`launchd/com.user.autocrm.plist` is gitignored so secrets are not committed. Only `com.user.autocrm.plist.example` is tracked.

## Requirements

- **Mac** for future iMessage / call-history collectors.
- **Python 3.11+**.

See **`autocrm/README.md`** for module-level detail.
