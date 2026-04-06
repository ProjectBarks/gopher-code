package bridge

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Default value parity — all values must match TS pollConfigDefaults.ts
// ---------------------------------------------------------------------------

func TestDefaultPollConfig_PollIntervalNotAtCapacity(t *testing.T) {
	want := 2000 * time.Millisecond
	if DefaultPollConfig.PollIntervalNotAtCapacity != want {
		t.Fatalf("PollIntervalNotAtCapacity = %v, want %v", DefaultPollConfig.PollIntervalNotAtCapacity, want)
	}
}

func TestDefaultPollConfig_PollIntervalAtCapacity(t *testing.T) {
	want := 600_000 * time.Millisecond
	if DefaultPollConfig.PollIntervalAtCapacity != want {
		t.Fatalf("PollIntervalAtCapacity = %v, want %v", DefaultPollConfig.PollIntervalAtCapacity, want)
	}
}

func TestDefaultPollConfig_NonExclusiveHeartbeatInterval(t *testing.T) {
	if DefaultPollConfig.NonExclusiveHeartbeatInterval != 0 {
		t.Fatalf("NonExclusiveHeartbeatInterval = %v, want 0 (disabled)", DefaultPollConfig.NonExclusiveHeartbeatInterval)
	}
}

func TestDefaultPollConfig_MultisessionNotAtCapacity(t *testing.T) {
	want := 2000 * time.Millisecond
	if DefaultPollConfig.MultisessionPollIntervalNotAtCapacity != want {
		t.Fatalf("MultisessionPollIntervalNotAtCapacity = %v, want %v", DefaultPollConfig.MultisessionPollIntervalNotAtCapacity, want)
	}
}

func TestDefaultPollConfig_MultisessionPartialCapacity(t *testing.T) {
	want := 2000 * time.Millisecond
	if DefaultPollConfig.MultisessionPollIntervalPartialCapacity != want {
		t.Fatalf("MultisessionPollIntervalPartialCapacity = %v, want %v", DefaultPollConfig.MultisessionPollIntervalPartialCapacity, want)
	}
}

func TestDefaultPollConfig_MultisessionAtCapacity(t *testing.T) {
	want := 600_000 * time.Millisecond
	if DefaultPollConfig.MultisessionPollIntervalAtCapacity != want {
		t.Fatalf("MultisessionPollIntervalAtCapacity = %v, want %v", DefaultPollConfig.MultisessionPollIntervalAtCapacity, want)
	}
}

func TestDefaultPollConfig_ReclaimOlderThan(t *testing.T) {
	want := 5000 * time.Millisecond
	if DefaultPollConfig.ReclaimOlderThan != want {
		t.Fatalf("ReclaimOlderThan = %v, want %v", DefaultPollConfig.ReclaimOlderThan, want)
	}
}

func TestDefaultPollConfig_SessionKeepaliveInterval(t *testing.T) {
	want := 120_000 * time.Millisecond
	if DefaultPollConfig.SessionKeepaliveInterval != want {
		t.Fatalf("SessionKeepaliveInterval = %v, want %v", DefaultPollConfig.SessionKeepaliveInterval, want)
	}
}

// ---------------------------------------------------------------------------
// Backoff constants
// ---------------------------------------------------------------------------

