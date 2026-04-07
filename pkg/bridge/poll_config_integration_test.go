package bridge

import (
	"testing"
	"time"
)

// TestDynamicPollConfig_IntegrationLifecycle exercises the full adaptive
// backoff lifecycle: construction with defaults, empty-poll backoff ramp,
// work-reset, capacity switching, and config hot-swap.
func TestDynamicPollConfig_IntegrationLifecycle(t *testing.T) {
	// Step 1: Construct with defaults (single-session mode).
	d := NewDynamicPollConfig(DefaultPollConfig, false)
	if d.ConsecutiveEmptyPolls() != 0 {
		t.Fatalf("initial empty poll count = %d, want 0", d.ConsecutiveEmptyPolls())
	}

	// Step 2: Fresh config with no empty polls returns the configured interval.
	baseDelay := d.NextPollDelay()
	if baseDelay != DefaultPollConfig.PollIntervalNotAtCapacity {
		t.Fatalf("initial NextPollDelay = %v, want %v", baseDelay, DefaultPollConfig.PollIntervalNotAtCapacity)
	}

	// Step 3: Record several empty polls — backoff should ramp up.
	// Use a config with a large not-at-capacity interval so the backoff ramp
	// is visible (default 2s is smaller than BaseDelay so backoff would
	// exceed it immediately).
	cfg := DefaultPollConfig
	cfg.PollIntervalNotAtCapacity = 120 * time.Second
	d.SetConfig(cfg)

	var delays []time.Duration
	for i := 0; i < 5; i++ {
		d.RecordEmptyPoll()
		delays = append(delays, d.NextPollDelay())
	}

	// Verify monotonic non-decrease (allowing for jitter).
	for i := 1; i < len(delays); i++ {
		if delays[i]+JitterRange < delays[i-1] {
			t.Errorf("backoff not increasing: delay[%d]=%v, delay[%d]=%v", i-1, delays[i-1], i, delays[i])
		}
	}

	// All backoff delays should be below the configured interval.
	for i, d := range delays {
		if d >= cfg.PollIntervalNotAtCapacity {
			t.Errorf("delay[%d]=%v should be below configured %v during backoff", i, d, cfg.PollIntervalNotAtCapacity)
		}
	}

	// Step 4: RecordWork resets backoff — delay returns to configured interval.
	d.RecordWork()
	if d.ConsecutiveEmptyPolls() != 0 {
		t.Fatalf("after RecordWork: empty poll count = %d, want 0", d.ConsecutiveEmptyPolls())
	}
	afterWork := d.NextPollDelay()
	if afterWork != cfg.PollIntervalNotAtCapacity {
		t.Fatalf("after RecordWork: NextPollDelay = %v, want %v", afterWork, cfg.PollIntervalNotAtCapacity)
	}

	// Step 5: Switch to CapacityFull — at-capacity interval, no backoff.
	d.SetCapacity(CapacityFull)
	for i := 0; i < 10; i++ {
		d.RecordEmptyPoll()
	}
	fullDelay := d.NextPollDelay()
	if fullDelay != cfg.PollIntervalAtCapacity {
		t.Fatalf("CapacityFull: NextPollDelay = %v, want %v", fullDelay, cfg.PollIntervalAtCapacity)
	}

	// Step 6: Switch back to CapacityNone with accumulated empty polls —
	// backoff should apply again.
	d.SetCapacity(CapacityNone)
	noneDelay := d.NextPollDelay()
	if noneDelay >= cfg.PollIntervalNotAtCapacity {
		t.Fatalf("CapacityNone with backoff: delay %v should be below configured %v", noneDelay, cfg.PollIntervalNotAtCapacity)
	}

	// Step 7: Hot-swap config changes the base interval.
	newCfg := cfg
	newCfg.PollIntervalNotAtCapacity = 250 * time.Second
	d.SetConfig(newCfg)
	d.RecordWork() // reset backoff
	swappedDelay := d.NextPollDelay()
	if swappedDelay != 250*time.Second {
		t.Fatalf("after SetConfig: NextPollDelay = %v, want 250s", swappedDelay)
	}
}

// TestDynamicPollConfig_IntegrationMultisession exercises the multisession
// path through the same lifecycle.
func TestDynamicPollConfig_IntegrationMultisession(t *testing.T) {
	cfg := PollIntervalConfig{
		PollIntervalNotAtCapacity:               1000 * time.Millisecond,
		PollIntervalAtCapacity:                  500_000 * time.Millisecond,
		MultisessionPollIntervalNotAtCapacity:   3000 * time.Millisecond,
		MultisessionPollIntervalPartialCapacity: 4000 * time.Millisecond,
		MultisessionPollIntervalAtCapacity:      700_000 * time.Millisecond,
	}
	d := NewDynamicPollConfig(cfg, true)

	// CapacityNone → multisession not-at-capacity interval.
	if got := d.NextPollDelay(); got != 3000*time.Millisecond {
		t.Errorf("multisession CapacityNone: got %v, want 3s", got)
	}

	// CapacityPartial → multisession partial-capacity interval.
	d.SetCapacity(CapacityPartial)
	if got := d.NextPollDelay(); got != 4000*time.Millisecond {
		t.Errorf("multisession CapacityPartial: got %v, want 4s", got)
	}

	// CapacityFull → multisession at-capacity interval.
	d.SetCapacity(CapacityFull)
	if got := d.NextPollDelay(); got != 700_000*time.Millisecond {
		t.Errorf("multisession CapacityFull: got %v, want 700s", got)
	}

	// Empty polls at CapacityFull do not affect delay.
	for i := 0; i < 5; i++ {
		d.RecordEmptyPoll()
	}
	if got := d.NextPollDelay(); got != 700_000*time.Millisecond {
		t.Errorf("multisession CapacityFull after empty polls: got %v, want 700s", got)
	}
}
