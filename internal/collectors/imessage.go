package collectors

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/akpiya/autocrm/internal/common"
	"github.com/akpiya/autocrm/internal/outbox"
)

// IMessageCollector reads Messages chat.db into the outbox.
type IMessageCollector struct {
	AppName           string
	ChatDBPath        string
	OutboxDBPath      string
	BootstrapLookback time.Duration
	Now               func() time.Time
}

// NewIMessageCollector returns a collector with default paths.
func NewIMessageCollector() (*IMessageCollector, error) {
	outboxPath, err := common.OutboxDBPath()
	if err != nil {
		return nil, err
	}
	return &IMessageCollector{
		AppName:      "imessage",
		ChatDBPath:   common.DefaultChatDBPath(),
		OutboxDBPath: outboxPath,
	}, nil
}

func (c *IMessageCollector) App() string {
	if c.AppName != "" {
		return c.AppName
	}
	return "imessage"
}

func eventsForMessage(row MessageRow) []outbox.Event {
	createdAt, ok := common.AppleNsToUnix(row.MDate)
	if !ok {
		return nil
	}
	if row.IsGroup {
		if row.IsFromMe {
			var events []outbox.Event
			for _, handle := range row.MemberHandles {
				events = append(events, outbox.Event{
					Platform:  common.PlatformText,
					PartyID:   handle,
					Direction: common.DirectionOutbound,
					CreatedAt: createdAt,
				})
			}
			return events
		}
		if row.SenderHandle != "" {
			return []outbox.Event{{
				Platform:  common.PlatformText,
				PartyID:   row.SenderHandle,
				Direction: common.DirectionInbound,
				CreatedAt: createdAt,
			}}
		}
		return nil
	}
	if row.SenderHandle == "" {
		return nil
	}
	dir := common.DirectionInbound
	if row.IsFromMe {
		dir = common.DirectionOutbound
	}
	return []outbox.Event{{
		Platform:  common.PlatformText,
		PartyID:   row.SenderHandle,
		Direction: dir,
		CreatedAt: createdAt,
	}}
}

func (c *IMessageCollector) bootstrapLookback() time.Duration {
	if c.BootstrapLookback > 0 {
		return c.BootstrapLookback
	}
	return 10 * time.Minute
}

func (c *IMessageCollector) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

func appleNSTime(t time.Time) int64 {
	d := t.UTC().Sub(common.AppleEpochUTC)
	if d <= 0 {
		return 0
	}
	return d.Nanoseconds()
}

// Collect ingests messages after the stored cursor.
func (c *IMessageCollector) Collect() (CollectResult, error) {
	if _, err := os.Stat(c.ChatDBPath); err != nil {
		if os.IsNotExist(err) {
			return CollectResult{}, fmt.Errorf("chat.db not found: %s", c.ChatDBPath)
		}
		return CollectResult{}, err
	}

	t0 := time.Now()
	cursorBefore, err := outbox.GetCursor(c.App(), c.OutboxDBPath)
	if err != nil {
		return CollectResult{}, err
	}

	if cursorBefore == nil {
		cutoff := c.now().Add(-c.bootstrapLookback())
		effectiveCursor, err := MaxMessageRowidBeforeAppleNS(c.ChatDBPath, appleNSTime(cutoff))
		if err != nil {
			return CollectResult{}, err
		}
		log.Printf("imessage bootstrap lookback=%.0fm effective_cursor=%d cutoff=%s", c.bootstrapLookback().Minutes(), effectiveCursor, cutoff.Format(time.RFC3339))
		return c.collectAfterCursor(effectiveCursor, nil, t0)
	}

	return c.collectAfterCursor(int(*cursorBefore), cursorBefore, t0)
}

func (c *IMessageCollector) collectAfterCursor(lastRow int, cursorBefore *float64, t0 time.Time) (CollectResult, error) {
	tFetch := time.Now()
	rows, err := FetchMessagesAfterRowid(c.ChatDBPath, lastRow)
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
		events = append(events, eventsForMessage(row)...)
	}
	eventsS := time.Since(tEvents).Seconds()

	enqueued := 0
	ingestS := 0.0
	if cursorBefore == nil || maxRow > lastRow || len(events) > 0 {
		cv := float64(maxRow)
		tIngest := time.Now()
		enqueued, err = outbox.IngestBatch(c.App(), events, &cv, c.OutboxDBPath)
		if err != nil {
			return CollectResult{}, err
		}
		ingestS = time.Since(tIngest).Seconds()
		after := cv
		log.Printf(
			"imessage ingest: %.2fs total (fetch=%.2fs %d msgs, events=%.2fs %d rows, outbox_write=%.2fs)",
			time.Since(t0).Seconds(), fetchS, len(rows), eventsS, len(events), ingestS,
		)
		return CollectResult{
			Source:       c.App(),
			Enqueued:     enqueued,
			CursorBefore: cursorBefore,
			CursorAfter:  &after,
		}, nil
	}

	after := float64(maxRow)
	log.Printf(
		"imessage ingest: %.2fs total (fetch=%.2fs %d msgs, events=%.2fs %d rows, outbox_write=%.2fs)",
		time.Since(t0).Seconds(), fetchS, len(rows), eventsS, len(events), ingestS,
	)
	return CollectResult{
		Source:       c.App(),
		Enqueued:     enqueued,
		CursorBefore: cursorBefore,
		CursorAfter:  &after,
	}, nil
}
