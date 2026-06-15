# Internal packages

Application implementation for AutoCRM (not importable outside this module).

| Package | Role |
|---------|------|
| `common/` | Paths, Apple epoch, Notion constants. |
| `outbox/` | SQLite outbox + cursors. |
| `notion/` | Notion API + sync. |
| `collectors/` | iMessage, phone, Beeper ingest. |
| `testfixtures/` | Fake `chat.db` for tests only. |

Entrypoint: `cmd/autocrm`.
