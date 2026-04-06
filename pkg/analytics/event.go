package analytics

import (
	"sync"
)

// EventMetadata holds key-value pairs attached to every analytics event.
// Values are intentionally restricted to bool/number to prevent accidental
// logging of code or file paths. The TS source enforces this via branded
// types; Go relies on the restricted value type.
type EventMetadata map[string]any

// Sink is the analytics backend interface. The default implementation
// routes events to Datadog + first-party logging.
type Sink interface {
	// LogEvent sends an event synchronously (fire-and-forget).
	LogEvent(eventName string, metadata EventMetadata)
	// LogEventAsync sends an event that may require async enrichment.
	LogEventAsync(eventName string, metadata EventMetadata)
	// Shutdown flushes pending events and releases resources.
	Shutdown()
}

// queuedEvent is an event logged before the sink was attached.
type queuedEvent struct {
	eventName string
	metadata  EventMetadata
	async     bool
}

// sinkState is the global analytics state. Access is serialized via mu.
var (
	mu         sync.Mutex
	sink       Sink
	eventQueue []queuedEvent
)

// AttachSink registers the analytics backend. Queued events are drained
// immediately. Idempotent: subsequent calls are no-ops.
func AttachSink(s Sink) {
	mu.Lock()
	defer mu.Unlock()

	if sink != nil {
		return
	}
	sink = s

	// Drain queued events.
	queued := eventQueue
	eventQueue = nil

	for _, e := range queued {
		if e.async {
			sink.LogEventAsync(e.eventName, e.metadata)
		} else {
			sink.LogEvent(e.eventName, e.metadata)
		}
	}
}

// LogEvent logs an analytics event. If no sink is attached yet, the event
// is queued and drained when AttachSink is called.
func LogEvent(eventName string, metadata EventMetadata) {
	mu.Lock()
	defer mu.Unlock()

	if sink == nil {
		eventQueue = append(eventQueue, queuedEvent{eventName, metadata, false})
		return
	}
	sink.LogEvent(eventName, metadata)
}

// LogEventAsync logs an analytics event that may require async enrichment.
func LogEventAsync(eventName string, metadata EventMetadata) {
	mu.Lock()
	defer mu.Unlock()

	if sink == nil {
		eventQueue = append(eventQueue, queuedEvent{eventName, metadata, true})
		return
	}
	sink.LogEventAsync(eventName, metadata)
}

// Shutdown flushes and tears down the attached sink.
func Shutdown() {
	mu.Lock()
	s := sink
	mu.Unlock()

	if s != nil {
		s.Shutdown()
	}
}

// ResetForTesting clears all analytics state. Test-only.
func ResetForTesting() {
	mu.Lock()
	defer mu.Unlock()
	sink = nil
	eventQueue = nil
}

// StripProtoFields removes all _PROTO_* keys from metadata.
// These keys carry PII-tagged values destined only for the privileged
// first-party proto columns. General-access backends (Datadog) must never
// see them. Returns the input unchanged when no _PROTO_ keys are present.
func StripProtoFields(m EventMetadata) EventMetadata {
	hasProto := false
	for k := range m {
		if len(k) > 7 && k[:7] == "_PROTO_" {
			hasProto = true
			break
		}
	}
	if !hasProto {
		return m
	}
	out := make(EventMetadata, len(m))
	for k, v := range m {
		if len(k) > 7 && k[:7] == "_PROTO_" {
			continue
		}
		out[k] = v
	}
	return out
}
