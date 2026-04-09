package notifications

import (
	"testing"
)

func TestNotificationQueue_Empty(t *testing.T) {
	q := NewNotificationQueue()
	if !q.IsEmpty() {
		t.Error("new queue should be empty")
	}
	if q.Current != nil {
		t.Error("current should be nil")
	}
}

func TestNotificationQueue_AddPromotesToCurrent(t *testing.T) {
	q := NewNotificationQueue()
	cmd := q.Add(Notification{Key: "a", Message: "hello", Priority: PriorityHigh})
	if q.Current == nil || q.Current.Key != "a" {
		t.Error("first add should promote to current")
	}
	if len(q.Queue) != 0 {
		t.Error("queue should be empty after promotion")
	}
	if cmd == nil {
		t.Error("should return timeout cmd")
	}
}

func TestNotificationQueue_SecondAddQueues(t *testing.T) {
	q := NewNotificationQueue()
	q.Add(Notification{Key: "a", Priority: PriorityHigh})
	q.Add(Notification{Key: "b", Priority: PriorityLow})
	if q.Current.Key != "a" {
		t.Error("current should still be first")
	}
	if len(q.Queue) != 1 || q.Queue[0].Key != "b" {
		t.Error("second should be queued")
	}
}

func TestNotificationQueue_Dedup(t *testing.T) {
	q := NewNotificationQueue()
	q.Add(Notification{Key: "a", Priority: PriorityHigh})
	q.Add(Notification{Key: "a", Priority: PriorityHigh}) // duplicate
	if len(q.Queue) != 0 {
		t.Error("duplicate key should not be queued twice")
	}

	q.Add(Notification{Key: "b", Priority: PriorityLow})
	q.Add(Notification{Key: "b", Priority: PriorityLow}) // duplicate in queue
	if len(q.Queue) != 1 {
		t.Errorf("expected 1 queued, got %d", len(q.Queue))
	}
}

func TestNotificationQueue_ImmediatePreempts(t *testing.T) {
	q := NewNotificationQueue()
	q.Add(Notification{Key: "a", Priority: PriorityLow})
	q.Add(Notification{Key: "urgent", Priority: PriorityImmediate})

	if q.Current == nil || q.Current.Key != "urgent" {
		t.Error("immediate should replace current")
	}
	// Original "a" should be re-queued
	found := false
	for _, n := range q.Queue {
		if n.Key == "a" {
			found = true
		}
	}
	if !found {
		t.Error("preempted notification should be re-queued")
	}
}

func TestNotificationQueue_HandleExpired(t *testing.T) {
	q := NewNotificationQueue()
	q.Add(Notification{Key: "a", Priority: PriorityHigh})
	q.Add(Notification{Key: "b", Priority: PriorityLow})

	cmd := q.HandleExpired("a")
	if q.Current == nil || q.Current.Key != "b" {
		t.Error("after expiry, next should be promoted")
	}
	if cmd == nil {
		t.Error("should return timeout cmd for new current")
	}
}

func TestNotificationQueue_HandleExpiredStale(t *testing.T) {
	q := NewNotificationQueue()
	q.Add(Notification{Key: "a", Priority: PriorityHigh})

	// Stale expiry for wrong key
	cmd := q.HandleExpired("nonexistent")
	if cmd != nil {
		t.Error("stale expiry should return nil cmd")
	}
	if q.Current.Key != "a" {
		t.Error("current should be unchanged")
	}
}

func TestNotificationQueue_Remove(t *testing.T) {
	q := NewNotificationQueue()
	q.Add(Notification{Key: "a", Priority: PriorityHigh})
	q.Add(Notification{Key: "b", Priority: PriorityLow})

	// Remove current
	cmd := q.Remove("a")
	if q.Current == nil || q.Current.Key != "b" {
		t.Error("after removing current, next should promote")
	}
	if cmd == nil {
		t.Error("should return timeout cmd")
	}
}

func TestNotificationQueue_RemoveFromQueue(t *testing.T) {
	q := NewNotificationQueue()
	q.Add(Notification{Key: "a", Priority: PriorityHigh})
	q.Add(Notification{Key: "b", Priority: PriorityLow})
	q.Add(Notification{Key: "c", Priority: PriorityLow})

	q.Remove("b")
	if len(q.Queue) != 1 || q.Queue[0].Key != "c" {
		t.Errorf("queue should only have c, got %v", q.Queue)
	}
}

func TestNotificationQueue_PriorityOrdering(t *testing.T) {
	q := NewNotificationQueue()
	q.Add(Notification{Key: "current", Priority: PriorityHigh})
	q.Add(Notification{Key: "low", Priority: PriorityLow})
	q.Add(Notification{Key: "high", Priority: PriorityHigh})

	// Expire current → high should be promoted (higher priority than low)
	q.HandleExpired("current")
	if q.Current == nil || q.Current.Key != "high" {
		t.Errorf("higher priority should be promoted first, got %v", q.Current)
	}

	q.HandleExpired("high")
	if q.Current == nil || q.Current.Key != "low" {
		t.Errorf("low should be promoted last, got %v", q.Current)
	}
}

func TestNotificationQueue_Invalidation(t *testing.T) {
	q := NewNotificationQueue()
	q.Add(Notification{Key: "a", Priority: PriorityHigh})
	q.Add(Notification{Key: "b", Priority: PriorityLow})
	q.Add(Notification{Key: "c", Priority: PriorityLow})

	// Add notification that invalidates "a" (current) and "b" (queued)
	q.Add(Notification{Key: "d", Priority: PriorityLow, Invalidates: []string{"a", "b"}})

	// "a" should be cleared, "b" removed from queue
	if q.Current != nil && q.Current.Key == "a" {
		t.Error("invalidated current should be cleared")
	}
	for _, n := range q.Queue {
		if n.Key == "b" {
			t.Error("invalidated queued notification should be removed")
		}
	}
}

func TestNotificationQueue_IsEmpty(t *testing.T) {
	q := NewNotificationQueue()
	if !q.IsEmpty() {
		t.Error("should be empty initially")
	}

	q.Add(Notification{Key: "a", Priority: PriorityLow})
	if q.IsEmpty() {
		t.Error("should not be empty after add")
	}

	q.HandleExpired("a")
	if !q.IsEmpty() {
		t.Error("should be empty after expiry with no queue")
	}
}

func TestPriorityRank(t *testing.T) {
	if priorityRank(PriorityImmediate) >= priorityRank(PriorityHigh) {
		t.Error("immediate should rank higher than high")
	}
	if priorityRank(PriorityHigh) >= priorityRank(PriorityLow) {
		t.Error("high should rank higher than low")
	}
}
