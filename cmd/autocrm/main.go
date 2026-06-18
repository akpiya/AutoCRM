// AutoCRM entrypoint.
package main

import (
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/akpiya/autocrm/internal/collectors"
	"github.com/akpiya/autocrm/internal/notion"
)

func main() {
	log.SetFlags(log.LstdFlags)

	if len(os.Args) < 2 {
		printUsage(os.Stderr)
		os.Exit(2)
	}

	switch os.Args[1] {
	case "install":
		os.Exit(runInstall())
	case "uninstall":
		os.Exit(runUninstall())
	case "run":
		runPipeline()
	case "doctor":
		os.Exit(runDoctor())
	case "help", "-h", "--help":
		printUsage(os.Stdout)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
		printUsage(os.Stderr)
		os.Exit(2)
	}
}

func printUsage(out *os.File) {
	fmt.Fprint(out, `AutoCRM syncs Mac communication activity to a Notion people database.

Usage:
  autocrm <command>

Commands:
  install  Install AutoCRM as a background LaunchAgent
  uninstall Remove AutoCRM's LaunchAgent and installed binary
  run      Run collectors and sync pending activity to Notion
  doctor   Check install location, Full Disk Access, launchd, and Notion setup
  help     Show this help
`)
}

func runPipeline() {
	var failures []string

	imessage, err := collectors.NewIMessageCollector()
	if err != nil {
		log.Fatalf("imessage collector: %v", err)
	}
	phoneCalls, err := collectors.NewPhoneCallsCollector()
	if err != nil {
		log.Fatalf("phone_calls collector: %v", err)
	}
	collectors := []collectors.Collector{
		imessage,
		phoneCalls,
		collectors.BeeperCollector{},
	}

	for _, c := range collectors {
		log.Printf("Collector start: %s", c.App())
		t0 := time.Now()
		result, err := c.Collect()
		if err != nil {
			failures = append(failures, c.App())
			log.Printf("Collector failed: %s: %v", c.App(), err)
			continue
		}
		var cursorBefore any
		if result.CursorBefore != nil {
			cursorBefore = *result.CursorBefore
		}
		var cursorAfter any
		if result.CursorAfter != nil {
			cursorAfter = *result.CursorAfter
		}
		log.Printf(
			"Collector done: %s in %.2fs (enqueued=%d cursor_before=%v cursor_after=%v)",
			c.App(), time.Since(t0).Seconds(), result.Enqueued,
			cursorBefore, cursorAfter,
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
