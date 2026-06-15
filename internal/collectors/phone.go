package collectors

// PhoneCallsCollector reads call history into the outbox (stub).
type PhoneCallsCollector struct{}

func (PhoneCallsCollector) App() string { return "phone_calls" }

func (PhoneCallsCollector) Collect() (CollectResult, error) {
	return CollectResult{Source: "phone_calls", Enqueued: 0}, nil
}
