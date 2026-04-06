package commands

import (
	"sync"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Queue tests
// ---------------------------------------------------------------------------

func TestQueue_FIFO(t *testing.T) {
	q := NewQueue()
	q.Enqueue(QueuedCommand{Value: "a", Priority: PriorityNext})
	q.Enqueue(QueuedCommand{Value: "b", Priority: PriorityNext})
	q.Enqueue(QueuedCommand{Value: "c", Priority: PriorityNext})

	got := drainValues(q)
	assert.Equal(t, []string{"a", "b", "c"}, got)
}

func TestQueue_PriorityOrder(t *testing.T) {
	q := NewQueue()
	q.Enqueue(QueuedCommand{Value: "later", Priority: PriorityLater})
	q.Enqueue(QueuedCommand{Value: "next", Priority: PriorityNext})
	q.Enqueue(QueuedCommand{Value: "now", Priority: PriorityNow})

	got := drainValues(q)
	assert.Equal(t, []string{"now", "next", "later"}, got)
}

func TestQueue_MixedPriorityFIFO(t *testing.T) {
	q := NewQueue()
	q.Enqueue(QueuedCommand{Value: "next-1", Priority: PriorityNext})
	q.Enqueue(QueuedCommand{Value: "now-1", Priority: PriorityNow})
	q.Enqueue(QueuedCommand{Value: "next-2", Priority: PriorityNext})
	q.Enqueue(QueuedCommand{Value: "now-2", Priority: PriorityNow})

	got := drainValues(q)
	// Now items first (FIFO within priority), then next items (FIFO).
	assert.Equal(t, []string{"now-1", "now-2", "next-1", "next-2"}, got)
}

func TestQueue_DequeueEmpty(t *testing.T) {
	q := NewQueue()
	_, ok := q.Dequeue()
	assert.False(t, ok)
}

func TestQueue_Peek(t *testing.T) {
	q := NewQueue()
	q.Enqueue(QueuedCommand{Value: "later", Priority: PriorityLater})
	q.Enqueue(QueuedCommand{Value: "now", Priority: PriorityNow})

	cmd, ok := q.Peek()
	require.True(t, ok)
	assert.Equal(t, "now", cmd.Value)

	// Peek does not remove.
	assert.Equal(t, 2, q.Len())
}

func TestQueue_Clear(t *testing.T) {
	q := NewQueue()
	q.Enqueue(QueuedCommand{Value: "a"})
	q.Enqueue(QueuedCommand{Value: "b"})
	q.Clear()
	assert.Equal(t, 0, q.Len())
}

func TestQueue_Snapshot(t *testing.T) {
	q := NewQueue()
	q.Enqueue(QueuedCommand{Value: "a", Priority: PriorityNext})
	q.Enqueue(QueuedCommand{Value: "b", Priority: PriorityNow})

	snap := q.Snapshot()
	assert.Len(t, snap, 2)
	// Snapshot returns insertion order, not priority order.
	assert.Equal(t, "a", snap[0].Value)
	assert.Equal(t, "b", snap[1].Value)

	// Modifying snapshot should not affect queue.
	snap[0].Value = "mutated"
	cmd, _ := q.Peek()
	assert.Equal(t, "b", cmd.Value) // "b" has PriorityNow
}

func TestQueue_ConcurrentSafe(t *testing.T) {
	q := NewQueue()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			q.Enqueue(QueuedCommand{Value: "x", Priority: PriorityNext})
		}()
	}
	wg.Wait()
	assert.Equal(t, 100, q.Len())
}

// ---------------------------------------------------------------------------
// QueueProcessor tests
// ---------------------------------------------------------------------------

func TestQueueProcessor_DrainOrdering(t *testing.T) {
	q := NewQueue()
	q.Enqueue(QueuedCommand{Value: "/first", Priority: PriorityNext})
	q.Enqueue(QueuedCommand{Value: "/second", Priority: PriorityNext})
	q.Enqueue(QueuedCommand{Value: "/third", Priority: PriorityNext})

	var executed []string
	exec := func(cmd QueuedCommand) tea.Cmd {
		executed = append(executed, cmd.Value)
		return func() tea.Msg { return QueueDrainedMsg{Completed: cmd} }
	}

	p := NewQueueProcessor(q, exec)

	// Kick off processing.
	cmd := p.Update(ProcessQueueMsg{})
	require.NotNil(t, cmd)

	// Simulate the message loop: execute the cmd, feed the resulting
	// QueueDrainedMsg back into the processor.
	for cmd != nil {
		msg := cmd()
		cmd = p.Update(msg)
	}

	assert.Equal(t, []string{"/first", "/second", "/third"}, executed)
	assert.Equal(t, 0, q.Len())
}

