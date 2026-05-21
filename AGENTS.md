# AutoCRM — agent instructions

AutoCRM is a **macOS-only** Python tool that records **outbound** communication activity (iMessage/SMS, phone/FaceTime calls, Beeper Desktop) into a local SQLite **outbox**, then syncs to a Notion people database when configured.

This file is the shared briefing for Cursor Agent sessions. Prefer updating it when architecture or milestones change rather than re-explaining the project in chat.

## Current milestone

**Ingest + Notion sync:** collectors write the outbox and advance ingest cursors; `main.py` then runs `notion.sync_outbox()` if `NOTION_TOKEN` and `NOTION_DATABASE_ID` are set. Phone and Beeper collectors are still stubs.

## Repository layout

| Path | Role |
|------|------|
| `autocrm/main.py` | Collectors, then Notion sync when configured. |
| `autocrm/outbox.py` | Outbox schema, ingest batch, sync queue fetch/delete. |
| `autocrm/notion.py` | Notion API + drain outbox (stdlib HTTP). |
| `autocrm/common.py` | `~/.autocrm` paths (`OUTBOX_DB_PATH`). |
| `autocrm/collectors/collector.py` | `Collector` ABC and `CollectResult` dataclass. |
| `autocrm/collectors/imessage.py` | iMessage collector → outbox. |
| `autocrm/collectors/imessage_db.py` | Read outbound rows from `chat.db`. |
| `autocrm/collectors/phone.py` | Call history — read `CallHistory.storedata`. |
| `autocrm/collectors/beeper.py` | Beeper Desktop local API. |
| `launchd/com.user.autocrm.plist.example` | LaunchAgent template; real `com.user.autocrm.plist` is gitignored. |
| `scripts/e2e_verify.sh` | Install editable package, pytest (if any), run main. |
| `README.md`, `autocrm/README.md` | Human-oriented docs; keep in sync with behavior. |

Tests under `tests/`; run `pytest` from repo root.

## Runtime and setup

- **Python 3.11+**, stdlib only (no runtime dependencies in `pyproject.toml`).
- Install for development: `pip install -e ".[dev]"`.
- Run pipeline: `python3 -m autocrm.main`.
- Local state directory: **`~/.autocrm/`** (created on demand).
  - Outbox DB: `~/.autocrm/outbox.db`

## Architecture

```text
launchd → autocrm.main → collectors → outbox table
                              ↓
                    notion.sync_outbox (if env set)
                              ↓
                         Notion people DB
```

- Collectors run **serially** in fixed order: iMessage → phone → Beeper (`main.py`).
- One collector failure is logged; others still run; process exits with failure if any failed.
- Each collector implements `collect() -> CollectResult` and sets `app` to a stable source id (e.g. `"imessage"`).

## Outbox schema and API

Tables (see `autocrm/outbox.py`):

- **`outbox`** — communication events: `platform`, `party_id`, `direction` (int), `created_at` (Unix timestamp as `REAL`).
- **`cursor`** — per-source ingest position: `source` (PK), `cursor_value`, `updated_at`.

**API** (`autocrm/outbox.py`):

| Function | Purpose |
|----------|---------|
| `get_cursor(source)` | Read ingest checkpoint for a collector |
| `ingest_outbox_batch(source, events, cursor_value)` | Insert outbox rows and update ingest cursor in one transaction |
| `fetch_outbox_batch(limit)` | Read pending sync rows (FIFO by `id`) |
| `delete_outbox_rows(ids)` | Remove processed rows from the sync queue |
| `init_db` | Create tables if missing (called internally) |

The **`outbox` table** is the pending Notion sync queue. Rows are deleted after sync (matched or unmatched). Ingest cursors live in the **`cursor`** table.

Collectors should:

1. Read cursor with `get_cursor(self.app)`.
2. Fetch only outbound rows after the cursor from the Mac data source.
3. Insert events and update cursor with `ingest_outbox_batch`.
4. Use a monotonic cursor (e.g. message `ROWID` for iMessage).

Use `db_path=OUTBOX_DB_PATH` from `autocrm.common` unless testing with a temp DB.

## Collector implementation notes

### iMessage (`imessage`)

- DB: `~/Library/Messages/chat.db` (read-only URI: `file:...?mode=ro`). Query logic in `collectors/imessage_db.py`.
- Ingest **inbound and outbound** messages after the cursor (`message` → `chat_message_join` → `chat`).
- **Group chats** (`chat_identifier` or `guid` contains `;+;`): outbound fans out to every handle in `chat_handle_join`; inbound credits the sender only.
- **1:1 chats**: one outbox row per message; `party_id` is the other party’s handle (`handle.id`).
- `message.date` is **nanoseconds since 2001-01-01 UTC** (Apple epoch). Convert with `apple_ns_to_unix` in `common.py`.
- Outbox rows use `platform="text"` (`PLATFORM_TEXT`); `direction` is `DIRECTION_INBOUND` (0) or `DIRECTION_OUTBOUND` (1).
- Cursor: max `message.ROWID` processed, stored via `outbox.ingest_outbox_batch` in the same transaction as event inserts.
- If no ingest cursor exists yet, skip ingest and set cursor to `MAX(ROWID)` in `chat.db` (no historical backfill).
- No dedupe key; duplicate outbox rows on retry are acceptable.
- macOS may require **Full Disk Access** for Terminal/Python to read `chat.db`.

