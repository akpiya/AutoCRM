# AutoCRM

AutoCRM is a small **personal CRM pipeline**: when you send messages or place calls from your Mac (iMessage/SMS, [Beeper](https://beeper.com/) Desktop, and macOS call history), the system records **who** you contacted and **when**, then updates a **Notion** database of people—so “last touched” stays accurate without manual logging.

Everything runs **on your Mac** while it is awake: collectors append JSON events to a local SQLite **outbox**, and `python3 -m autocrm.main` runs the end-to-end pipeline (ingest → Notion sync).

## Goals

- **Automatic contact logging** — Outbound chats become structured events (phones, emails, handles, channel, timestamp).
- **Reliability** — Events land in **`~/.autocrm/outbox.db`** first; **`notion-sync`** can run on its own schedule or after collectors.
- **Single source of truth in Notion** — One integration token and people database; property names are configurable via env vars.

## How it fits together

```
  iMessage/SMS  --+
  Beeper        --+--> ~/.autocrm/outbox.db --> notion-sync --> Notion API
  Phone/FaceTime-+
```

**Environment:** **`AUTOCRM_NOTION_TOKEN`**, **`AUTOCRM_NOTION_DATABASE_ID`**. Optional: **`AUTOCRM_NOTION_PHONES_PROP`**, **`AUTOCRM_NOTION_EMAILS_PROP`**, **`AUTOCRM_NOTION_LAST_CONTACTED_PROP`**, **`AUTOCRM_NOTION_LAST_CHANNEL_PROP`**, **`AUTOCRM_NOTION_MIN_INTERVAL`** (default `0.35` seconds between writes).

**`notion-sync`** deletes outbox rows after a successful update, or when dropping bad JSON, non-outbound, or unmatched events. Notion API errors leave rows **pending** for the next run.

## Repository layout

| Path | Purpose |
|------|--------|
| `autocrm/` | Python package: CLI, outbox, collectors, **`notion-sync`**. |
| `launchd/` | Example `launchd` agent for **`sync-all`** (collect + Notion drain). |
| `autocrm/README.md` | Module-level responsibilities. |
| `pyproject.toml` | Installable project (`pip install -e ".[dev]"`). |
| `scripts/` | `e2e_verify.sh`, `mac_call_history_dump.py`, etc. |

## Usage

From the **repository root**:

```bash
python -m pip install -e ".[dev]"
export AUTOCRM_NOTION_TOKEN="secret_..."
export AUTOCRM_NOTION_DATABASE_ID="your-database-id"
python3 -m autocrm.main
autocrm outbox-status
```

Beeper needs **`BEEPER_DESKTOP_TOKEN`** in the environment to include Beeper messages.

Phone/FaceTime call history collection requires **Full Disk Access** for the Python runner (macOS stores it under `~/Library/Application Support/CallHistoryDB/`).

State lives under **`~/.autocrm/`** (outbox DB, iMessage cursor, Beeper sync watermark, phone call cursor).

## Requirements

- **Mac** for iMessage (`chat.db`) and optional Beeper Desktop API.
- **Full Disk Access** (for Phone/FaceTime call history collection).
- **Python 3.11+**.
- **Notion** integration with a people-style database (date + rich text + phone/email-style properties matching your env or defaults).

---

AutoCRM **bridges messaging activity into Notion** so follow-ups and recency stay trustworthy.
