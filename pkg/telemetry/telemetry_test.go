package telemetry

import (
	"context"
	"sort"
	"testing"
)

// T126: OTel counters are created without error using noop providers.
func TestNewCounters_Noop(t *testing.T) {
	p := NewNoopProviders()
	c, err := NewCounters(p)
	if err != nil {
		t.Fatalf("NewCounters() error: %v", err)
	}
	if c.Session == nil {
		t.Error("Session counter should not be nil")
	}
	if c.LOC == nil {
		t.Error("LOC counter should not be nil")
	}
	if c.PR == nil {
		t.Error("PR counter should not be nil")
	}
	if c.Commit == nil {
		t.Error("Commit counter should not be nil")
	}
	if c.Cost == nil {
		t.Error("Cost counter should not be nil")
	}
	if c.Token == nil {
		t.Error("Token counter should not be nil")
	}
	if c.CodeEditToolDecision == nil {
		t.Error("CodeEditToolDecision counter should not be nil")
	}
	if c.ActiveTime == nil {
		t.Error("ActiveTime counter should not be nil")
	}
}

// T126: Counters can be incremented without panic.
func TestCounters_Increment(t *testing.T) {
	tel := InitNoop()
	ctx := context.Background()

	// These should not panic even with no-op exporters.
	tel.RecordSession(ctx)
	tel.RecordLOC(ctx, 42)
	tel.RecordCost(ctx, 0.05)
	tel.RecordTokens(ctx, 1000)
	tel.Counters.PR.Add(ctx, 1)
	tel.Counters.Commit.Add(ctx, 3)
	tel.Counters.CodeEditToolDecision.Add(ctx, 2)
	tel.Counters.ActiveTime.Add(ctx, 5000.0)
}

// T127: statsStore.observe records values and Get returns them.
func TestStatsStore_Observe(t *testing.T) {
	s := NewStatsStore()

	s.Observe("api_latency", 100.5)
	s.Observe("api_latency", 200.3)
	s.Observe("token_count", 42)

	vals := s.Get("api_latency")
	if len(vals) != 2 {
		t.Fatalf("Get(api_latency) len = %d, want 2", len(vals))
	}
	if vals[0] != 100.5 || vals[1] != 200.3 {
		t.Errorf("Get(api_latency) = %v, want [100.5, 200.3]", vals)
	}

	vals2 := s.Get("token_count")
	if len(vals2) != 1 || vals2[0] != 42 {
		t.Errorf("Get(token_count) = %v, want [42]", vals2)
	}

	// Unknown name returns empty slice
	vals3 := s.Get("unknown")
	if len(vals3) != 0 {
		t.Errorf("Get(unknown) = %v, want empty", vals3)
	}
}

// T127: statsStore.Names returns all observed metric names.
func TestStatsStore_Names(t *testing.T) {
	s := NewStatsStore()
	s.Observe("a", 1)
	s.Observe("b", 2)
	s.Observe("a", 3)

	names := s.Names()
	sort.Strings(names)
	if len(names) != 2 || names[0] != "a" || names[1] != "b" {
		t.Errorf("Names() = %v, want [a, b]", names)
	}
}

// T127: statsStore.Clear removes all observations.
func TestStatsStore_Clear(t *testing.T) {
	s := NewStatsStore()
	s.Observe("x", 1)
	s.Clear()

	if len(s.Names()) != 0 {
		t.Error("Clear() should remove all entries")
	}
	if len(s.Get("x")) != 0 {
		t.Error("Get after Clear should return empty")
	}
}

// T128: Providers created with noop have non-nil meter and tracer.
func TestProviders_Noop(t *testing.T) {
	p := NewNoopProviders()
	if p.MeterProvider == nil {
		t.Error("MeterProvider should not be nil")
	}
	if p.TracerProvider == nil {
		t.Error("TracerProvider should not be nil")
	}

	m := p.Meter("test")
	if m == nil {
		t.Error("Meter() should not return nil")
	}

	tr := p.Tracer("test")
	if tr == nil {
		t.Error("Tracer() should not return nil")
	}
}

// T128: Init creates a full Telemetry with global providers (no-op by default).
func TestInit(t *testing.T) {
	tel, err := Init()
	if err != nil {
		t.Fatalf("Init() error: %v", err)
	}
	if tel.Providers == nil {
		t.Error("Providers should not be nil")
	}
	if tel.Counters == nil {
		t.Error("Counters should not be nil")
	}
	if tel.Stats == nil {
		t.Error("Stats should not be nil")
	}
}

// T128: InitNoop creates a fully functional no-op Telemetry.
func TestInitNoop(t *testing.T) {
	tel := InitNoop()
	if tel.Providers == nil {
		t.Error("Providers should not be nil")
	}
	if tel.Counters == nil {
		t.Error("Counters should not be nil")
	}
	if tel.Stats == nil {
		t.Error("Stats should not be nil")
	}

	// Should not panic
	ctx := context.Background()
	tel.RecordSession(ctx)
	tel.Stats.Observe("test", 1.0)
}
