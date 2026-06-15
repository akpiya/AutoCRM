# AutoCRM — agent instructions

AutoCRM is a **macOS-only** Go tool that records **inbound and outbound** communication activity (iMessage/SMS, phone/FaceTime calls, Beeper Desktop) into a local SQLite **outbox**, then syncs to a Notion people database when configured.

## Current milestone

**Ingest + Notion sync (Go):** collectors write the outbox and advance ingest cursors; `cmd/autocrm` runs `notion.SyncOutbox` if `NOTION_TOKEN` and `NOTION_DATABASE_ID` are set. Phone and Beeper collectors are stubs.

## Repository layout

| Path | Role |
|------|------|
| `cmd/autocrm/main.go` | Collectors, then Notion sync when configured. |
| `internal/outbox/` | Outbox schema, ingest batch, sync queue fetch/delete. |
| `internal/notion/` | Notion API + drain outbox (`net/http`). |
| `internal/common/` | `~/.autocrm` paths, Apple epoch, Notion constants. |
| `internal/collectors/` | iMessage, phone, Beeper collectors. |
| `internal/testfixtures/` | Minimal `chat.db` for tests. |
| `launchd/com.user.autocrm.plist.example` | LaunchAgent template. |
| `scripts/e2e_verify.sh` | `go test` + `go run ./cmd/autocrm`. |

Run tests: `go test ./...` from repo root.

## Architecture

```text
launchd → autocrm binary → collectors → outbox table
                              ↓
                    notion.SyncOutbox (if env set)
                              ↓
                         Notion people DB
```

## Outbox API (`internal/outbox`)

| Function | Purpose |
|----------|---------|
| `GetCursor` | Read ingest checkpoint |
| `IngestBatch` | Insert events + update cursor (one transaction) |
| `FetchAll` / `FetchBatch` | Read pending sync rows |
| `DeleteRows` | Remove processed rows |

## iMessage notes

- DB: `~/Library/Messages/chat.db` (read-only URI).
- Group chats: `;+;` in `chat_identifier` or `guid`; outbound fans out to `chat_handle_join` members.
- Apple `message.date` is nanoseconds since 2001-01-01 UTC — `common.AppleNsToUnix`.
- Bootstrap: no cursor → set cursor to `MAX(ROWID)` without backfill.
- **Full Disk Access** required on the binary (or Terminal for `go run`).

## Notion sync

- Only `NOTION_TOKEN` and `NOTION_DATABASE_ID` from environment.
- Property names and rate limits: `internal/common/common.go`.
- Parallel PATCHes: `NOTION_PATCH_WORKERS` goroutines with staggered starts.

## Verification

```bash
go test ./...
go run ./cmd/autocrm
./scripts/e2e_verify.sh
```

## Code style

- Stdlib-first; SQLite via `github.com/mattn/go-sqlite3` (CGO).
- No emojis in logs or commit messages.
- Keep diffs scoped.
