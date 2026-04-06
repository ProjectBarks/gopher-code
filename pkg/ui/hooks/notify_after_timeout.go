package hooks

import (
	"sync"
	"sync/atomic"
	"time"
)

// DefaultInteractionThreshold is the idle threshold for notification (6 seconds).
// Source: hooks/useNotifyAfterTimeout.ts DEFAULT_INTERACTION_THRESHOLD_MS
const DefaultInteractionThreshold = 6 * time.Second

// Notifier is the callback signature for sending a desktop/terminal notification.
type Notifier func(message, notificationType string)

// InteractionTracker records the most recent user interaction timestamp.
// Safe for concurrent use. Updated by the main input batch loop.
type InteractionTracker struct {
	lastTime atomic.Int64 // unix nanoseconds
}

// NewInteractionTracker creates a tracker initialized to the current time.
func NewInteractionTracker() *InteractionTracker {
	t := &InteractionTracker{}
	t.Touch()
	return t
}

// Touch records a user interaction at the current time.
func (t *InteractionTracker) Touch() {
	t.lastTime.Store(time.Now().UnixNano())
}

// SinceLastInteraction returns the duration since the last interaction.
func (t *InteractionTracker) SinceLastInteraction() time.Duration {
	last := time.Unix(0, t.lastTime.Load())
	return time.Since(last)
}

// NotifyAfterTimeout fires a one-shot notification if the user has been
// idle for longer than the threshold. The timer checks periodically and
// fires at most once.
// Source: hooks/useNotifyAfterTimeout.ts
type NotifyAfterTimeout struct {
	mu        sync.Mutex
	tracker   *InteractionTracker
	threshold time.Duration
	notify    Notifier
	timer     *time.Ticker
	done      chan struct{}
	fired     bool
	testMode  bool
}

// NewNotifyAfterTimeout creates and starts the idle notification timer.
// On creation, it resets the interaction tracker to prevent stale timestamps
// from causing premature notifications (mirrors the TS mount-time reset).
func NewNotifyAfterTimeout(
	tracker *InteractionTracker,
	message string,
	notificationType string,
	notify Notifier,
) *NotifyAfterTimeout {
	return newNotifyAfterTimeout(tracker, message, notificationType, notify, DefaultInteractionThreshold, false)
}

func newNotifyAfterTimeout(
	tracker *InteractionTracker,
	message string,
	notificationType string,
	notify Notifier,
	threshold time.Duration,
	testMode bool,
) *NotifyAfterTimeout {
	// Reset interaction time on creation to avoid premature fire.
	tracker.Touch()

	n := &NotifyAfterTimeout{
		tracker:   tracker,
		threshold: threshold,
		notify:    notify,
		done:      make(chan struct{}),
		testMode:  testMode,
	}

	if testMode {
		return n
	}

	n.timer = time.NewTicker(threshold)
	go n.run(message, notificationType)
	return n
}

func (n *NotifyAfterTimeout) run(message, notificationType string) {
	for {
		select {
		case <-n.done:
			return
		case <-n.timer.C:
			n.mu.Lock()
			if !n.fired && n.tracker.SinceLastInteraction() >= n.threshold {
				n.fired = true
				n.mu.Unlock()
				n.notify(message, notificationType)
				n.timer.Stop()
				return
			}
			n.mu.Unlock()
		}
	}
}

// HasFired returns true if the notification has already been sent.
func (n *NotifyAfterTimeout) HasFired() bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.fired
}

// Stop cancels the timer. Safe to call multiple times.
func (n *NotifyAfterTimeout) Stop() {
	select {
	case <-n.done:
		return
	default:
		close(n.done)
		if n.timer != nil {
			n.timer.Stop()
		}
	}
}
