// Poll interval configuration defaults.
// Source: src/bridge/pollConfigDefaults.ts + src/bridge/pollConfig.ts
package bridge

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// Backoff constants for poll retry
// ---------------------------------------------------------------------------

const (
	// BaseDelay is the initial delay between poll retries.
	BaseDelay = 5 * time.Second

	// MaxDelay is the upper bound on exponential backoff (before jitter).
	MaxDelay = 30 * time.Second

	// JitterRange is the maximum random jitter added to each delay.
	JitterRange = 3 * time.Second

	// EmptyPollLogInterval is the number of consecutive empty polls before
	// logging a status message.
	EmptyPollLogInterval = 100

	// KeepAliveInterval is the interval between keep-alive frames sent to
	// the bridge server (5 minutes).
	KeepAliveInterval = 300 * time.Second
)

// ---------------------------------------------------------------------------
// PollIntervalConfig — mirrors TS PollIntervalConfig type
// ---------------------------------------------------------------------------

// PollIntervalConfig holds tunable poll intervals for the bridge.
// JSON tags use snake_case millisecond keys to match the TS wire format.
type PollIntervalConfig struct {
	PollIntervalNotAtCapacity                 time.Duration `json:"-"`
	PollIntervalAtCapacity                    time.Duration `json:"-"`
	NonExclusiveHeartbeatInterval             time.Duration `json:"-"`
	MultisessionPollIntervalNotAtCapacity     time.Duration `json:"-"`
	MultisessionPollIntervalPartialCapacity   time.Duration `json:"-"`
	MultisessionPollIntervalAtCapacity        time.Duration `json:"-"`
	ReclaimOlderThan                          time.Duration `json:"-"`
	SessionKeepaliveInterval                  time.Duration `json:"-"`
}

// pollIntervalConfigWire is the JSON wire representation (milliseconds).
type pollIntervalConfigWire struct {
	PollIntervalMSNotAtCapacity               int64 `json:"poll_interval_ms_not_at_capacity"`
	PollIntervalMSAtCapacity                  int64 `json:"poll_interval_ms_at_capacity"`
	NonExclusiveHeartbeatIntervalMS           int64 `json:"non_exclusive_heartbeat_interval_ms"`
	MultisessionPollIntervalMSNotAtCapacity   int64 `json:"multisession_poll_interval_ms_not_at_capacity"`
	MultisessionPollIntervalMSPartialCapacity int64 `json:"multisession_poll_interval_ms_partial_capacity"`
	MultisessionPollIntervalMSAtCapacity      int64 `json:"multisession_poll_interval_ms_at_capacity"`
	ReclaimOlderThanMS                        int64 `json:"reclaim_older_than_ms"`
	SessionKeepaliveIntervalV2MS              int64 `json:"session_keepalive_interval_v2_ms"`
}

// MarshalJSON encodes the config as millisecond values with snake_case keys.
func (c PollIntervalConfig) MarshalJSON() ([]byte, error) {
	return json.Marshal(pollIntervalConfigWire{
		PollIntervalMSNotAtCapacity:               c.PollIntervalNotAtCapacity.Milliseconds(),
		PollIntervalMSAtCapacity:                  c.PollIntervalAtCapacity.Milliseconds(),
		NonExclusiveHeartbeatIntervalMS:           c.NonExclusiveHeartbeatInterval.Milliseconds(),
		MultisessionPollIntervalMSNotAtCapacity:   c.MultisessionPollIntervalNotAtCapacity.Milliseconds(),
		MultisessionPollIntervalMSPartialCapacity: c.MultisessionPollIntervalPartialCapacity.Milliseconds(),
		MultisessionPollIntervalMSAtCapacity:      c.MultisessionPollIntervalAtCapacity.Milliseconds(),
		ReclaimOlderThanMS:                        c.ReclaimOlderThan.Milliseconds(),
		SessionKeepaliveIntervalV2MS:              c.SessionKeepaliveInterval.Milliseconds(),
	})
}

