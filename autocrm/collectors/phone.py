"""
Phone / FaceTime / Continuity call-history collector → outbox.

Requires Full Disk Access to read:
  ~/Library/Application Support/CallHistoryDB/
"""

from __future__ import annotations

import sqlite3
from datetime import datetime, timedelta, timezone
from pathlib import Path

from autocrm import outbox
from autocrm.collectors.base import CollectResult, Collector
from autocrm.common import data_dir, load_json_state, outbox_db, save_json_state
from autocrm.models import ContactEvent, PersonHints

APPLE_EPOCH_UTC = datetime(2001, 1, 1, 0, 0, 0, tzinfo=timezone.utc)


class PhoneCallsCollector(Collector):
    app = "phone_calls"
    outbox_db_path: Path = outbox_db()
    phone_db_path: Path | None = None  # discovered at runtime
    phone_cursor_path: Path = data_dir() / "phone_sync.json"

    def __init__(self, phone_db_path: Path | None = None) -> None:
        if phone_db_path is not None:
            self.phone_db_path = phone_db_path

    def collect(self) -> CollectResult:
        call_db = self.phone_db_path or _find_callhistory_db()
        if not call_db or not call_db.exists():
            raise SystemExit(
                "Call history DB not found under ~/Library/Application Support/CallHistoryDB"
            )

        cursor_path = self.phone_cursor_path
        state = load_json_state(cursor_path, default={"last_call_zdate": 0.0})
        last_zdate = float(state.get("last_call_zdate") or 0.0)

        ob = self.outbox_db_path
        outbox.init_db(ob)

        uri = f"file:{call_db}?mode=ro"
        conn = sqlite3.connect(uri, uri=True)
        conn.row_factory = sqlite3.Row
        try:
            table, colmap = _pick_call_table(conn)

            def col(*names: str) -> str | None:
                for n in names:
                    c = colmap.get(n.upper())
                    if c:
                        return c
                return None

            c_date = col("ZDATE")
            c_addr = col("ZADDRESS")
            c_name = col("ZNAME")
            c_orig = col("ZORIGINATED")
            c_type = col("ZCALLTYPE")
            c_uid = col("ZUNIQUE_ID")

            if not c_date:
                raise SystemExit(f"Call table {table} missing ZDATE")

            where = f'"{c_date}" > ?'
            sql = f'SELECT * FROM "{table}" WHERE {where} ORDER BY "{c_date}" ASC'
            rows = conn.execute(sql, (last_zdate,)).fetchall()
        finally:
            conn.close()

        max_zdate = last_zdate
        enq = 0

        for r in rows:
            zdate = float(r[c_date]) if c_date and r[c_date] is not None else None
            if zdate is None:
                continue
            if zdate > max_zdate:
                max_zdate = zdate

            originated = None
            if c_orig and r[c_orig] is not None:
                try:
                    originated = int(r[c_orig])
                except Exception:
                    originated = None
            if originated != 1:
                continue  # only outbound

            occurred_at = _decode_apple_date_to_iso_utc(zdate)
            if not occurred_at:
                continue

            addr = (
                str(r[c_addr]).strip() if c_addr and r[c_addr] is not None else ""
            )
            name = (
                str(r[c_name]).strip() if c_name and r[c_name] is not None else ""
            )

            hints = PersonHints()
            if addr:
                if "@" in addr:
                    hints.emails.append(addr)
                else:
                    hints.phones.append(addr)
                hints.handles.append(addr)

            channel = "phone"
            if c_type and r[c_type] is not None:
                try:
                    t = int(r[c_type])
                    if t in (2, 3, 9):
                        channel = "facetime"
                except Exception:
                    pass

            uid = ""
            if c_uid and r[c_uid] is not None:
                uid = str(r[c_uid]).strip()

            dedupe = f"phone:{uid or zdate}:{addr or name or 'unknown'}"
            ev = ContactEvent(
                source="phone",
                channel=channel,
                person_hints=hints,
                occurred_at=occurred_at,
                dedupe_id=dedupe,
                direction="outbound",
            )
            payload = ev.to_json_bytes().decode("utf-8")
            ins = outbox.enqueue(ob, payload, ev.dedupe_id)
            if ins is not None:
                enq += 1

        if max_zdate > last_zdate:
            save_json_state(cursor_path, {"last_call_zdate": max_zdate})

        return CollectResult(
            source="phone",
            enqueued=enq,
            cursor_before=last_zdate,
            cursor_after=max_zdate,
        )


def _decode_apple_date_to_iso_utc(val: object) -> str | None:
    if val is None:
        return None
    try:
        v = float(val)
    except (TypeError, ValueError):
        return None
    if v == 0:
        return None
    seconds = v / 1e9 if abs(v) > 1e11 else v
    dt = APPLE_EPOCH_UTC + timedelta(seconds=seconds)
    return dt.astimezone(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def _find_callhistory_db() -> Path | None:
    base = Path.home() / "Library" / "Application Support" / "CallHistoryDB"
    single = base / "CallHistory.storedata"
    if single.is_file():
        return single
    if not base.is_dir():
        return None
    for p in sorted(base.rglob("*")):
        if p.is_file():
            return p
    return None


def _pick_call_table(conn: sqlite3.Connection) -> tuple[str, dict[str, str]]:
    cur = conn.execute("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name")
    tables = [r[0] for r in cur.fetchall()]
    preferred = None
    for t in ("ZCALLRECORD", "ZCALL"):
        if t in tables:
            preferred = t
            break
    if preferred is None:
        raise SystemExit("No call table found (expected ZCALLRECORD/ZCALL).")
    cols = [r[1] for r in conn.execute(f"PRAGMA table_info({preferred})").fetchall()]
    colmap = {c.upper(): c for c in cols}
    return preferred, colmap

