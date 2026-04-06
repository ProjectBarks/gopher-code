// Poll interval configuration defaults.
// Source: src/bridge/pollConfigDefaults.ts
package bridge

import (
	"encoding/json"
	"math/rand/v2"
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
