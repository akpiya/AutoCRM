# `autocrm` package

Collectors read outbound activity from local sources and append rows to a SQLite outbox at `~/.autocrm/outbox.db`. Notion sync is out of scope for now.

## Layout

| Module | Role |
|--------|------|
| **`main.py`** | Entrypoint: run collectors serially (iMessage → phone → Beeper). |
| **`outbox.py`** | SQLite outbox + per-source cursors. |
| **`common.py`** | Paths, Apple epoch conversion, `PLATFORM_TEXT`, `DIRECTION_OUTBOUND`. |
| **`collectors/collector.py`** | `Collector` interface and `CollectResult`. |
| **`collectors/imessage_db.py`** | Query outbound rows from `chat.db`. |
| **`collectors/imessage.py`** | iMessage collector → outbox (`platform="text"`). |
| **`collectors/phone.py`** | Phone/FaceTime call-history stub. |
| **`collectors/beeper.py`** | Beeper Desktop stub. |

## Usage

```bash
python3 -m pip install -e ".[dev]"
pytest
python3 -m autocrm.main
```
