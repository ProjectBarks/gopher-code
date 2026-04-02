package testharness

import (
	"sync"

	"github.com/projectbarks/gopher-code/pkg/query"
)

// EventLog captures query events for post-hoc assertion.
type EventLog struct {
	mu     sync.Mutex
	events []query.QueryEvent
}

// Events returns a copy of all captured events.
func (l *EventLog) Events() []query.QueryEvent {
	l.mu.Lock()
	defer l.mu.Unlock()
	result := make([]query.QueryEvent, len(l.events))
	copy(result, l.events)
	return result
}

// TextDeltas returns all text delta strings.
func (l *EventLog) TextDeltas() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	var deltas []string
	for _, e := range l.events {
		if e.Type == query.QEventTextDelta {
			deltas = append(deltas, e.Text)
		}
	}
	return deltas
}

// ToolResults returns all tool result events.
func (l *EventLog) ToolResults() []query.QueryEvent {
	l.mu.Lock()
	defer l.mu.Unlock()
	var results []query.QueryEvent
	for _, e := range l.events {
		if e.Type == query.QEventToolResult {
			results = append(results, e)
		}
	}
	return results
}

// UsageEvents returns all usage events.
func (l *EventLog) UsageEvents() []query.QueryEvent {
	l.mu.Lock()
	defer l.mu.Unlock()
	var results []query.QueryEvent
	for _, e := range l.events {
		if e.Type == query.QEventUsage {
			results = append(results, e)
		}
	}
	return results
}

// NewEventCollector creates an EventCallback and EventLog pair.
func NewEventCollector() (query.EventCallback, *EventLog) {
	log := &EventLog{}
	callback := func(event query.QueryEvent) {
		log.mu.Lock()
		log.events = append(log.events, event)
		log.mu.Unlock()
	}
	return callback, log
}