func TestBackoffConstants(t *testing.T) {
	cases := []struct {
		name string
		got  time.Duration
		want time.Duration
	}{
		{"BaseDelay", BaseDelay, 5 * time.Second},
		{"MaxDelay", MaxDelay, 30 * time.Second},
		{"JitterRange", JitterRange, 3 * time.Second},
		{"KeepAliveInterval", KeepAliveInterval, 300 * time.Second},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
}

func TestEmptyPollLogInterval(t *testing.T) {
	if EmptyPollLogInterval != 100 {
		t.Fatalf("EmptyPollLogInterval = %d, want 100", EmptyPollLogInterval)
	}
}

// ---------------------------------------------------------------------------
// JSON serialisation — snake_case wire keys
// ---------------------------------------------------------------------------

func TestPollIntervalConfig_JSONKeys(t *testing.T) {
	b, err := json.Marshal(DefaultPollConfig)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	wantKeys := []string{
		"poll_interval_ms_not_at_capacity",
		"poll_interval_ms_at_capacity",
		"non_exclusive_heartbeat_interval_ms",
		"multisession_poll_interval_ms_not_at_capacity",
		"multisession_poll_interval_ms_partial_capacity",
		"multisession_poll_interval_ms_at_capacity",
		"reclaim_older_than_ms",
		"session_keepalive_interval_v2_ms",
	}
	for _, key := range wantKeys {
		if _, ok := raw[key]; !ok {
			t.Errorf("missing JSON key %q", key)
		}
	}
}

func TestPollIntervalConfig_JSONValues(t *testing.T) {
	b, err := json.Marshal(DefaultPollConfig)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]int64
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		key  string
		want int64
	}{
		{"poll_interval_ms_not_at_capacity", 2000},
		{"poll_interval_ms_at_capacity", 600_000},
		{"non_exclusive_heartbeat_interval_ms", 0},
		{"multisession_poll_interval_ms_not_at_capacity", 2000},
		{"multisession_poll_interval_ms_partial_capacity", 2000},
		{"multisession_poll_interval_ms_at_capacity", 600_000},
		{"reclaim_older_than_ms", 5000},
		{"session_keepalive_interval_v2_ms", 120_000},
	}
	for _, c := range cases {
		if raw[c.key] != c.want {
			t.Errorf("JSON %s = %d, want %d", c.key, raw[c.key], c.want)
		}
	}
}

// ---------------------------------------------------------------------------
// ComputeNextDelay — exponential backoff + jitter
// ---------------------------------------------------------------------------

func TestComputeNextDelay_FirstAttempt(t *testing.T) {
	d := ComputeNextDelay(0)
	// attempt 0: BaseDelay * 2^0 = 5s, plus jitter in [0, 3s)
	if d < BaseDelay || d >= BaseDelay+JitterRange {
		t.Fatalf("attempt 0: delay %v not in [%v, %v)", d, BaseDelay, BaseDelay+JitterRange)
	}
}

func TestComputeNextDelay_ExponentialGrowth(t *testing.T) {
	// attempt 1: base = 5s * 2^1 = 10s
	// attempt 2: base = 5s * 2^2 = 20s
	// attempt 3: base = 5s * 2^3 = 40s → capped at 30s
	for _, tc := range []struct {
		attempt  int
		wantBase time.Duration
	}{
		{1, 10 * time.Second},
		{2, 20 * time.Second},
	} {
		d := ComputeNextDelay(tc.attempt)
		if d < tc.wantBase || d >= tc.wantBase+JitterRange {
			t.Errorf("attempt %d: delay %v not in [%v, %v)", tc.attempt, d, tc.wantBase, tc.wantBase+JitterRange)
		}
	}
}

func TestComputeNextDelay_CapsAtMax(t *testing.T) {
	// attempt 3: 5s * 8 = 40s → capped at 30s
	for attempt := 3; attempt <= 10; attempt++ {
		d := ComputeNextDelay(attempt)
		if d < MaxDelay || d >= MaxDelay+JitterRange {
			t.Errorf("attempt %d: delay %v not in [%v, %v)", attempt, d, MaxDelay, MaxDelay+JitterRange)
		}
	}
}

func TestComputeNextDelay_JitterBounds(t *testing.T) {
	// Run many iterations to verify jitter stays in bounds.
	for i := 0; i < 1000; i++ {
		d := ComputeNextDelay(0)
		if d < BaseDelay {
			t.Fatalf("delay %v below BaseDelay %v", d, BaseDelay)
		}
		if d >= BaseDelay+JitterRange {
			t.Fatalf("delay %v >= BaseDelay+JitterRange %v", d, BaseDelay+JitterRange)
		}
	}
}