func TestQueueProcessor_DrainPriorityOrder(t *testing.T) {
	q := NewQueue()
	q.Enqueue(QueuedCommand{Value: "/later", Priority: PriorityLater})
	q.Enqueue(QueuedCommand{Value: "/now", Priority: PriorityNow})
	q.Enqueue(QueuedCommand{Value: "/next", Priority: PriorityNext})

	var executed []string
	exec := func(cmd QueuedCommand) tea.Cmd {
		executed = append(executed, cmd.Value)
		return func() tea.Msg { return QueueDrainedMsg{Completed: cmd} }
	}

	p := NewQueueProcessor(q, exec)
	cmd := p.Update(ProcessQueueMsg{})
	for cmd != nil {
		msg := cmd()
		cmd = p.Update(msg)
	}

	assert.Equal(t, []string{"/now", "/next", "/later"}, executed)
}

func TestQueueProcessor_QueueWhileExecuting(t *testing.T) {
	// Verify that commands enqueued while another is executing are buffered
	// and processed after the current one completes.
	q := NewQueue()
	q.Enqueue(QueuedCommand{Value: "/first", Priority: PriorityNext})

	var executed []string
	exec := func(cmd QueuedCommand) tea.Cmd {
		executed = append(executed, cmd.Value)
		return func() tea.Msg { return QueueDrainedMsg{Completed: cmd} }
	}

	p := NewQueueProcessor(q, exec)

	// Start processing: dequeues /first.
	cmd := p.Update(ProcessQueueMsg{})
	require.NotNil(t, cmd)
	assert.True(t, p.IsProcessing())

	// While /first is executing, enqueue two more commands.
	q.Enqueue(QueuedCommand{Value: "/second", Priority: PriorityNext})
	q.Enqueue(QueuedCommand{Value: "/third", Priority: PriorityNext})

	// Trying to start again while processing should be a no-op.
	noop := p.Update(ProcessQueueMsg{})
	assert.Nil(t, noop)

	// Complete /first, which should trigger drain of /second.
	msg := cmd()
	cmd = p.Update(msg)
	require.NotNil(t, cmd)
	assert.Equal(t, []string{"/first", "/second"}, executed)

	// Complete /second, drain /third.
	msg = cmd()
	cmd = p.Update(msg)
	require.NotNil(t, cmd)

	// Complete /third, nothing left.
	msg = cmd()
	cmd = p.Update(msg)
	assert.Nil(t, cmd)
	assert.False(t, p.IsProcessing())

	assert.Equal(t, []string{"/first", "/second", "/third"}, executed)
}

func TestQueueProcessor_EmptyQueue(t *testing.T) {
	q := NewQueue()
	exec := func(cmd QueuedCommand) tea.Cmd {
		t.Fatal("should not execute anything")
		return nil
	}

	p := NewQueueProcessor(q, exec)
	cmd := p.Update(ProcessQueueMsg{})
	assert.Nil(t, cmd, "empty queue should produce no command")
	assert.False(t, p.IsProcessing())
}

func TestQueueProcessor_IgnoresUnrelatedMsg(t *testing.T) {
	q := NewQueue()
	q.Enqueue(QueuedCommand{Value: "/x"})
	exec := func(cmd QueuedCommand) tea.Cmd { return nil }

	p := NewQueueProcessor(q, exec)
	// An unrelated message type should be ignored.
	cmd := p.Update(ExecuteCommandMsg{Command: "/x"})
	assert.Nil(t, cmd)
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func drainValues(q *Queue) []string {
	var out []string
	for {
		cmd, ok := q.Dequeue()
		if !ok {
			break
		}
		out = append(out, cmd.Value)
	}
	return out
}
