package collectors

// BeeperCollector reads Beeper Desktop into the outbox (stub).
type BeeperCollector struct{}

func (BeeperCollector) App() string { return "beeper" }

func (BeeperCollector) Collect() (CollectResult, error) {
	return CollectResult{Source: "beeper", Enqueued: 0}, nil
}