// UnmarshalJSON decodes millisecond values from snake_case JSON keys.
func (c *PollIntervalConfig) UnmarshalJSON(data []byte) error {
	var w pollIntervalConfigWire
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	c.PollIntervalNotAtCapacity = time.Duration(w.PollIntervalMSNotAtCapacity) * time.Millisecond
	c.PollIntervalAtCapacity = time.Duration(w.PollIntervalMSAtCapacity) * time.Millisecond
	c.NonExclusiveHeartbeatInterval = time.Duration(w.NonExclusiveHeartbeatIntervalMS) * time.Millisecond
	c.MultisessionPollIntervalNotAtCapacity = time.Duration(w.MultisessionPollIntervalMSNotAtCapacity) * time.Millisecond
	c.MultisessionPollIntervalPartialCapacity = time.Duration(w.MultisessionPollIntervalMSPartialCapacity) * time.Millisecond
	c.MultisessionPollIntervalAtCapacity = time.Duration(w.MultisessionPollIntervalMSAtCapacity) * time.Millisecond
	c.ReclaimOlderThan = time.Duration(w.ReclaimOlderThanMS) * time.Millisecond
	c.SessionKeepaliveInterval = time.Duration(w.SessionKeepaliveIntervalV2MS) * time.Millisecond
	return nil
}

// DefaultPollConfig matches TS DEFAULT_POLL_CONFIG from pollConfigDefaults.ts.
var DefaultPollConfig = PollIntervalConfig{
	PollIntervalNotAtCapacity:               2000 * time.Millisecond,
	PollIntervalAtCapacity:                  600_000 * time.Millisecond,
	NonExclusiveHeartbeatInterval:           0,
	MultisessionPollIntervalNotAtCapacity:   2000 * time.Millisecond,
	MultisessionPollIntervalPartialCapacity: 2000 * time.Millisecond,
	MultisessionPollIntervalAtCapacity:      600_000 * time.Millisecond,
	ReclaimOlderThan:                        5000 * time.Millisecond,
	SessionKeepaliveInterval:                120_000 * time.Millisecond,
}

// ComputeNextDelay returns the next poll delay using exponential backoff
// with additive jitter. The base delay doubles each attempt and is capped
// at MaxDelay; a random jitter in [0, JitterRange) is added.
func ComputeNextDelay(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}

	// Exponential backoff: BaseDelay * 2^attempt, capped at MaxDelay.
	base := BaseDelay
	if attempt < 63 { // avoid overflow
		shifted := time.Duration(1) << uint(attempt)
		base = BaseDelay * shifted
	} else {
		base = MaxDelay
	}
	if base > MaxDelay {
		base = MaxDelay
	}

	// Additive jitter in [0, JitterRange).
	jitter := time.Duration(rand.Int64N(int64(JitterRange)))

	return base + jitter
}

// ---------------------------------------------------------------------------
// Validation — mirrors TS pollConfig.ts schema + cross-field refines
// ---------------------------------------------------------------------------

// Validation error messages matching TS verbatim strings.
const (
	ErrZeroOrAtLeast100 = "must be 0 (disabled) or ≥100ms"
	ErrSingleSessionLiveness = "at-capacity liveness requires non_exclusive_heartbeat_interval_ms > 0 or poll_interval_ms_at_capacity > 0"
	ErrMultisessionLiveness  = "at-capacity liveness requires non_exclusive_heartbeat_interval_ms > 0 or multisession_poll_interval_ms_at_capacity > 0"
)

// pollIntervalConfigValidationWire uses pointers for optional fields so we can
// distinguish "absent from JSON" (nil → apply default) from "explicitly 0"
// (valid value meaning disabled).
type pollIntervalConfigValidationWire struct {
	PollIntervalMSNotAtCapacity               int64  `json:"poll_interval_ms_not_at_capacity"`
	PollIntervalMSAtCapacity                  int64  `json:"poll_interval_ms_at_capacity"`
	NonExclusiveHeartbeatIntervalMS           *int64 `json:"non_exclusive_heartbeat_interval_ms"`
	MultisessionPollIntervalMSNotAtCapacity   *int64 `json:"multisession_poll_interval_ms_not_at_capacity"`
	MultisessionPollIntervalMSPartialCapacity *int64 `json:"multisession_poll_interval_ms_partial_capacity"`
	MultisessionPollIntervalMSAtCapacity      *int64 `json:"multisession_poll_interval_ms_at_capacity"`
	ReclaimOlderThanMS                        *int64 `json:"reclaim_older_than_ms"`
	SessionKeepaliveIntervalV2MS              *int64 `json:"session_keepalive_interval_v2_ms"`
}

func i64OrDefault(p *int64, def int64) int64 {
	if p == nil {
		return def
	}
	return *p
}

