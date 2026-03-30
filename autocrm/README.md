# `autocrm` package

This directory is the **AutoCRM** stack: outbound messages become JSON **`ContactEvent`** rows in SQLite, then **`notion-sync`** updates your Notion people database while the Mac is online.

---

## 1. Collectors — “What happened in chat?”

**Role:** Read **new outbound** activity from a source, build a `ContactEvent` (see `models.py`), and **append** rows to the outbox.

| Piece | Responsibility |
|--------|----------------|
| **`imessage.py`** | Opens macOS **Messages** SQLite (`chat.db`), walks messages **after** a saved cursor (`~/.autocrm/imessage_cursor.json`), maps rows to phones/handles and channel (`imessage` vs `sms`), writes events to the outbox, advances the cursor. |
| **`beeper.py`** | Calls **Beeper Desktop**’s local API (inline HTTP client), fetches messages newer than `~/.autocrm/beeper_sync.json`, derives `PersonHints` from chat participants, enqueues events, updates the watermark. |
| **`phone.py`** | Reads macOS call history (Phone / FaceTime / Continuity) from `~/Library/Application Support/CallHistoryDB/`, walks calls **after** a saved cursor (`~/.autocrm/phone_sync.json`), enqueues outbound call events. Requires **Full Disk Access**. |
| **`common.py`** | Shared helpers (time conversion + `~/.autocrm` paths). |

**Entrypoint:** Run everything via `python3 -m autocrm.main` (serial collectors + Notion drain).

Collectors **do not** call Notion; they only enqueue JSON.

---

## 2. Outbox & paths — “Durable queue on disk”

**Role:** **SQLite** so events survive sleep, offline use, or failed Notion syncs. Read by **`notion_sync`**.

| Piece | Responsibility |
|--------|----------------|
| **`outbox.py`** | **`enqueue`** (optional **`dedupe_id`**), **`fetch_pending_batch`**, **`delete_row`** after successful downstream handling. |
| **`models.py`** | **`ContactEvent`** / **`PersonHints`**: JSON shape consumed by **`notion_sync`**. |
| **`common.py`** | **`~/.autocrm`** paths: outbox DB, iMessage cursor, Beeper sync state. |

**CLI:** `autocrm outbox-status`.

---

## 3. Notion sync — “Apply outbox → Notion”

**Role:** Load all pages in your people database once per run, match **phones/emails** from each event, **PATCH** **Last contacted** / **Last channel** when the event time is newer than Notion’s current value; **delete** outbox rows that were applied or intentionally skipped (bad JSON, non-outbound, no match). Notion API failures leave rows **pending** for retry.

| Piece | Responsibility |
|--------|----------------|
| **`notion_sync.py`** | Env-based config (`AUTOCRM_NOTION_*`), urllib Notion API client. |

**Entrypoint:** `python3 -m autocrm.main`.

---

## How they chain

```
  imessage.py / beeper.py / phone.py  --->  outbox.py (+ models.py, common.py)  --->  notion_sync.py  --->  Notion
         ^                              ^
         |                              |
    common.py
```

Repository overview: **`README.md`** at the repo root.
