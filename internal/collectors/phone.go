package collectors

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/akpiya/autocrm/internal/common"
	"github.com/akpiya/autocrm/internal/outbox"
)

// PhoneCallsCollector reads macOS call history into the outbox.
type PhoneCallsCollector struct {
	AppName      string
	CallDBPath   string
	OutboxDBPath string
}

// NewPhoneCallsCollector returns a collector with default paths.
func NewPhoneCallsCollector() (*PhoneCallsCollector, error) {
	outboxPath, err := common.OutboxDBPath()
	if err != nil {
		return nil, err
	}
	return &PhoneCallsCollector{
		AppName:      "phone_calls",
		CallDBPath:   common.DefaultCallHistoryDBPath(),
		OutboxDBPath: outboxPath,
	}, nil
}

func (c *PhoneCallsCollector) App() string {
	if c.AppName != "" {
		return c.AppName
	}
	return "phone_calls"
}

func eventsForCall(row PhoneCallRow) []outbox.Event {
	partyID := strings.TrimSpace(row.Address)
	if partyID == "" {
		return nil
	}
	createdAt, ok := common.AppleSecToUnix(row.DateSeconds)
	if !ok {
		return nil
	}
	direction := common.DirectionInbound
	if row.IsOriginated {
		direction = common.DirectionOutbound
	}
	platform := common.PlatformPhoneCall
	switch row.CallType {
	case 8:
		platform = common.PlatformFaceTimeVideo
	case 16:
		platform = common.PlatformFaceTimeAudio
	}
	return []outbox.Event{{
		Platform:  platform,
		PartyID:   partyID,
		Direction: direction,
		CreatedAt: createdAt,
	}}
}

// Collect ingests call records after the stored cursor.
func (c *PhoneCallsCollector) Collect() (CollectResult, error) {
	if _, err := os.Stat(c.CallDBPath); err != nil {
		if os.IsNotExist(err) {
			return CollectResult{}, fmt.Errorf("call history db not found: %s", c.CallDBPath)
		}
		return CollectResult{}, err
	}

	t0 := time.Now()
	cursorBefore, err := outbox.GetCursor(c.App(), c.OutboxDBPath)
	if err != nil {
		return CollectResult{}, err
	}

	if cursorBefore == nil {
		maxRow, err := MaxCallRowid(c.CallDBPath)
		if err != nil {
			return CollectResult{}, err
		}
		cv := float64(maxRow)
		if _, err := outbox.IngestBatch(c.App(), nil, &cv, c.OutboxDBPath); err != nil {
			return CollectResult{}, err
		}
		log.Printf("%s bootstrap cursor=%d in %.2fs", c.App(), maxRow, time.Since(t0).Seconds())
		return CollectResult{
			Source:      c.App(),
			Enqueued:    0,
			CursorAfter: &cv,
		}, nil
	}

	lastRow := int(*cursorBefore)
	tFetch := time.Now()
	rows, err := FetchCallsAfterRowid(c.CallDBPath, lastRow)
	if err != nil {
		return CollectResult{}, err
	}
	fetchS := time.Since(tFetch).Seconds()

	maxRow := lastRow
	var events []outbox.Event
	tEvents := time.Now()
	for _, row := range rows {
		if row.RowID > maxRow {
			maxRow = row.RowID
		}
		events = append(events, eventsForCall(row)...)
	}
	eventsS := time.Since(tEvents).Seconds()

	enqueued := 0
	ingestS := 0.0
	before := float64(lastRow)
	if maxRow > lastRow || len(events) > 0 {
		cv := float64(maxRow)
		tIngest := time.Now()
		enqueued, err = outbox.IngestBatch(c.App(), events, &cv, c.OutboxDBPath)
		if err != nil {
			return CollectResult{}, err
		}
		ingestS = time.Since(tIngest).Seconds()
		after := cv
		log.Printf(
			"%s ingest: %.2fs total (fetch=%.2fs %d msgs, events=%.2fs %d rows, outbox_write=%.2fs)",
			c.App(), time.Since(t0).Seconds(), fetchS, len(rows), eventsS, len(events), ingestS,
		)
		return CollectResult{
			Source:       c.App(),
			Enqueued:     enqueued,
			CursorBefore: &before,
			CursorAfter:  &after,
		}, nil
	}

	after := float64(maxRow)
	log.Printf(
		"%s ingest: %.2fs total (fetch=%.2fs %d msgs, events=%.2fs %d rows, outbox_write=%.2fs)",
		c.App(), time.Since(t0).Seconds(), fetchS, len(rows), eventsS, len(events), ingestS,
	)
	return CollectResult{
		Source:       c.App(),
		Enqueued:     enqueued,
		CursorBefore: &before,
		CursorAfter:  &after,
	}, nil
}