// ValidatePollConfig unmarshals raw JSON into a PollIntervalConfig, applying
// the same validation rules as the TS Zod schema. On any violation the entire
// object is rejected and DefaultPollConfig is returned (no partial trust).
func ValidatePollConfig(raw json.RawMessage) (PollIntervalConfig, error) {
	var w pollIntervalConfigValidationWire
	if err := json.Unmarshal(raw, &w); err != nil {
		return DefaultPollConfig, err
	}

	// Apply defaults for optional fields (TS uses .default()). nil = absent.
	heartbeatMS := i64OrDefault(w.NonExclusiveHeartbeatIntervalMS, 0)
	msNotAtCap := i64OrDefault(w.MultisessionPollIntervalMSNotAtCapacity, DefaultPollConfig.MultisessionPollIntervalNotAtCapacity.Milliseconds())
	msPartialCap := i64OrDefault(w.MultisessionPollIntervalMSPartialCapacity, DefaultPollConfig.MultisessionPollIntervalPartialCapacity.Milliseconds())
	msAtCap := i64OrDefault(w.MultisessionPollIntervalMSAtCapacity, DefaultPollConfig.MultisessionPollIntervalAtCapacity.Milliseconds())
	reclaimMS := i64OrDefault(w.ReclaimOlderThanMS, DefaultPollConfig.ReclaimOlderThan.Milliseconds())
	keepaliveMS := i64OrDefault(w.SessionKeepaliveIntervalV2MS, DefaultPollConfig.SessionKeepaliveInterval.Milliseconds())

	// Single-field validation: min(100) floors on seek-work intervals.
	if w.PollIntervalMSNotAtCapacity < 100 {
		return DefaultPollConfig, fmt.Errorf("poll_interval_ms_not_at_capacity: min 100, got %d", w.PollIntervalMSNotAtCapacity)
	}
	if msNotAtCap < 100 {
		return DefaultPollConfig, fmt.Errorf("multisession_poll_interval_ms_not_at_capacity: min 100, got %d", msNotAtCap)
	}
	if msPartialCap < 100 {
		return DefaultPollConfig, fmt.Errorf("multisession_poll_interval_ms_partial_capacity: min 100, got %d", msPartialCap)
	}

	// 0-or-≥100 refinement on at-capacity intervals.
	if w.PollIntervalMSAtCapacity != 0 && w.PollIntervalMSAtCapacity < 100 {
		return DefaultPollConfig, fmt.Errorf("poll_interval_ms_at_capacity: %s", ErrZeroOrAtLeast100)
	}
	if msAtCap != 0 && msAtCap < 100 {
		return DefaultPollConfig, fmt.Errorf("multisession_poll_interval_ms_at_capacity: %s", ErrZeroOrAtLeast100)
	}

	// Heartbeat interval floor.
	if heartbeatMS < 0 {
		return DefaultPollConfig, fmt.Errorf("non_exclusive_heartbeat_interval_ms: min 0, got %d", heartbeatMS)
	}
	// reclaim_older_than_ms min(1).
	if reclaimMS < 1 {
		return DefaultPollConfig, fmt.Errorf("reclaim_older_than_ms: min 1, got %d", reclaimMS)
	}
	// session_keepalive_interval_v2_ms min(0).
	if keepaliveMS < 0 {
		return DefaultPollConfig, fmt.Errorf("session_keepalive_interval_v2_ms: min 0, got %d", keepaliveMS)
	}

	// Cross-field refines: at least one at-capacity liveness mechanism.
	if heartbeatMS <= 0 && w.PollIntervalMSAtCapacity <= 0 {
		return DefaultPollConfig, fmt.Errorf("%s", ErrSingleSessionLiveness)
	}
	if heartbeatMS <= 0 && msAtCap <= 0 {
		return DefaultPollConfig, fmt.Errorf("%s", ErrMultisessionLiveness)
	}

	cfg := PollIntervalConfig{
		PollIntervalNotAtCapacity:               time.Duration(w.PollIntervalMSNotAtCapacity) * time.Millisecond,
		PollIntervalAtCapacity:                  time.Duration(w.PollIntervalMSAtCapacity) * time.Millisecond,
		NonExclusiveHeartbeatInterval:           time.Duration(heartbeatMS) * time.Millisecond,
		MultisessionPollIntervalNotAtCapacity:   time.Duration(msNotAtCap) * time.Millisecond,
		MultisessionPollIntervalPartialCapacity: time.Duration(msPartialCap) * time.Millisecond,
		MultisessionPollIntervalAtCapacity:      time.Duration(msAtCap) * time.Millisecond,
		ReclaimOlderThan:                        time.Duration(reclaimMS) * time.Millisecond,
		SessionKeepaliveInterval:                time.Duration(keepaliveMS) * time.Millisecond,
	}
	return cfg, nil
}

// ---------------------------------------------------------------------------
// CapacityState represents the bridge capacity level.
// ---------------------------------------------------------------------------

// CapacityState represents the bridge's current capacity level.
type CapacityState int

