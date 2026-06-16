# AutoCRM

AutoCRM records communication activity on your Mac into a local SQLite **outbox** (`~/.autocrm/outbox.db`), then syncs matched contacts to a Notion people database. iMessage and phone call collectors are implemented; Beeper remains a stub.

Implemented in **Go** (module `github.com/akpiya/autocrm`).

## Quickstart

### 1. Build

```bash
go build -o ~/.local/bin/autocrm ./cmd/autocrm
```

### 2. Grant Full Disk Access

AutoCRM reads:

- `~/Library/Messages/chat.db` (iMessage/SMS)
- `~/Library/Application Support/CallHistoryDB/CallHistory.storedata` (phone/FaceTime call history)

macOS checks Full Disk Access on the process that opens those files:

- terminal runs: grant FDA to the terminal app (Terminal, iTerm, Alacritty, Cursor, etc.)
- launchd runs: grant FDA to `~/.local/bin/autocrm`

After toggling FDA, fully restart the app (or log out/in) before retrying.

### 3. Set up your Notion database

Create a Notion database with these properties (names must match exactly):

| Property | Type | Notes |
|----------|------|-------|
| **Phones** | Multi-select | One tag per phone number, any format |
| **Emails** | Email or Multi-select | |
| **Last Contacted** | Date | |
| **Last Channel** | Select | Include **Text**, **Phone**, and **Facetime** options |

Then create a [Notion integration](https://www.notion.so/my-integrations), share the database with it, and grab the token and database ID.

### 4. Run

Without Notion (collector + outbox only):

```bash
~/.local/bin/autocrm
```

With Notion sync:

```bash
export NOTION_TOKEN="secret_..."
export NOTION_DATABASE_ID="your-database-id"
~/.local/bin/autocrm
```

The first run bootstraps the cursor to `MAX(ROWID)` without backfilling old messages. Subsequent runs pick up new messages incrementally.

### 5. Import existing contacts (one-time, optional)

If you have a vCard export of your contacts:

```bash
python3 scripts/import_imessage_notion.py path/to/contacts.vcf
```

This walks each contact interactively and creates Notion pages for the ones you choose. Same `NOTION_TOKEN` / `NOTION_DATABASE_ID` env vars required.

### 6. Schedule with launchd (optional)

```bash
cp launchd/com.user.autocrm.plist.example launchd/com.user.autocrm.plist
# Edit NOTION_TOKEN, NOTION_DATABASE_ID, and binary path in the plist, then:
cp launchd/com.user.autocrm.plist ~/Library/LaunchAgents/
launchctl bootout gui/$(id -u)/com.user.autocrm 2>/dev/null || true
launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/com.user.autocrm.plist
```

The gitignored copy keeps your secrets out of the repo.

---

## Layout

| Path | Purpose |
|------|---------|
| `internal/` | App implementation: `common`, `outbox`, `notion`, `collectors`. |
| `cmd/autocrm/` | Main pipeline binary. |
| `scripts/import_imessage_notion.py` | Interactive vCard → Notion import (Python). |
| `launchd/` | LaunchAgent template (`.plist.example`). |
| `go.mod` | Go module definition. |

## Notion sync details

State lives under **`~/.autocrm/`** (outbox DB at `outbox.db`, per-source cursors). The `outbox` table is a pending sync queue; rows are deleted after processing.

- **Unmatched rows** (no Notion page with that phone/email) are removed silently.
- **Last Contacted** only moves forward — an older outbox event won't overwrite a newer value on the page.
- **Multiple phones or emails per person:** use Multi-select on the Phones and Emails columns. AutoCRM matches if the iMessage handle equals any value on that page.
- **Phone formatting:** tags can be `+1 703-395-5764`, `(703) 395-5764`, `7033955764`, etc. Matching strips non-digits and compares US numbers with or without a leading `1`.

Property names, rate-limit interval, and parallel PATCH worker count live in [`internal/common/common.go`](internal/common/common.go). Change those constants if your Notion schema differs.

## Phone calls collector behavior

- Source DB: `~/Library/Application Support/CallHistoryDB/CallHistory.storedata`.
- Event scope: connected inbound/outbound calls only.
- Included sources: cellular + FaceTime (audio/video).
- Excluded in this version: missed/unanswered calls.
- First run bootstrap: if no `phone_calls` cursor exists, cursor is set to current max call row id without backfill.

## Requirements

- **macOS** for iMessage / future call-history collectors.
- **Go 1.22+** and CGO (for `github.com/mattn/go-sqlite3`).

See **`internal/README.md`** for package layout.
