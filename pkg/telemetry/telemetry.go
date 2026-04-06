// Package telemetry provides OpenTelemetry-based metrics, tracing, and logging
// for gopher-code sessions.
//
// T126: OTel counters — named counters for session events.
// T127: statsStore.observe — lightweight metric observation.
// T128: OTel providers — logger, event logger, meter, and tracer providers.
//
// Source: bootstrap/state.ts — meter, sessionCounter, locCounter, prCounter,
// commitCounter, costCounter, tokenCounter, codeEditToolDecisionCounter,
// activeTimeCounter, statsStore, loggerProvider, eventLogger, meterProvider,
// tracerProvider.
package telemetry

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"
	nooptrace "go.opentelemetry.io/otel/trace/noop"
)

// ---------------------------------------------------------------------------
// T128: OTel providers
// ---------------------------------------------------------------------------

// Providers holds the OTel provider instances for a session.
// Source: bootstrap/state.ts — loggerProvider, eventLogger, meterProvider, tracerProvider
type Providers struct {
	MeterProvider  metric.MeterProvider
	TracerProvider trace.TracerProvider
}

// NewProviders creates Providers using the global OTel providers.
// If no real exporter is configured, OTel defaults to no-op automatically.
func NewProviders() *Providers {
	mp := otel.GetMeterProvider()
	tp := otel.GetTracerProvider()
	return &Providers{
		MeterProvider:  mp,
		TracerProvider: tp,
	}
}

// NewNoopProviders creates Providers backed by no-op implementations.
// Useful for tests and headless runs that should not emit telemetry.
func NewNoopProviders() *Providers {
	return &Providers{
		MeterProvider:  noop.NewMeterProvider(),
		TracerProvider: nooptrace.NewTracerProvider(),
	}
}

// Meter returns a named meter from the provider.
func (p *Providers) Meter(name string) metric.Meter {
	return p.MeterProvider.Meter(name)
}

// Tracer returns a named tracer from the provider.
func (p *Providers) Tracer(name string) trace.Tracer {
	return p.TracerProvider.Tracer(name)
}

// ---------------------------------------------------------------------------
// T126: OTel counters
// ---------------------------------------------------------------------------

const meterName = "gopher-code"

// Counters holds all named OTel counters used throughout a session.
// Source: bootstrap/state.ts — sessionCounter, locCounter, prCounter,
// commitCounter, costCounter, tokenCounter, codeEditToolDecisionCounter,
// activeTimeCounter
type Counters struct {
	Session              metric.Int64Counter
	LOC                  metric.Int64Counter
	PR                   metric.Int64Counter
	Commit               metric.Int64Counter
	Cost                 metric.Float64Counter
	Token                metric.Int64Counter
	CodeEditToolDecision metric.Int64Counter
	ActiveTime           metric.Float64Counter
}

// NewCounters creates all named counters from the given providers.
func NewCounters(p *Providers) (*Counters, error) {
	m := p.Meter(meterName)

	session, err := m.Int64Counter("session.count",
		metric.WithDescription("Number of sessions started"))
	if err != nil {
		return nil, err
	}
	loc, err := m.Int64Counter("code.loc",
		metric.WithDescription("Lines of code changed"))
	if err != nil {
		return nil, err
	}
	pr, err := m.Int64Counter("code.pr",
		metric.WithDescription("Pull requests created"))
	if err != nil {
		return nil, err
	}
	commit, err := m.Int64Counter("code.commit",
		metric.WithDescription("Commits created"))
	if err != nil {
		return nil, err
	}
	cost, err := m.Float64Counter("api.cost_usd",
		metric.WithDescription("Cumulative API cost in USD"))
	if err != nil {
		return nil, err
	}
	token, err := m.Int64Counter("api.tokens",
		metric.WithDescription("Total tokens consumed"))
	if err != nil {
		return nil, err
	}
	codeEdit, err := m.Int64Counter("tool.code_edit_decision",
		metric.WithDescription("Code edit tool decisions"))
	if err != nil {
		return nil, err
	}
	activeTime, err := m.Float64Counter("session.active_time_ms",
		metric.WithDescription("Active session time in milliseconds"))
	if err != nil {
		return nil, err
	}

	return &Counters{
		Session:              session,
		LOC:                  loc,
		PR:                   pr,
		Commit:               commit,
		Cost:                 cost,
		Token:                token,
		CodeEditToolDecision: codeEdit,
		ActiveTime:           activeTime,
	}, nil
}

// ---------------------------------------------------------------------------
// T127: statsStore.observe
// ---------------------------------------------------------------------------

// StatsStore collects named observations for later export.
// Source: bootstrap/state.ts — statsStore.observe(name, value)
type StatsStore struct {
	mu      sync.Mutex
	entries map[string][]float64
}

// NewStatsStore creates an empty StatsStore.
func NewStatsStore() *StatsStore {
	return &StatsStore{
		entries: make(map[string][]float64),
	}
}

// Observe records a named metric value.
func (s *StatsStore) Observe(name string, value float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[name] = append(s.entries[name], value)
}

// Get returns all recorded values for a metric name.
func (s *StatsStore) Get(name string) []float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]float64, len(s.entries[name]))
	copy(cp, s.entries[name])
	return cp
}

// Names returns all metric names that have observations.
func (s *StatsStore) Names() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	names := make([]string, 0, len(s.entries))
	for k := range s.entries {
		names = append(names, k)
	}
	return names
}

// Clear removes all observations.
func (s *StatsStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = make(map[string][]float64)
}

// ---------------------------------------------------------------------------
// Telemetry — convenience aggregation of providers + counters + stats
// ---------------------------------------------------------------------------

// Telemetry is the top-level telemetry handle for a session.
type Telemetry struct {
	Providers *Providers
	Counters  *Counters
	Stats     *StatsStore
}

// Init creates a Telemetry instance using the global OTel providers.
func Init() (*Telemetry, error) {
	p := NewProviders()
	c, err := NewCounters(p)
	if err != nil {
		return nil, err
	}
	return &Telemetry{
		Providers: p,
		Counters:  c,
		Stats:     NewStatsStore(),
	}, nil
}

// InitNoop creates a no-op Telemetry instance (tests, headless).
func InitNoop() *Telemetry {
	p := NewNoopProviders()
	c, _ := NewCounters(p) // noop counters never error
	return &Telemetry{
		Providers: p,
		Counters:  c,
		Stats:     NewStatsStore(),
	}
}

// RecordSession increments the session counter.
func (t *Telemetry) RecordSession(ctx context.Context) {
	t.Counters.Session.Add(ctx, 1)
}

// RecordLOC records lines of code changed.
func (t *Telemetry) RecordLOC(ctx context.Context, lines int64) {
	t.Counters.LOC.Add(ctx, lines)
}

// RecordCost records API cost in USD.
func (t *Telemetry) RecordCost(ctx context.Context, usd float64) {
	t.Counters.Cost.Add(ctx, usd)
}

// RecordTokens records token usage.
func (t *Telemetry) RecordTokens(ctx context.Context, count int64) {
	t.Counters.Token.Add(ctx, count)
}
