# AutoCRM

AutoCRM is a macOS command-line app that keeps a Notion people database updated from your communication activity.

It reads recent iMessage/SMS and phone/FaceTime activity from your Mac, stores pending updates in a local SQLite outbox, and syncs matched people in Notion with:

- `Last Contacted`
- `Last Channel`

AutoCRM is designed to run quietly in the background with `launchd`.

## Current Status

Implemented:

- iMessage/SMS activity
- phone and FaceTime call activity
- Notion sync
- background installation with LaunchAgent

Not implemented yet:

- Beeper Desktop collection
- a graphical macOS app
- Apple notarization

Because AutoCRM is not notarized by Apple, macOS may warn you before running it. You can still use it, but you will need to approve it manually in System Settings.

## Requirements

- macOS
- a Notion account
- a Notion integration token
- a Notion people database
- Full Disk Access for the installed AutoCRM binary

You do not need Go installed if you download a release zip.

## 1. Create a Notion Integration

1. Open [Notion integrations](https://www.notion.so/my-integrations).
2. Create a new internal integration.
3. Copy the integration secret. AutoCRM will ask for this during install.

## 2. Create the Notion Database

Create or choose a Notion database for people.

AutoCRM requires these property names exactly:

| Property | Type | Notes |
|----------|------|-------|
| `Name` | Title | Your person's name. |
| `Phones` | Multi-select | One phone number per tag. Formatting can vary. |
| `Emails` | Email or Multi-select | Email matching is case-insensitive. |
| `Last Contacted` | Date | AutoCRM updates this when it sees newer activity. |
| `Last Channel` | Select | Include `Text`, `Phone`, and `Facetime` options. |

Then share the database with your Notion integration:

1. Open the database in Notion.
2. Click `...` in the top-right.
3. Choose `Connections`.
4. Add your integration.

Copy the database ID from the database URL. AutoCRM will ask for it during install.

## 3. Download AutoCRM

Download the correct zip from GitHub Releases:

- Apple Silicon Macs: `autocrm-macos-arm64.zip`
- Intel Macs: `autocrm-macos-amd64.zip`

Most M1, M2, M3, and M4 Macs use Apple Silicon.

Unzip the file. It contains:

```text
autocrm
README.txt
```

## 4. Install

Open Terminal in the unzipped folder and run:

```bash
./autocrm install
```

If macOS refuses to run the downloaded binary, open System Settings > Privacy & Security and approve AutoCRM. If Terminal still reports that the file is blocked, remove the download quarantine flag:

```bash
xattr -d com.apple.quarantine ./autocrm
```

The installer will:

1. Show the required Notion database setup.
2. Ask for your Notion integration token.
3. Ask for your Notion database ID.
4. Validate that Notion is reachable and has the required properties.
5. Copy AutoCRM to `~/.local/bin/autocrm`.
6. Write `~/Library/LaunchAgents/com.user.autocrm.plist`.
7. Guide you through enabling Full Disk Access.
8. Load the background LaunchAgent.

The default background sync interval is 5 minutes.

The Notion token and database ID are stored locally in:

```text
~/Library/LaunchAgents/com.user.autocrm.plist
```

## 5. Enable Full Disk Access

AutoCRM reads these local macOS databases:

- `~/Library/Messages/chat.db`
- `~/Library/Application Support/CallHistoryDB/CallHistory.storedata`

macOS blocks those files unless the exact executable has Full Disk Access.

During install, enable Full Disk Access for:

```text
~/.local/bin/autocrm
```

If AutoCRM cannot read Messages or call history, run:

```bash
~/.local/bin/autocrm doctor
```

## Commands

```bash
autocrm install
autocrm run
autocrm doctor
autocrm uninstall
autocrm version
```

`autocrm run` runs one collector/sync pass. The LaunchAgent uses this command in the background.

`autocrm doctor` checks:

- Notion credentials
- required Notion database properties
- Messages database readability
- call history database readability
- local outbox initialization
- LaunchAgent presence

`autocrm uninstall` unloads and removes the LaunchAgent. It asks before deleting `~/.local/bin/autocrm` and keeps `~/.autocrm` data by default.

## Logs

Background logs are written to:

```text
/tmp/autocrm.log
/tmp/autocrm.err
```

## Behavior

AutoCRM keeps local state under:

```text
~/.autocrm/
```

The outbox database is:

```text
~/.autocrm/outbox.db
```

First run behavior:

- iMessage bootstraps to the current max message row without backfilling old messages.
- phone calls bootstrap to the current max call row without backfilling old calls.
- later runs process new activity incrementally.

Notion sync behavior:

- unmatched outbox rows are removed silently
- `Last Contacted` only moves forward
- phone matching strips non-digits and handles US numbers with or without leading `1`
- channel labels are `Text`, `Phone`, and `Facetime`

## Troubleshooting

Run:

```bash
~/.local/bin/autocrm doctor
```

Common issues:

| Problem | Fix |
|---------|-----|
| Notion validation fails | Confirm the token, database ID, database sharing, and required properties. |
| Messages database fails | Enable Full Disk Access for `~/.local/bin/autocrm`. |
| Call history database fails | Enable Full Disk Access for `~/.local/bin/autocrm`. |
| No Notion pages update | Confirm people have matching values in `Phones` or `Emails`. |
| macOS blocks the binary | Open System Settings > Privacy & Security and approve AutoCRM. |

## Building From Source

Developer requirements:

- Go 1.22+
- CGO support
- SQLite driver dependencies for `github.com/mattn/go-sqlite3`

Build:

```bash
go build -o ./bin/autocrm ./cmd/autocrm
```

Run tests:

```bash
go test ./...
```

Package release zips:

```bash
scripts/package_release.sh v0.1.0
```

This writes:

```text
dist/autocrm-macos-arm64.zip
dist/autocrm-macos-amd64.zip
```

## Repository Layout

| Path | Purpose |
|------|---------|
| `cmd/autocrm/` | CLI entrypoint and install/doctor/uninstall commands. |
| `internal/collectors/` | iMessage, phone, and Beeper collectors. |
| `internal/outbox/` | SQLite outbox and ingest cursors. |
| `internal/notion/` | Notion API integration and outbox sync. |
| `internal/common/` | shared paths, constants, and time helpers. |
| `scripts/package_release.sh` | release zip builder. |

## Privacy

AutoCRM reads local communication metadata needed to identify contact activity. It stores pending sync rows locally in SQLite and sends matched updates to the Notion integration you configure.

AutoCRM does not include a server component.