**Tests:** `pytest` from repo root (minimal `chat.db` fixture under `tests/fixtures/`).

### Phone (`phone_calls`)

- Source: macOS Call History (`CallHistory.storedata` under `~/Library/Application Support/`).
- Track **outbound** calls only; advance cursor by call id or timestamp.

### Beeper (`beeper`)

- Use Beeper Desktop’s **local** API; persist cursor in SQLite (not a separate JSON file unless you have a good reason).
- Map chats to `party_id` / platform fields consistently with other collectors.

### Notion sync (`notion.py`)

- Runs automatically at end of `main.py` when `notion_configured()`.
- Match `party_id` to Notion **Phones** / **Emails** (phone if no `@`, else email). Phones match across `+1`, parentheses, hyphens, and 10- vs 11-digit US forms.
- Apply updates for both inbound and outbound outbox rows (monotonic **Last Contacted**).
- Group pending rows by matched page; PATCH with max `created_at` and `platform` as Last Channel.
- Unmatched rows: delete from outbox silently (no log).
- Notion API errors: leave affected rows in outbox for retry; exit non-zero if `errors > 0`.

## Code style

Readability over cleverness. Prefer simple, linear code that a future reader can follow without hunting through abstractions.

**Comments**

- Do not add comments that restate what the code already says.
- Comment only non-obvious business rules or external constraints (e.g. Apple epoch units, macOS permissions).
- No commented-out dead code; delete it.

**Naming and structure**

- Clear names over abbreviations (`cursor_value`, not `cv`).
- Small functions and modules; one main idea per file when practical.
- `from __future__ import annotations` and type hints on public functions and non-trivial locals.
- Stdlib-first; avoid new dependencies unless explicitly requested.

**Logging and output**

- Use `logging` in library/collector code; plain `print` only for CLI-style scripts if needed.
- Log messages are plain text: no emojis in code, comments, logs, or commit messages for this repo.
- Agent responses about this project should not use emojis.

**Changes**

- Keep diffs scoped (one collector or one concern at a time).
- Do not reintroduce V0-style JSON payload outbox blobs unless explicitly requested.
- `CollectResult` fields: `source`, `enqueued`, optional `cursor_before` / `cursor_after` for observability.

## What not to do unless asked

- Add Notion/CRM sync, web UI, or cloud services.
- Commit secrets (`.env`, API tokens) or expand `.gitignore` exceptions.
- Force-push `main` or skip git hooks.
- Treat `test.ipynb` as source of truth — it may reference **removed** V0 APIs (e.g. `_fetch_outbound_rows_since`, `APPLE_EPOCH_UTC` in `common`). Update the notebook or delete stale cells when rebuilding collectors.

## Prior implementation (reference only)

Commit **`9ec2450`** (`V0 Pass of AutoCRM`) contained working collectors, `ContactEvent` / `PersonHints` models, JSON sidecar cursors, and `notion_sync.py`. The tree was reset to a **clean scaffold** in **`5192534`** — simpler outbox columns, stub collectors, no `models.py` or `notion_sync.py`.

When porting logic from V0:

- Adapt to the **current** flat outbox columns (`platform`, `party_id`, `direction`, `created_at`), not V0’s JSON blob shape, unless the schema is deliberately extended.
- Prefer **SQLite cursors** (`get_cursor` / `ingest_outbox_batch`) over separate `*_cursor.json` files unless a source requires otherwise.
- Inspect V0 with: `git show 9ec2450:autocrm/collectors/imessage.py` (and siblings).

## Verification

```bash
pip install -e ".[dev]"
python3 -m autocrm.main          # should complete; stubs log enqueued=0
./scripts/e2e_verify.sh          # install + pytest + main
```

After implementing a collector, manually verify on a Mac with real data: run main twice and confirm `enqueued` is 0 on the second run and cursor advances on the first.

## Suggested work order

1. **iMessage** — highest value; well-defined DB schema; use read-only SQLite.
2. **Phone** — call history dump/script existed in V0 (`scripts/mac_call_history_dump.py` in that commit).
3. **Beeper** — depends on local API availability.
4. Tests under `tests/` for outbox cursor idempotency and collector edge cases (empty DB, missing files).
5. Later: CRM sync layer consuming `outbox` (design TBD).

## Docs to update when behavior changes

- Root `README.md` — user-facing overview.
- `autocrm/README.md` — package/module map.
- This **`AGENTS.md`** — milestones, schema, and agent-facing constraints.
