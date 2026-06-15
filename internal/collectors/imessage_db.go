package collectors

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

// MessageRow is one message from chat.db.
type MessageRow struct {
	RowID          int
	MDate          int64
	IsFromMe       bool
	SenderHandle   string
	IsGroup        bool
	MemberHandles  []string
}

// MaxMessageRowid returns MAX(ROWID) from message, or 0.
func MaxMessageRowid(chatDB string) (int, error) {
	db, err := openChatDB(chatDB)
	if err != nil {
		return 0, err
	}
	defer db.Close()
	var max sql.NullInt64
	err = db.QueryRow(`SELECT MAX(ROWID) FROM message`).Scan(&max)
	if err != nil || !max.Valid {
		return 0, err
	}
	return int(max.Int64), nil
}

func openChatDB(chatDB string) (*sql.DB, error) {
	uri := fmt.Sprintf("file:%s?mode=ro&immutable=1", chatDB)
	return sql.Open("sqlite3", uri)
}

func memberHandlesForChats(db *sql.DB, chatIDs map[int]struct{}) (map[int][]string, error) {
	if len(chatIDs) == 0 {
		return map[int][]string{}, nil
	}
	ids := make([]int, 0, len(chatIDs))
	for id := range chatIDs {
		ids = append(ids, id)
	}
	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	q := fmt.Sprintf(`
SELECT chj.chat_id, h.id
FROM chat_handle_join chj
JOIN handle h ON h.ROWID = chj.handle_id
WHERE chj.chat_id IN (%s)
ORDER BY chj.chat_id, h.id`, placeholders)
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	buckets := map[int][]string{}
	seen := map[int]map[string]struct{}{}
	for rows.Next() {
		var chatID int
		var raw string
		if err := rows.Scan(&chatID, &raw); err != nil {
			return nil, err
		}
		handle := strings.TrimSpace(raw)
		if handle == "" {
			continue
		}
		if _, ok := seen[chatID]; !ok {
			seen[chatID] = map[string]struct{}{}
		}
		if _, dup := seen[chatID][handle]; dup {
			continue
		}
		seen[chatID][handle] = struct{}{}
		buckets[chatID] = append(buckets[chatID], handle)
	}
	return buckets, rows.Err()
}

// FetchMessagesAfterRowid returns messages with ROWID > afterRowid.
func FetchMessagesAfterRowid(chatDB string, afterRowid int) ([]MessageRow, error) {
	db, err := openChatDB(chatDB)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`
SELECT
  m.ROWID AS rowid,
  m.date AS mdate,
  m.is_from_me AS is_from_me,
  h.id AS sender_handle,
  c.ROWID AS chat_id,
  CASE
    WHEN c.chat_identifier LIKE '%;+;%'
      OR IFNULL(c.guid, '') LIKE '%;+;%'
    THEN 1
    ELSE 0
  END AS is_group
FROM message m
INNER JOIN chat_message_join cmj ON cmj.message_id = m.ROWID
INNER JOIN chat c ON c.ROWID = cmj.chat_id
LEFT JOIN handle h ON m.handle_id = h.ROWID
WHERE m.ROWID > ?
ORDER BY m.ROWID ASC`, afterRowid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type rawRow struct {
		rowid, chatID, mdate, isGroup int
		isFromMe                      bool
		sender                        sql.NullString
	}
	var raws []rawRow
	groupChatIDs := map[int]struct{}{}
	for rows.Next() {
		var r rawRow
		var isFromMe int
		if err := rows.Scan(&r.rowid, &r.mdate, &isFromMe, &r.sender, &r.chatID, &r.isGroup); err != nil {
			return nil, err
		}
		r.isFromMe = isFromMe != 0
		if r.isGroup != 0 {
			groupChatIDs[r.chatID] = struct{}{}
		}
		raws = append(raws, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	memberCache, err := memberHandlesForChats(db, groupChatIDs)
	if err != nil {
		return nil, err
	}

	var result []MessageRow
	for _, r := range raws {
		isGroup := r.isGroup != 0
		var sender string
		if r.sender.Valid {
			sender = strings.TrimSpace(r.sender.String)
		}
		members := memberCache[r.chatID]
		if !isGroup {
			members = nil
		}
		var senderPtr string
		if sender != "" {
			senderPtr = sender
		}
		result = append(result, MessageRow{
			RowID:         r.rowid,
			MDate:         int64(r.mdate),
			IsFromMe:      r.isFromMe,
			SenderHandle:  senderPtr,
			IsGroup:       isGroup,
			MemberHandles: members,
		})
	}
	return result, nil
}
