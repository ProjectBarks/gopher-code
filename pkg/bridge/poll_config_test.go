package bridge

import (
	"encoding/json"
	"math"
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
