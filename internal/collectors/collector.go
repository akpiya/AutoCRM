// Package collectors ingest Mac communication sources into the outbox.
package collectors

// CollectResult summarizes one collector run.
type CollectResult struct {
	Source       string
	Enqueued     int
	CursorBefore *float64
	CursorAfter  *float64
}

// Collector ingests a single source into the outbox.
type Collector interface {
	App() string
	Collect() (CollectResult, error)
}
