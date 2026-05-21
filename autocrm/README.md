# `autocrm` package

Collectors read outbound activity from local sources and append rows to a SQLite outbox at `~/.autocrm/outbox.db`. `main.py` runs `notion.sync_outbox()` after collectors when Notion env vars are set.

## Layout

| Module | Role |
|--------|------|
| **`main.py`** | Entrypoint: run collectors serially (iMessage → phone → Beeper). |
| **`outbox.py`** | SQLite outbox + ingest cursors + sync queue helpers. |
| **`notion.py`** | Match `party_id` to Notion pages; update Last Contacted / Last Channel. |
| **`common.py`** | Paths, Apple epoch conversion, `PLATFORM_TEXT`, `DIRECTION_INBOUND` / `DIRECTION_OUTBOUND`. |
| **`collectors/collector.py`** | `Collector` interface and `CollectResult`. |
| **`collectors/imessage_db.py`** | Query messages from `chat.db` (1:1 and group). |
| **`collectors/imessage.py`** | iMessage collector → outbox (`platform="text"`). |
| **`collectors/phone.py`** | Phone/FaceTime call-history stub. |
| **`collectors/beeper.py`** | Beeper Desktop stub. |

## Usage

```bash
python3 -m pip install -e ".[dev]"
pytest
python3 -m autocrm.main
```