func TestComputeNextDelay_NegativeAttempt(t *testing.T) {
	// Negative attempt should be treated like 0.
	d := ComputeNextDelay(-1)
	if d < BaseDelay || d >= BaseDelay+JitterRange {
		t.Fatalf("negative attempt: delay %v not in [%v, %v)", d, BaseDelay, BaseDelay+JitterRange)
	}
}

func TestComputeNextDelay_LargeAttempt(t *testing.T) {
	// Very large attempt number must not overflow; should cap at MaxDelay.
	d := ComputeNextDelay(math.MaxInt32)
	if d < MaxDelay || d >= MaxDelay+JitterRange {
		t.Fatalf("large attempt: delay %v not in [%v, %v)", d, MaxDelay, MaxDelay+JitterRange)
	}
}

// ---------------------------------------------------------------------------
// ValidatePollConfig — schema validation + cross-field refines
// ---------------------------------------------------------------------------

func validWireJSON() string {
	return `{
		"poll_interval_ms_not_at_capacity": 2000,
		"poll_interval_ms_at_capacity": 600000,
		"non_exclusive_heartbeat_interval_ms": 0,
		"multisession_poll_interval_ms_not_at_capacity": 2000,
		"multisession_poll_interval_ms_partial_capacity": 2000,
		"multisession_poll_interval_ms_at_capacity": 600000,
		"reclaim_older_than_ms": 5000,
		"session_keepalive_interval_v2_ms": 120000
	}`
}

