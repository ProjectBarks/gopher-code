package query

import "testing"

// Source: utils/messageQueueManager.ts

func TestCommandQueue(t *testing.T) {

	t.Run("enqueue_and_dequeue", func(t *testing.T) {
		// Source: messageQueueManager.ts:128-193
		q := NewCommandQueue()
		q.Enqueue(QueuedCommand{Value: "hello", Priority: PriorityNext})
		q.Enqueue(QueuedCommand{Value: "world", Priority: PriorityNext})

		cmd := q.Dequeue()
		if cmd == nil || cmd.Value != "hello" {
			t.Errorf("expected 'hello', got %v", cmd)
		}
		cmd = q.Dequeue()
		if cmd == nil || cmd.Value != "world" {
			t.Errorf("expected 'world', got %v", cmd)
		}
		cmd = q.Dequeue()
		if cmd != nil {
			t.Error("expected nil from empty queue")
		}
	})

	t.Run("priority_ordering", func(t *testing.T) {
		// Source: messageQueueManager.ts:151-155 — now > next > later
		q := NewCommandQueue()
		q.Enqueue(QueuedCommand{Value: "later", Priority: PriorityLater})
		q.Enqueue(QueuedCommand{Value: "next", Priority: PriorityNext})
		q.Enqueue(QueuedCommand{Value: "now", Priority: PriorityNow})

		cmd := q.Dequeue()
		if cmd.Value != "now" {
			t.Errorf("expected 'now' first, got %q", cmd.Value)
		}
		cmd = q.Dequeue()
		if cmd.Value != "next" {
			t.Errorf("expected 'next' second, got %q", cmd.Value)
		}
		cmd = q.Dequeue()
		if cmd.Value != "later" {
			t.Errorf("expected 'later' third, got %q", cmd.Value)
		}
	})

	t.Run("fifo_within_priority", func(t *testing.T) {
		q := NewCommandQueue()
		q.Enqueue(QueuedCommand{Value: "a", Priority: PriorityNext})
		q.Enqueue(QueuedCommand{Value: "b", Priority: PriorityNext})
		q.Enqueue(QueuedCommand{Value: "c", Priority: PriorityNext})

		for _, expected := range []string{"a", "b", "c"} {
			cmd := q.Dequeue()
			if cmd.Value != expected {
				t.Errorf("expected %q, got %q", expected, cmd.Value)
			}
		}
	})

	t.Run("peek_does_not_remove", func(t *testing.T) {
		// Source: messageQueueManager.ts:219-238
		q := NewCommandQueue()
		q.Enqueue(QueuedCommand{Value: "hello", Priority: PriorityNext})

		cmd := q.Peek()
		if cmd == nil || cmd.Value != "hello" {
			t.Error("peek should return command")
		}
		if q.Len() != 1 {
			t.Error("peek should not remove command")
		}
	})

	t.Run("dequeue_filtered", func(t *testing.T) {
		// Source: messageQueueManager.ts:169-193
		q := NewCommandQueue()
		q.Enqueue(QueuedCommand{Value: "agent-1", Priority: PriorityNext, AgentID: "agent-1"})
		q.Enqueue(QueuedCommand{Value: "main", Priority: PriorityNext})

		// Filter for main thread only (no AgentID)
		cmd := q.DequeueFiltered(func(c QueuedCommand) bool {
			return c.AgentID == ""
		})
		if cmd == nil || cmd.Value != "main" {
			t.Errorf("expected 'main', got %v", cmd)
		}

		// Agent command still in queue
		if q.Len() != 1 {
			t.Errorf("expected 1 remaining, got %d", q.Len())
		}
	})

	t.Run("clear", func(t *testing.T) {
		// Source: messageQueueManager.ts:322-328
		q := NewCommandQueue()
		q.Enqueue(QueuedCommand{Value: "a", Priority: PriorityNext})
		q.Enqueue(QueuedCommand{Value: "b", Priority: PriorityNext})
		q.Clear()
		if q.Len() != 0 {
			t.Errorf("expected 0 after clear, got %d", q.Len())
		}
	})

	t.Run("dequeue_all", func(t *testing.T) {
		// Source: messageQueueManager.ts:199-213
		q := NewCommandQueue()
		q.Enqueue(QueuedCommand{Value: "a", Priority: PriorityNext})
		q.Enqueue(QueuedCommand{Value: "b", Priority: PriorityLater})

		all := q.DequeueAll()
		if len(all) != 2 {
			t.Fatalf("expected 2, got %d", len(all))
		}
		if q.Len() != 0 {
			t.Error("queue should be empty after dequeue all")
		}
	})

	t.Run("subscribe_notified", func(t *testing.T) {
		q := NewCommandQueue()
		notified := 0
		q.Subscribe(func() { notified++ })

		q.Enqueue(QueuedCommand{Value: "test", Priority: PriorityNext})
		if notified != 1 {
			t.Errorf("expected 1 notification, got %d", notified)
		}

		q.Dequeue()
		if notified != 2 {
			t.Errorf("expected 2 notifications, got %d", notified)
		}
	})

	t.Run("has_commands", func(t *testing.T) {
		// Source: messageQueueManager.ts:104-106
		q := NewCommandQueue()
		if q.HasCommands() {
			t.Error("empty queue should not have commands")
		}
		q.Enqueue(QueuedCommand{Value: "test", Priority: PriorityNext})
		if !q.HasCommands() {
			t.Error("should have commands after enqueue")
		}
	})
}