const (
	// CapacityNone means no active sessions (not at capacity).
	CapacityNone CapacityState = iota
	// CapacityPartial means some sessions active but not at max (multisession).
	CapacityPartial
	// CapacityFull means at maximum capacity.
	CapacityFull
)

// ---------------------------------------------------------------------------
// DynamicPollConfig — adaptive poll interval management
// ---------------------------------------------------------------------------

// DynamicPollConfig adjusts poll intervals based on capacity state and
// consecutive empty poll tracking. It implements adaptive backoff: empty
// polls cause delay to increase (up to the configured interval), and
// receiving work resets the backoff. Thread-safe.
//
// Mirrors the TS bridge's poll interval selection logic from replBridge.ts
// (single-session) and bridgeMain.ts (multisession).
type DynamicPollConfig struct {
	mu                    sync.Mutex
	config                PollIntervalConfig
	multisession          bool
	capacity              CapacityState
	consecutiveEmptyPolls int
}

// NewDynamicPollConfig creates a DynamicPollConfig. When multisession is true,
// the multisession_poll_interval_ms_* fields are used; otherwise the
// single-session poll_interval_ms_* fields apply.
func NewDynamicPollConfig(config PollIntervalConfig, multisession bool) *DynamicPollConfig {
	return &DynamicPollConfig{
		config:       config,
		multisession: multisession,
	}
}

// SetConfig hot-swaps the underlying PollIntervalConfig (e.g. on GrowthBook
// refresh). Does not reset backoff state.
func (d *DynamicPollConfig) SetConfig(config PollIntervalConfig) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.config = config
}

// Config returns the current PollIntervalConfig.
func (d *DynamicPollConfig) Config() PollIntervalConfig {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.config
}

// SetCapacity updates the capacity state.
func (d *DynamicPollConfig) SetCapacity(state CapacityState) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.capacity = state
}

// Capacity returns the current capacity state.
func (d *DynamicPollConfig) Capacity() CapacityState {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.capacity
}

// RecordEmptyPoll increments the consecutive empty poll counter and returns
// the new count. The caller can use count%EmptyPollLogInterval==0 to
// throttle debug logging.
func (d *DynamicPollConfig) RecordEmptyPoll() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.consecutiveEmptyPolls++
	return d.consecutiveEmptyPolls
}

// RecordWork resets the consecutive empty poll counter (work received).
func (d *DynamicPollConfig) RecordWork() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.consecutiveEmptyPolls = 0
}

// ConsecutiveEmptyPolls returns the current count.
func (d *DynamicPollConfig) ConsecutiveEmptyPolls() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.consecutiveEmptyPolls
}

// NextPollDelay returns the poll delay for the current capacity state.
// At CapacityFull it returns the at-capacity interval; at CapacityPartial
// (multisession only) the partial-capacity interval; otherwise the
// not-at-capacity interval. The delay incorporates adaptive backoff:
// consecutive empty polls cause the delay to ramp from BaseDelay up to
// the configured interval, resetting when work is received.
func (d *DynamicPollConfig) NextPollDelay() time.Duration {
	d.mu.Lock()
	defer d.mu.Unlock()

	var base time.Duration

	switch d.capacity {
	case CapacityFull:
		if d.multisession {
			base = d.config.MultisessionPollIntervalAtCapacity
		} else {
			base = d.config.PollIntervalAtCapacity
		}
	case CapacityPartial:
		if d.multisession {
			base = d.config.MultisessionPollIntervalPartialCapacity
		} else {
			// Single-session has no partial state; treat as not-at-capacity.
			base = d.config.PollIntervalNotAtCapacity
		}
	default: // CapacityNone
		if d.multisession {
			base = d.config.MultisessionPollIntervalNotAtCapacity
		} else {
			base = d.config.PollIntervalNotAtCapacity
		}
	}

	// Adaptive backoff: on consecutive empty polls, ramp up from BaseDelay
	// toward the configured interval. After receiving work (counter=0),
	// use the full configured interval immediately.
	if d.consecutiveEmptyPolls > 0 && d.capacity != CapacityFull {
		backoff := ComputeNextDelay(d.consecutiveEmptyPolls - 1)
		if backoff < base {
			return backoff
		}
	}

	return base
}

// ShouldLogEmptyPoll returns true when the empty poll count warrants
// a debug log (first empty poll or every EmptyPollLogInterval polls).
func (d *DynamicPollConfig) ShouldLogEmptyPoll() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.consecutiveEmptyPolls == 1 ||
		d.consecutiveEmptyPolls%EmptyPollLogInterval == 0
}
