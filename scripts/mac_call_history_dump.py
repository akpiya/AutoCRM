#!/usr/bin/env python3
"""
Read macOS Call History (FaceTime / Phone / Continuity) from the local store.

Path: ~/Library/Application Support/CallHistoryDB/

Requires Full Disk Access for the terminal / Python you run this with:
  System Settings → Privacy & Security → Full Disk Access

The schema is undocumented and can change between macOS versions; this script
introspects tables and prefers ZCALLRECORD when present.

Compare output to iPhone Recents manually — rows that never hit the Mac may be missing.
"""

from __future__ import annotations

import argparse
import sqlite3
import sys
from datetime import datetime, timedelta, timezone
from pathlib import Path

APPLE_EPOCH_UTC = datetime(2001, 1, 1, 0, 0, 0, tzinfo=timezone.utc)
SQLITE_MAGIC = b"SQLite format 3\x00"


def _looks_sqlite_file(path: Path) -> bool:
    try:
        with path.open("rb") as f:
            return f.read(16) == SQLITE_MAGIC
    except OSError:
        return False


def _find_callhistory_dbs() -> list[Path]:
    base = Path.home() / "Library" / "Application Support" / "CallHistoryDB"
    if not base.is_dir():
        return []
    found: list[Path] = []
    for path in sorted(base.rglob("*")):
        if path.is_file() and _looks_sqlite_file(path):
            found.append(path)
    # Main bundle is often a single file named CallHistory.storedata (still SQLite).
    single = base / "CallHistory.storedata"
    if single.is_file() and _looks_sqlite_file(single) and single not in found:
        found.insert(0, single)
    return found


def _decode_zdate(val: object) -> str:
    """Core Data often stores NSDate as seconds since 2001; some builds use nanoseconds."""
    if val is None:
        return "—"
    try:
        v = float(val)
    except (TypeError, ValueError):
        return str(val)
    if v == 0:
        return "—"
    seconds = v / 1e9 if abs(v) > 1e11 else v
    dt = APPLE_EPOCH_UTC + timedelta(seconds=seconds)
    return dt.astimezone(timezone.utc).strftime("%Y-%m-%d %H:%M:%S UTC")


def _pick_call_table(conn: sqlite3.Connection) -> tuple[str, list[str]] | None:
    cur = conn.execute(
        "SELECT name FROM sqlite_master WHERE type='table' ORDER BY name"
    )
    tables = [r[0] for r in cur.fetchall()]
    for preferred in ("ZCALLRECORD", "ZCALL"):
        if preferred in tables:
            cols = [
                r[1]
                for r in conn.execute(f"PRAGMA table_info({preferred})").fetchall()
            ]
            return preferred, cols
    for t in tables:
        if not t.startswith("Z") or "CALL" not in t.upper():
            continue
        cols = [r[1] for r in conn.execute(f"PRAGMA table_info({t})").fetchall()]
        upper = {c.upper() for c in cols}
        if "ZDATE" in upper and (
            "ZADDRESS" in upper or "ZNAME" in upper or "ZUNIQUE_ID" in upper
        ):
            return t, cols
    return None


def _originated_label(v: object) -> str:
    if v is None:
        return "?"
    try:
        i = int(v)
    except (TypeError, ValueError):
        return str(v)
    if i == 1:
        return "out"
    if i == 0:
        return "in"
    return str(i)


def _calltype_label(v: object) -> str:
    if v is None:
        return "?"
    try:
        i = int(v)
    except (TypeError, ValueError):
        return str(v)
    # Heuristic; Apple does not publish these. Adjust if your rows disagree with UI.
    names = {
        1: "phone?",
        2: "ft_audio?",
        3: "ft_video?",
        4: "other?",
        8: "phone?",
        9: "ft?",
    }
    return names.get(i, str(i))


