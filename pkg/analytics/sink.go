package analytics

import (
	"math/rand/v2"
)

// DatadogGateName is the GrowthBook feature gate controlling Datadog event dispatch.
const DatadogGateName = "tengu_log_datadog_events"

// EventSamplingConfigName is the GrowthBook dynamic config key for per-event sample rates.
const EventSamplingConfigName = "tengu_event_sampling_config"

// EventSamplingEntry holds the sample rate for a single event type.
type EventSamplingEntry struct {
	SampleRate float64 `json:"sample_rate"`
}

// GateChecker returns whether a named feature gate is enabled.
// Injected at startup; defaults to false if unset.
type GateChecker func(gateName string) bool

// SamplingConfigProvider returns the per-event sampling config.
type SamplingConfigProvider func() map[string]EventSamplingEntry

// CompositeSinkConfig configures the composite analytics sink.
type CompositeSinkConfig struct {
	Datadog        *DatadogSink
	GateChecker    GateChecker
	SamplingConfig SamplingConfigProvider
}

// CompositeSink routes events to Datadog and first-party backends with
// sampling, killswitch, and gate checks.
type CompositeSink struct {
	dd             *DatadogSink
	gateChecker    GateChecker
	samplingConfig SamplingConfigProvider
}

// NewCompositeSink creates the main analytics sink.
func NewCompositeSink(cfg CompositeSinkConfig) *CompositeSink {
	return &CompositeSink{
		dd:             cfg.Datadog,
		gateChecker:    cfg.GateChecker,
		samplingConfig: cfg.SamplingConfig,
	}
}

// LogEvent implements Sink.
func (c *CompositeSink) LogEvent(eventName string, metadata EventMetadata) {
	c.dispatch(eventName, metadata)
}

// LogEventAsync implements Sink. Currently identical to LogEvent since
// both remaining backends (Datadog, 1P) are fire-and-forget.
func (c *CompositeSink) LogEventAsync(eventName string, metadata EventMetadata) {
	c.dispatch(eventName, metadata)
}

// Shutdown flushes all backends.
func (c *CompositeSink) Shutdown() {
	if c.dd != nil {
		c.dd.Close()
	}
}

func (c *CompositeSink) dispatch(eventName string, metadata EventMetadata) {
	// Check sampling.
	sampleResult := c.shouldSampleEvent(eventName)
	if sampleResult == 0 {
		return // dropped by sampling
	}

	// Annotate with sample_rate if sampled.
	md := metadata
	if sampleResult > 0 {
		md = cloneMetadata(metadata)
		md["sample_rate"] = sampleResult
	}

	// Datadog: general-access — strip _PROTO_*, check gate + allowlist.
	if c.shouldTrackDatadog() && c.dd != nil {
		stripped := StripProtoFields(md)
		if DatadogAllowedEvents[eventName] {
			enriched := GetEventMetadata()
			for k, v := range stripped {
				enriched[k] = v
			}
			c.dd.Enqueue(DatadogLog{
				DDSource: "go",
				DDTags:   "event:" + eventName,
				Message:  eventName,
				Service:  "claude-code",
				Hostname: "claude-code",
				Extra:    enriched,
			})
		}
	}

	// 1P logging: receives full payload including _PROTO_*.
	// The OTel exporter handles PII routing internally.
	// (First-party event logging will be wired when the OTel exporter is implemented.)
}

// shouldSampleEvent checks the dynamic sampling config.
// Returns: nil-like 0 = drop; negative = log without annotation; positive = sample rate to annotate.
// We encode: -1 means "log without annotation", 0 means "drop", >0 means "annotate with rate".
func (c *CompositeSink) shouldSampleEvent(eventName string) float64 {
	if c.samplingConfig == nil {
		return -1 // no config → log everything
	}
	config := c.samplingConfig()
	entry, ok := config[eventName]
	if !ok {
		return -1 // no config for this event → log at 100%
	}
	rate := entry.SampleRate
	if rate < 0 || rate > 1 {
		return -1 // invalid → log at 100%
	}
	if rate >= 1 {
		return -1 // 100% → log without annotation
	}
	if rate <= 0 {
		return 0 // 0% → drop
	}
	if rand.Float64() < rate {
		return rate // sampled in
	}
	return 0 // sampled out
}

func (c *CompositeSink) shouldTrackDatadog() bool {
	if IsSinkKilled(SinkDatadog) {
		return false
	}
	if c.gateChecker != nil {
		return c.gateChecker(DatadogGateName)
	}
	return false
}

func cloneMetadata(m EventMetadata) EventMetadata {
	out := make(EventMetadata, len(m)+1)
	for k, v := range m {
		out[k] = v
	}
	return out
}
