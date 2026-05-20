#!/usr/bin/env bash
# View, clear, or inspect ~/.autocrm/outbox.db (cursor or outbox table).

set -euo pipefail

OUTBOX_DB="${HOME}/.autocrm/outbox.db"
ACTION="${1:-}"
TABLE="${2:-}"

usage() {
  echo "Usage: $(basename "$0") <view|clear|size> <cursor|outbox>" >&2
  exit 1
}

[[ "$ACTION" == "view" || "$ACTION" == "clear" || "$ACTION" == "size" ]] || usage
[[ "$TABLE" == "cursor" || "$TABLE" == "outbox" ]] || usage

if [[ ! -f "$OUTBOX_DB" ]]; then
  echo "Database not found: $OUTBOX_DB" >&2
  exit 1
fi

case "$ACTION" in
  view)
    case "$TABLE" in
      outbox)
        sqlite3 -header -column "$OUTBOX_DB" \
          "SELECT id, platform, party_id, direction,
                  datetime(created_at, 'unixepoch') AS created_at
           FROM outbox
           ORDER BY id DESC
           LIMIT 20;"
        ;;
      cursor)
        sqlite3 -header -column "$OUTBOX_DB" \
          "SELECT source, cursor_value,
                  datetime(updated_at, 'unixepoch') AS updated_at
           FROM cursor
           ORDER BY source;"
        ;;
    esac
    ;;
  clear)
    sqlite3 "$OUTBOX_DB" "DELETE FROM ${TABLE};"
    echo "Cleared table: ${TABLE}"
    ;;
  size)
    rows=$(sqlite3 "$OUTBOX_DB" "SELECT COUNT(*) FROM ${TABLE};")
    cols=$(sqlite3 "$OUTBOX_DB" "SELECT COUNT(*) FROM pragma_table_info('${TABLE}');")
    echo "${TABLE}: ${rows} rows, ${cols} columns"
    ;;
esac