func TestValidatePollConfig_ValidInput(t *testing.T) {
	cfg, err := ValidatePollConfig(json.RawMessage(validWireJSON()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.PollIntervalNotAtCapacity != 2000*time.Millisecond {
		t.Errorf("PollIntervalNotAtCapacity = %v, want 2s", cfg.PollIntervalNotAtCapacity)
	}
	if cfg.PollIntervalAtCapacity != 600_000*time.Millisecond {
		t.Errorf("PollIntervalAtCapacity = %v, want 600s", cfg.PollIntervalAtCapacity)
	}
}

func TestValidatePollConfig_InvalidJSON(t *testing.T) {
	cfg, err := ValidatePollConfig(json.RawMessage(`{bad`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if cfg != DefaultPollConfig {
		t.Error("expected DefaultPollConfig on invalid JSON")
	}
}

func TestValidatePollConfig_Min100Floor(t *testing.T) {
	// poll_interval_ms_not_at_capacity below 100 → reject
	raw := `{"poll_interval_ms_not_at_capacity": 50, "poll_interval_ms_at_capacity": 600000}`
	_, err := ValidatePollConfig(json.RawMessage(raw))
	if err == nil {
		t.Fatal("expected error for poll_interval_ms_not_at_capacity < 100")
	}
}

func TestValidatePollConfig_ZeroOrAtLeast100_Reject(t *testing.T) {
	// poll_interval_ms_at_capacity = 10 (1–99 range) → reject
	raw := `{"poll_interval_ms_not_at_capacity": 2000, "poll_interval_ms_at_capacity": 10}`
	_, err := ValidatePollConfig(json.RawMessage(raw))
	if err == nil {
		t.Fatal("expected error for at_capacity value in 1-99 range")
	}
	if !strings.Contains(err.Error(), ErrZeroOrAtLeast100) {
		t.Errorf("error %q should contain %q", err, ErrZeroOrAtLeast100)
	}
}

func TestValidatePollConfig_ZeroOrAtLeast100_ZeroOK(t *testing.T) {
	// at_capacity = 0 is valid when heartbeat > 0
	raw := `{
		"poll_interval_ms_not_at_capacity": 2000,
		"poll_interval_ms_at_capacity": 0,
		"non_exclusive_heartbeat_interval_ms": 5000,
		"multisession_poll_interval_ms_at_capacity": 0
	}`
	_, err := ValidatePollConfig(json.RawMessage(raw))
	if err != nil {
		t.Fatalf("zero at_capacity with heartbeat should be valid: %v", err)
	}
}

func TestValidatePollConfig_SingleSessionLivenessRefine(t *testing.T) {
	// Both heartbeat=0 and at_capacity=0 → single-session liveness violation
	raw := `{
		"poll_interval_ms_not_at_capacity": 2000,
		"poll_interval_ms_at_capacity": 0,
		"non_exclusive_heartbeat_interval_ms": 0,
		"multisession_poll_interval_ms_at_capacity": 600000
	}`
	_, err := ValidatePollConfig(json.RawMessage(raw))
	if err == nil {
		t.Fatal("expected single-session liveness error")
	}
	if !strings.Contains(err.Error(), ErrSingleSessionLiveness) {
		t.Errorf("error %q should contain %q", err, ErrSingleSessionLiveness)
	}
}

func TestValidatePollConfig_MultisessionLivenessRefine(t *testing.T) {
	// heartbeat=0 and multisession_at_capacity=0 → multisession liveness violation
	raw := `{
		"poll_interval_ms_not_at_capacity": 2000,
		"poll_interval_ms_at_capacity": 600000,
		"non_exclusive_heartbeat_interval_ms": 0,
		"multisession_poll_interval_ms_at_capacity": 0
	}`
	_, err := ValidatePollConfig(json.RawMessage(raw))
	if err == nil {
		t.Fatal("expected multisession liveness error")
	}
	if !strings.Contains(err.Error(), ErrMultisessionLiveness) {
		t.Errorf("error %q should contain %q", err, ErrMultisessionLiveness)
	}
}

func TestValidatePollConfig_DefaultsAppliedForOptionalFields(t *testing.T) {
	// Only provide required fields, optional fields should get defaults
	raw := `{
		"poll_interval_ms_not_at_capacity": 2000,
		"poll_interval_ms_at_capacity": 600000
	}`
	cfg, err := ValidatePollConfig(json.RawMessage(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ReclaimOlderThan != DefaultPollConfig.ReclaimOlderThan {
		t.Errorf("ReclaimOlderThan = %v, want default %v", cfg.ReclaimOlderThan, DefaultPollConfig.ReclaimOlderThan)
	}
	if cfg.SessionKeepaliveInterval != DefaultPollConfig.SessionKeepaliveInterval {
		t.Errorf("SessionKeepaliveInterval = %v, want default %v", cfg.SessionKeepaliveInterval, DefaultPollConfig.SessionKeepaliveInterval)
	}
}

func TestValidatePollConfig_WholeObjectRejection(t *testing.T) {
	// One bad field → entire object rejected, returns DefaultPollConfig
	raw := `{"poll_interval_ms_not_at_capacity": 50, "poll_interval_ms_at_capacity": 600000}`
	cfg, err := ValidatePollConfig(json.RawMessage(raw))
	if err == nil {
		t.Fatal("expected error")
	}
	if cfg != DefaultPollConfig {
		t.Error("expected DefaultPollConfig on partial rejection")
	}
}

// ---------------------------------------------------------------------------
// DynamicPollConfig — adaptive backoff
// ---------------------------------------------------------------------------

func TestDynamicPollConfig_InitialState(t *testing.T) {
	d := NewDynamicPollConfig(DefaultPollConfig, false)
	if d.ConsecutiveEmptyPolls() != 0 {
		t.Error("expected 0 consecutive empty polls initially")
	}
	if d.Capacity() != CapacityNone {
		t.Error("expected CapacityNone initially")
	}
}

func TestDynamicPollConfig_EmptyPollIncrementsCounter(t *testing.T) {
	d := NewDynamicPollConfig(DefaultPollConfig, false)
	n := d.RecordEmptyPoll()
	if n != 1 {
		t.Errorf("RecordEmptyPoll returned %d, want 1", n)
	}
	n = d.RecordEmptyPoll()
	if n != 2 {
		t.Errorf("RecordEmptyPoll returned %d, want 2", n)
	}
}

func TestDynamicPollConfig_WorkResetsCounter(t *testing.T) {
	d := NewDynamicPollConfig(DefaultPollConfig, false)
	d.RecordEmptyPoll()
	d.RecordEmptyPoll()
	d.RecordEmptyPoll()
	d.RecordWork()
	if d.ConsecutiveEmptyPolls() != 0 {
		t.Errorf("expected 0 after RecordWork, got %d", d.ConsecutiveEmptyPolls())
	}
}

func TestDynamicPollConfig_AdaptiveDelayIncreasesOnEmptyPolls(t *testing.T) {
	// Use a large configured interval so backoff ramp is visible
	cfg := DefaultPollConfig
	cfg.PollIntervalNotAtCapacity = 60 * time.Second
	d := NewDynamicPollConfig(cfg, false)

	d.RecordEmptyPoll() // count=1, backoff attempt=0 → ~5s
	delay1 := d.NextPollDelay()
	d.RecordEmptyPoll() // count=2, backoff attempt=1 → ~10s
	delay2 := d.NextPollDelay()

	// Strip jitter for comparison: delay should be monotonically non-decreasing
	// in the base component. Allow JitterRange tolerance.
	if delay2+JitterRange < delay1 {
		t.Errorf("delay should increase: delay1=%v delay2=%v", delay1, delay2)
	}
	// Both should be less than configured interval
	if delay1 >= cfg.PollIntervalNotAtCapacity {
		t.Errorf("delay1 %v should be less than configured %v during backoff", delay1, cfg.PollIntervalNotAtCapacity)
	}
}

func TestDynamicPollConfig_AdaptiveDelayResetsOnWork(t *testing.T) {
	// Use a config with a large not-at-capacity interval so backoff is visible
	cfg := DefaultPollConfig
	cfg.PollIntervalNotAtCapacity = 60 * time.Second
	d := NewDynamicPollConfig(cfg, false)

	// Build up backoff
	for i := 0; i < 5; i++ {
		d.RecordEmptyPoll()
	}
	delayBefore := d.NextPollDelay()

	// Reset via work
	d.RecordWork()
	delayAfter := d.NextPollDelay()

	// After reset, should return configured interval (no backoff)
	if delayAfter != cfg.PollIntervalNotAtCapacity {
		t.Errorf("after work: delay = %v, want %v", delayAfter, cfg.PollIntervalNotAtCapacity)
	}
	// Before reset, delay should have been lower due to backoff ramp
	if delayBefore >= cfg.PollIntervalNotAtCapacity {
		t.Errorf("during backoff: delay %v should be less than configured %v", delayBefore, cfg.PollIntervalNotAtCapacity)
	}
}

func TestDynamicPollConfig_CapacityFullUsesAtCapacityInterval(t *testing.T) {
	d := NewDynamicPollConfig(DefaultPollConfig, false)
	d.SetCapacity(CapacityFull)

	delay := d.NextPollDelay()
	if delay != DefaultPollConfig.PollIntervalAtCapacity {
		t.Errorf("CapacityFull: delay = %v, want %v", delay, DefaultPollConfig.PollIntervalAtCapacity)
	}
}

func TestDynamicPollConfig_CapacityFullSkipsBackoff(t *testing.T) {
	// At capacity, backoff does not apply — always use at-capacity interval.
	d := NewDynamicPollConfig(DefaultPollConfig, false)
	d.SetCapacity(CapacityFull)
	for i := 0; i < 10; i++ {
		d.RecordEmptyPoll()
	}
	delay := d.NextPollDelay()
	if delay != DefaultPollConfig.PollIntervalAtCapacity {
		t.Errorf("CapacityFull with empty polls: delay = %v, want %v", delay, DefaultPollConfig.PollIntervalAtCapacity)
	}
}

func TestDynamicPollConfig_CapacitySwitchingChangesInterval(t *testing.T) {
	d := NewDynamicPollConfig(DefaultPollConfig, false)

	d.SetCapacity(CapacityNone)
	delayNone := d.NextPollDelay()

	d.SetCapacity(CapacityFull)
	delayFull := d.NextPollDelay()

	if delayNone == delayFull {
		t.Errorf("CapacityNone delay %v should differ from CapacityFull delay %v", delayNone, delayFull)
	}
	if delayFull != DefaultPollConfig.PollIntervalAtCapacity {
		t.Errorf("CapacityFull delay = %v, want %v", delayFull, DefaultPollConfig.PollIntervalAtCapacity)
	}
}

func TestDynamicPollConfig_MultisessionIntervalSelection(t *testing.T) {
	cfg := PollIntervalConfig{
		PollIntervalNotAtCapacity:               1000 * time.Millisecond,
		PollIntervalAtCapacity:                  500_000 * time.Millisecond,
		MultisessionPollIntervalNotAtCapacity:   3000 * time.Millisecond,
		MultisessionPollIntervalPartialCapacity: 4000 * time.Millisecond,
		MultisessionPollIntervalAtCapacity:      700_000 * time.Millisecond,
	}

	d := NewDynamicPollConfig(cfg, true) // multisession=true

	d.SetCapacity(CapacityNone)
	if got := d.NextPollDelay(); got != 3000*time.Millisecond {
		t.Errorf("multisession CapacityNone: got %v, want 3s", got)
	}

	d.SetCapacity(CapacityPartial)
	if got := d.NextPollDelay(); got != 4000*time.Millisecond {
		t.Errorf("multisession CapacityPartial: got %v, want 4s", got)
	}

	d.SetCapacity(CapacityFull)
	if got := d.NextPollDelay(); got != 700_000*time.Millisecond {
		t.Errorf("multisession CapacityFull: got %v, want 700s", got)
	}
}

func TestDynamicPollConfig_SingleSessionPartialTreatedAsNone(t *testing.T) {
	d := NewDynamicPollConfig(DefaultPollConfig, false) // single-session
	d.SetCapacity(CapacityPartial)
	if got := d.NextPollDelay(); got != DefaultPollConfig.PollIntervalNotAtCapacity {
		t.Errorf("single-session CapacityPartial: got %v, want %v", got, DefaultPollConfig.PollIntervalNotAtCapacity)
	}
}

func TestDynamicPollConfig_ShouldLogEmptyPoll(t *testing.T) {
	d := NewDynamicPollConfig(DefaultPollConfig, false)

	// First empty poll → should log
	d.RecordEmptyPoll()
	if !d.ShouldLogEmptyPoll() {
		t.Error("should log on first empty poll")
	}

	// Polls 2–99 → should not log
	for i := 2; i < EmptyPollLogInterval; i++ {
		d.RecordEmptyPoll()
	}
	if d.ShouldLogEmptyPoll() {
		t.Errorf("should not log at count %d", d.ConsecutiveEmptyPolls())
	}

	// Poll 100 → should log
	d.RecordEmptyPoll()
	if !d.ShouldLogEmptyPoll() {
		t.Errorf("should log at count %d (multiple of %d)", d.ConsecutiveEmptyPolls(), EmptyPollLogInterval)
	}
}

func TestDynamicPollConfig_SetConfigHotSwap(t *testing.T) {
	d := NewDynamicPollConfig(DefaultPollConfig, false)

	newCfg := DefaultPollConfig
	newCfg.PollIntervalNotAtCapacity = 5000 * time.Millisecond
	d.SetConfig(newCfg)

	if got := d.Config(); got.PollIntervalNotAtCapacity != 5000*time.Millisecond {
		t.Errorf("after SetConfig: PollIntervalNotAtCapacity = %v, want 5s", got.PollIntervalNotAtCapacity)
	}
}
