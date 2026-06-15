// AutoCRM entrypoint — run collectors, then Notion sync when configured.
package main

import (
	"log"
	"sort"
	"strings"
	"time"

	"github.com/akpiya/autocrm/internal/collectors"
	"github.com/akpiya/autocrm/internal/notion"
)

func main() {
	log.SetFlags(log.LstdFlags)
	var failures []string

	imessage, err := collectors.NewIMessageCollector()
	if err != nil {
		log.Fatalf("imessage collector: %v", err)
	}
	phoneCalls, err := collectors.NewPhoneCallsCollector()
	if err != nil {
		log.Fatalf("phone_calls collector: %v", err)
	}
	all := []collectors.Collector{
		imessage,
		phoneCalls,
		collectors.BeeperCollector{},
	}

	for _, c := range all {
		log.Printf("Collector start: %s", c.App())
		t0 := time.Now()
		result, err := c.Collect()
		if err != nil {
			failures = append(failures, c.App())
			log.Printf("Collector failed: %s: %v", c.App(), err)
			continue
		}
		log.Printf(
			"Collector done: %s in %.2fs (enqueued=%d cursor_before=%v cursor_after=%v)",
			c.App(), time.Since(t0).Seconds(), result.Enqueued,
			ptrVal(result.CursorBefore), ptrVal(result.CursorAfter),
		)
	}

	if notion.Configured() {
		log.Print("Notion sync start")
		t0 := time.Now()
		stats, err := notion.SyncOutbox("", nil, 0)
		if err != nil {
			failures = append(failures, "notion")
			log.Printf("Notion sync failed: %v", err)
		} else {
			log.Printf(
				"Notion sync done in %.2fs (pending=%d applied=%d errors=%d)",
				time.Since(t0).Seconds(), stats.Pending, stats.Applied, stats.Errors,
			)
			if stats.Errors > 0 {
				failures = append(failures, "notion")
			}
		}
	}

	if len(failures) > 0 {
		sort.Strings(failures)
		uniq := failures[:0]
		for i, f := range failures {
			if i == 0 || f != failures[i-1] {
				uniq = append(uniq, f)
			}
		}
		log.Fatalf("pipeline completed with failures: %s", strings.Join(uniq, ", "))
	}
}

func ptrVal(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}
