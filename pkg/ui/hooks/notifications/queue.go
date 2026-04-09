package notifications

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

// Source: context/notifications.tsx
//
// NotificationQueue manages a priority queue of notifications with one
// "current" visible notification and a waiting queue. It handles:
//   - Priority ordering (immediate > high > medium > low)
//   - Timeout-based auto-dismiss via tea.Cmd
//   - Deduplication by key
//   - Invalidation (a notification can remove others by key)
//   - Fold/merge (combine notifications with the same key)

// DefaultTimeoutMs is the default notification display duration.
const DefaultTimeoutMs = 8000

// priorityRank returns a numeric rank where lower = more urgent.
// PriorityImmediate(2) > PriorityHigh(1) > PriorityLow(0) in the Priority enum,
// so we invert: higher Priority value = lower rank = more urgent.
func priorityRank(p Priority) int {
	return -int(p)
}

// NotificationQueue manages the current and queued notifications.
type NotificationQueue struct {
	Current *Notification
	Queue   []Notification
}

// NewNotificationQueue creates an empty notification queue.
func NewNotificationQueue() *NotificationQueue {
	return &NotificationQueue{}
}

// NotificationExpiredMsg is sent when a notification's timeout expires.
type NotificationExpiredMsg struct {
	Key string
}

// Add adds a notification to the queue. For immediate priority, it
// replaces the current notification. For others, it's queued.
// Returns a tea.Cmd if a timeout should be started.
func (q *NotificationQueue) Add(n Notification) tea.Cmd {
	timeout := time.Duration(n.TimeoutMs) * time.Millisecond
	if n.TimeoutMs == 0 {
		timeout = time.Duration(DefaultTimeoutMs) * time.Millisecond
	}

	// Handle invalidation: remove invalidated notifications
	if len(n.Invalidates) > 0 {
		invalidSet := make(map[string]bool, len(n.Invalidates))
		for _, k := range n.Invalidates {
			invalidSet[k] = true
		}
		// Remove from queue
		filtered := q.Queue[:0]
		for _, qn := range q.Queue {
			if !invalidSet[qn.Key] {
				filtered = append(filtered, qn)
			}
		}
		q.Queue = filtered
		// Clear current if invalidated
		if q.Current != nil && invalidSet[q.Current.Key] {
			q.Current = nil
		}
	}

	// Immediate priority: show right away
	if n.Priority == PriorityImmediate {
		// Re-queue current if it's not immediate
		if q.Current != nil && q.Current.Priority != PriorityImmediate {
			q.Queue = append(q.Queue, *q.Current)
		}
		q.Current = &n
		return timeoutCmd(n.Key, timeout)
	}

	// Dedup: don't add if already current or queued with same key
	if q.Current != nil && q.Current.Key == n.Key {
		return nil
	}
	for _, qn := range q.Queue {
		if qn.Key == n.Key {
			return nil
		}
	}

	q.Queue = append(q.Queue, n)

	// If nothing is current, promote from queue
	if q.Current == nil {
		return q.processQueue()
	}
	return nil
}

// Remove removes a notification by key (from current or queue).
// Returns a tea.Cmd if the next notification should be shown.
func (q *NotificationQueue) Remove(key string) tea.Cmd {
	if q.Current != nil && q.Current.Key == key {
		q.Current = nil
		return q.processQueue()
	}

	filtered := q.Queue[:0]
	for _, n := range q.Queue {
		if n.Key != key {
			filtered = append(filtered, n)
		}
	}
	q.Queue = filtered
	return nil
}

// HandleExpired processes a NotificationExpiredMsg. Clears the current
// notification if it matches and promotes the next one.
func (q *NotificationQueue) HandleExpired(key string) tea.Cmd {
	if q.Current == nil || q.Current.Key != key {
		return nil // stale timeout
	}
	q.Current = nil
	return q.processQueue()
}

// processQueue promotes the highest-priority queued notification to current.
func (q *NotificationQueue) processQueue() tea.Cmd {
	if len(q.Queue) == 0 || q.Current != nil {
		return nil
	}

	next := q.getNext()
	if next == nil {
		return nil
	}

	// Remove from queue
	filtered := q.Queue[:0]
	for _, n := range q.Queue {
		if n.Key != next.Key {
			filtered = append(filtered, n)
		}
	}
	q.Queue = filtered
	q.Current = next

	timeout := time.Duration(next.TimeoutMs) * time.Millisecond
	if next.TimeoutMs == 0 {
		timeout = time.Duration(DefaultTimeoutMs) * time.Millisecond
	}
	return timeoutCmd(next.Key, timeout)
}

// getNext returns the highest-priority notification from the queue.
func (q *NotificationQueue) getNext() *Notification {
	if len(q.Queue) == 0 {
		return nil
	}
	best := &q.Queue[0]
	for i := 1; i < len(q.Queue); i++ {
		if priorityRank(q.Queue[i].Priority) < priorityRank(best.Priority) {
			best = &q.Queue[i]
		}
	}
	cp := *best
	return &cp
}

// IsEmpty returns true if there's no current or queued notification.
func (q *NotificationQueue) IsEmpty() bool {
	return q.Current == nil && len(q.Queue) == 0
}

// timeoutCmd returns a tea.Cmd that sends NotificationExpiredMsg after duration.
func timeoutCmd(key string, d time.Duration) tea.Cmd {
	return tea.Tick(d, func(_ time.Time) tea.Msg {
		return NotificationExpiredMsg{Key: key}
	})
}