def dump_db(path: Path, limit: int) -> None:
    uri = f"file:{path}?mode=ro"
    try:
        conn = sqlite3.connect(uri, uri=True)
    except sqlite3.Error as e:
        print(f"Cannot open {path}: {e}", file=sys.stderr)
        return
    try:
        picked = _pick_call_table(conn)
        if not picked:
            print(f"{path}: no call-like Z* table found.", file=sys.stderr)
            cur = conn.execute(
                "SELECT name FROM sqlite_master WHERE type='table' ORDER BY 1"
            )
            print("Tables:", ", ".join(r[0] for r in cur.fetchall()))
            return
        table, cols = picked
        colset = {c.upper(): c for c in cols}

        def col(*names: str) -> str | None:
            for n in names:
                if n.upper() in colset:
                    return colset[n.upper()]
            return None

        c_date = col("ZDATE")
        c_addr = col("ZADDRESS")
        c_name = col("ZNAME")
        c_dur = col("ZDURATION")
        c_orig = col("ZORIGINATED")
        c_type = col("ZCALLTYPE")
        c_ans = col("ZANSWERED")
        c_uid = col("ZUNIQUE_ID")
        c_dev = col("ZDEVICE_ID")
        c_cc = col("ZISO_COUNTRY_CODE")

        if not c_date:
            print(f"{path}: table {table} has no ZDATE.", file=sys.stderr)
            return

        select_parts = [f'"{c_date}" AS zdate']
        aliases = [
            (c_addr, "zaddress"),
            (c_name, "zname"),
            (c_dur, "zduration"),
            (c_orig, "zoriginated"),
            (c_type, "zcalltype"),
            (c_ans, "zanswered"),
            (c_uid, "zunique_id"),
            (c_dev, "zdevice_id"),
            (c_cc, "zcountry"),
        ]
        for c, alias in aliases:
            if c:
                select_parts.append(f'"{c}" AS {alias}')
        sql = f'SELECT {", ".join(select_parts)} FROM "{table}" ORDER BY zdate DESC LIMIT ?'

        cur = conn.execute(sql, (limit,))
        names = [x[0] for x in cur.description]
        rows = cur.fetchall()

        print(f"\n=== {path} ===")
        print(f"Table: {table}  (showing up to {limit} rows, newest first)\n")
        if not rows:
            print("(no rows)")
            return

        for r in rows:
            d = dict(zip(names, r))
            when = _decode_zdate(d.get("zdate"))
            addr = d.get("zaddress") or "—"
            name = d.get("zname") or "—"
            dur = d.get("zduration")
            dur_s = f"{float(dur):.1f}s" if dur is not None else "—"
            orig = _originated_label(d.get("zoriginated"))
            ctype = _calltype_label(d.get("zcalltype"))
            ans = d.get("zanswered")
            ans_s = "yes" if ans == 1 else ("no" if ans == 0 else str(ans))
            uid = (d.get("zunique_id") or "")[:13]
            dev = d.get("zdevice_id") or "—"
            cc = d.get("zcountry") or "—"
            print(
                f"{when}  {orig:>3}  type={ctype:8}  dur={dur_s:>8}  ans={ans_s:3}  "
                f"cc={cc}  addr={addr}  name={name}"
            )
            if uid:
                print(f"         id…={uid}…  device={dev}")
    finally:
        conn.close()


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--limit",
        type=int,
        default=40,
        help="Max rows per database (default: 40)",
    )
    parser.add_argument(
        "--db",
        type=Path,
        default=None,
        help="Explicit SQLite path (skip discovery)",
    )
    args = parser.parse_args()

    if args.db is not None:
        paths = [args.db.expanduser()]
        for p in paths:
            if not p.is_file():
                print(f"Not a file: {p}", file=sys.stderr)
                sys.exit(1)
            if not _looks_sqlite_file(p):
                print(f"Not a SQLite file: {p}", file=sys.stderr)
                sys.exit(1)
    else:
        paths = _find_callhistory_dbs()

    if not paths:
        base = Path.home() / "Library/Application Support/CallHistoryDB"
        print(
            f"No SQLite call history DB found under:\n  {base}\n\n"
            "Enable Full Disk Access for this app, or open FaceTime/Phone once so "
            "the store is created. Use --db PATH if you know the file location.",
            file=sys.stderr,
        )
        sys.exit(1)

    print(
        "macOS Call History dump — compare to iPhone Recents.\n"
        "If the Mac never participated in a call, it may be missing here.\n"
    )
    for p in paths:
        dump_db(p, max(1, args.limit))


if __name__ == "__main__":
    main()
