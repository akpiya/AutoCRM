# `autocrm` package

Collectors read outbound activity from local sources and append rows to a SQLite outbox at `~/.autocrm/outbox.db`. Notion sync is out of scope for now.

## Layout

| Module | Role |
|--------|------|
| **`main.py`** | Entrypoint: run collectors serially (iMessage → phone → Beeper). |
| **`outbox.py`** | SQLite outbox + per-source cursors. |
| **`common.py`** | `~/.autocrm` paths. |
| **`collectors/collector.py`** | `Collector` interface and `CollectResult`. |
| **`collectors/imessage.py`** | iMessage/SMS stub. |
| **`collectors/phone.py`** | Phone/FaceTime call-history stub. |
| **`collectors/beeper.py`** | Beeper Desktop stub. |

## Usage

```bash
python3 -m autocrm.main
```
