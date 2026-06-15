# AutoCRM

AutoCRM records communication activity on your Mac into a local SQLite **outbox** (`~/.autocrm/outbox.db`), then syncs matched contacts to a Notion people database. The iMessage collector is implemented; phone and Beeper collectors are stubs.

Implemented in **Go** (module `github.com/akpiya/autocrm`).

## Layout

| Path | Purpose |
|------|---------|
| `internal/` | App implementation: `common`, `outbox`, `notion`, `collectors`. |
| `cmd/autocrm/` | Main pipeline binary. |
| `scripts/import_imessage_notion.py` | Interactive vCard → Notion import (Python). |
| `launchd/` | LaunchAgent template (`.plist.example`). |
| `go.mod` | Go module definition. |

## Usage

```bash
go test ./...
export NOTION_TOKEN="secret_..."
export NOTION_DATABASE_ID="your-database-id"
go run ./cmd/autocrm
```

Or build and install:

```bash
go build -o ~/.local/bin/autocrm ./cmd/autocrm
~/.local/bin/autocrm
```

`cmd/autocrm` runs collectors, then drains the outbox to Notion when both env vars are set.

State lives under **`~/.autocrm/`** (outbox DB at `outbox.db`, per-source cursors). The `outbox` table is a pending sync queue; rows are deleted after processing (matched or unmatched).

### Notion environment variables

| Variable | Required |
|----------|----------|
| `NOTION_TOKEN` | Yes (for sync) |
| `NOTION_DATABASE_ID` | Yes (for sync) |

Property names, rate-limit interval, and parallel PATCH worker count live in [`internal/common/common.go`](internal/common/common.go). Change those constants if your Notion schema differs.

Unmatched outbox rows (no Notion page with that phone/email) are removed silently. **Last Contacted** only moves forward when the outbox event is newer than the value on the page.

**Multiple phones or emails per person:** use **Multi-select** on the **Phones** and **Emails** columns. AutoCRM matches if the iMessage handle equals **any** value on that page.

**Phone formatting:** tags can be `+1 703-395-5764`, `(703) 395-5764`, `7033955764`, etc. Matching strips non-digits and compares US numbers with or without a leading `1`.

**Last Channel:** use a Notion **Select** property; include an option named **Text** (outbox platform `text` is written as **Text**).

### vCard import

```bash
python3 scripts/import_imessage_notion.py path/to/contacts.vcf
```

## Full Disk Access

Reading `~/Library/Messages/chat.db` requires **Full Disk Access** on macOS (per executable).

| How you run | What to add in FDA |
|-------------|-------------------|
| `~/.local/bin/autocrm` (recommended) | That binary |
| `go run ./cmd/autocrm` (dev) | Terminal or your IDE |

System Settings → Privacy & Security → Full Disk Access → **+** → choose the binary.

### Scheduled runs (launchd)

```bash
go build -o ~/.local/bin/autocrm ./cmd/autocrm
cp launchd/com.user.autocrm.plist.example launchd/com.user.autocrm.plist
# Edit NOTION_* and binary path, then:
cp launchd/com.user.autocrm.plist ~/Library/LaunchAgents/
launchctl bootout gui/$(id -u)/com.user.autocrm 2>/dev/null || true
launchctl load ~/Library/LaunchAgents/com.user.autocrm.plist
```

`launchd/com.user.autocrm.plist` is gitignored so secrets are not committed.

## Requirements

- **macOS** for iMessage / future call-history collectors.
- **Go 1.22+** and CGO (for `github.com/mattn/go-sqlite3`).

See **`internal/README.md`** for package layout. Migration notes: **`MIGRATION_GO.md`**.
