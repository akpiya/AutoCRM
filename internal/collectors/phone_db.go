package collectors

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

// PhoneCallRow is one call record from CallHistory.storedata.
type PhoneCallRow struct {
	RowID        int
	DateSeconds  float64
	IsOriginated bool
	CallType     int
	Address      string
}

// MaxCallRowid returns MAX(Z_PK) from call history, or 0.
func MaxCallRowid(callDB string) (int, error) {
	db, err := openCallHistoryDB(callDB)
	if err != nil {
		return 0, err
	}
	defer db.Close()

	table, err := callRecordTable(db)
	if err != nil {
		return 0, err
	}

	var max sql.NullInt64
	err = db.QueryRow(fmt.Sprintf(`SELECT MAX(Z_PK) FROM %s`, table)).Scan(&max)
	if err != nil || !max.Valid {
		return 0, err
	}
	return int(max.Int64), nil
}

func openCallHistoryDB(callDB string) (*sql.DB, error) {
	// CallHistory.storedata is WAL-backed; avoid immutable mode so recent rows
	// in the WAL are visible during read-only ingestion.
	uri := fmt.Sprintf("file:%s?mode=ro", callDB)
	return sql.Open("sqlite3", uri)
}

func callRecordTable(db *sql.DB) (string, error) {
	candidates := []string{"ZCALLRECORD", "ZCALLHISTORY"}
	for _, table := range candidates {
		var count int
		if err := db.QueryRow(
			`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name = ?`,
			table,
		).Scan(&count); err != nil {
			return "", err
		}
		if count > 0 {
			return table, nil
		}
	}
	return "", fmt.Errorf("call history table not found (expected ZCALLRECORD or ZCALLHISTORY)")
}

// FetchCallsAfterRowid returns connected call rows with Z_PK > afterRowid.
func FetchCallsAfterRowid(callDB string, afterRowid int) ([]PhoneCallRow, error) {
	db, err := openCallHistoryDB(callDB)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	table, err := callRecordTable(db)
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(fmt.Sprintf(`
SELECT
  Z_PK AS rowid,
  ZDATE AS zdate,
  ZORIGINATED AS originated,
  IFNULL(ZDURATION, 0) AS duration,
  IFNULL(ZANSWERED, 0) AS answered,
  IFNULL(ZCALLTYPE, 0) AS call_type,
  ZADDRESS AS address
FROM %s
WHERE Z_PK > ?
  AND IFNULL(ZORIGINATED, -1) IN (0, 1)
  AND (IFNULL(ZANSWERED, 0) = 1 OR IFNULL(ZDURATION, 0) > 0)
  AND IFNULL(ZCALLTYPE, 0) IN (1, 8, 16)
ORDER BY Z_PK ASC`, table), afterRowid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []PhoneCallRow
	for rows.Next() {
		var rowID int
		var dateSeconds float64
		var originated int
		var duration float64
		var answered int
		var callType int
		var address sql.NullString
		if err := rows.Scan(
			&rowID, &dateSeconds, &originated, &duration, &answered, &callType, &address,
		); err != nil {
			return nil, err
		}
		call := PhoneCallRow{
			RowID:        rowID,
			DateSeconds:  dateSeconds,
			IsOriginated: originated != 0,
			CallType:     callType,
		}
		if address.Valid {
			call.Address = strings.TrimSpace(address.String)
		}
		result = append(result, call)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
