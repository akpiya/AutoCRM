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
	AppName      string
	ChatDBPath   string
	OutboxDBPath string
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
		maxRow, err := MaxMessageRowid(c.ChatDBPath)
		if err != nil {
			return CollectResult{}, err
		}
		cv := float64(maxRow)
		if _, err := outbox.IngestBatch(c.App(), nil, &cv, c.OutboxDBPath); err != nil {
			return CollectResult{}, err
		}
		log.Printf("imessage bootstrap cursor=%d in %.2fs", maxRow, time.Since(t0).Seconds())
		return CollectResult{
			Source:       c.App(),
			Enqueued:     0,
			CursorAfter:  &cv,
		}, nil
	}

	lastRow := int(*cursorBefore)
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
			"imessage ingest: %.2fs total (fetch=%.2fs %d msgs, events=%.2fs %d rows, outbox_write=%.2fs)",
			time.Since(t0).Seconds(), fetchS, len(rows), eventsS, len(events), ingestS,
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
		"imessage ingest: %.2fs total (fetch=%.2fs %d msgs, events=%.2fs %d rows, outbox_write=%.2fs)",
		time.Since(t0).Seconds(), fetchS, len(rows), eventsS, len(events), ingestS,
	)
	return CollectResult{
		Source:       c.App(),
		Enqueued:     enqueued,
		CursorBefore: &before,
		CursorAfter:  &after,
	}, nil
}
