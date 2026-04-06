package commands

import (
	"sync"

	tea "charm.land/bubbletea/v2"
)

// Priority determines dequeue order: Now > Next > Later.
// Within the same priority commands are FIFO.
type Priority int

const (
	PriorityNow   Priority = 0 // immediate (e.g. keybinding-triggered)
	PriorityNext  Priority = 1 // user-initiated input
	PriorityLater Priority = 2 // system notifications
)

// QueuedCommand is a command waiting to be executed.
type QueuedCommand struct {
	// Value is typically a slash command string like "/commit".
	Value    string
	Priority Priority
	// Mode distinguishes command origins (e.g. "prompt", "bash",
	// "task-notification"). Slash and bash commands are processed
	// individually; other modes with the same value are batched.
	Mode string
}

// QueueDrainedMsg is sent when a command finishes executing and the
// processor should check for the next item. It carries the command that
// just completed.
type QueueDrainedMsg struct {
	Completed QueuedCommand
}

// Queue is a priority-aware FIFO command queue. It is safe for concurrent
// use from multiple goroutines.
type Queue struct {
	mu    sync.Mutex
	items []QueuedCommand
}

// NewQueue returns an empty Queue.
func NewQueue() *Queue {
	return &Queue{}
}

// Enqueue adds a command to the queue. If no priority is set the default
// is PriorityNext.
func (q *Queue) Enqueue(cmd QueuedCommand) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.items = append(q.items, cmd)
}

// Dequeue removes and returns the highest-priority command (lowest
// Priority value). Within the same priority, FIFO order is preserved.
// Returns the command and true, or the zero value and false if empty.
func (q *Queue) Dequeue() (QueuedCommand, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) == 0 {
		return QueuedCommand{}, false
	}

	bestIdx := 0
	for i := 1; i < len(q.items); i++ {
		if q.items[i].Priority < q.items[bestIdx].Priority {
			bestIdx = i
		}
	}

	cmd := q.items[bestIdx]
	q.items = append(q.items[:bestIdx], q.items[bestIdx+1:]...)
	return cmd, true
}

// Peek returns the highest-priority command without removing it.
func (q *Queue) Peek() (QueuedCommand, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) == 0 {
		return QueuedCommand{}, false
	}

	bestIdx := 0
	for i := 1; i < len(q.items); i++ {
		if q.items[i].Priority < q.items[bestIdx].Priority {
			bestIdx = i
		}
	}
	return q.items[bestIdx], true
}

// Len returns the number of commands in the queue.
func (q *Queue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}

// Clear removes all commands.
func (q *Queue) Clear() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.items = q.items[:0]
}

// Snapshot returns a copy of the current queue contents in priority order.
func (q *Queue) Snapshot() []QueuedCommand {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]QueuedCommand, len(q.items))
	copy(out, q.items)
	return out
}

// -----------------------------------------------------------------------
// QueueProcessor — bubbletea Model that drains the queue one command at
// a time: dequeue -> execute -> wait for QueueDrainedMsg -> dequeue next.
// -----------------------------------------------------------------------

// ExecuteFunc is called by the processor to run a dequeued command.
// It must return a tea.Cmd that eventually produces a QueueDrainedMsg
// (directly or via a chain of messages) so the processor knows when to
// advance.
type ExecuteFunc func(cmd QueuedCommand) tea.Cmd

// QueueProcessor drains a Queue serially. It starts processing when it
// receives a ProcessQueueMsg and continues until the queue is empty. While
// a command is executing (waiting for QueueDrainedMsg) new items may be
// enqueued and will be picked up on the next drain cycle.
type QueueProcessor struct {
	queue      *Queue
	execute    ExecuteFunc
	processing bool
}

// ProcessQueueMsg tells the processor to start (or re-check) the queue.
type ProcessQueueMsg struct{}

// NewQueueProcessor creates a processor that drains q using exec.
func NewQueueProcessor(q *Queue, exec ExecuteFunc) *QueueProcessor {
	return &QueueProcessor{
		queue:   q,
		execute: exec,
	}
}

// IsProcessing returns true if a command is currently executing.
func (p *QueueProcessor) IsProcessing() bool {
	return p.processing
}

// Init implements tea.Model.
func (p *QueueProcessor) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (p *QueueProcessor) Update(msg tea.Msg) tea.Cmd {
	switch msg.(type) {
	case ProcessQueueMsg:
		return p.tryDrain()
	case QueueDrainedMsg:
		p.processing = false
		return p.tryDrain()
	}
	return nil
}

// View implements tea.Model.
func (p *QueueProcessor) View() string {
	return ""
}

// tryDrain dequeues the next command and begins executing it, or does
// nothing if the queue is empty or a command is already in flight.
func (p *QueueProcessor) tryDrain() tea.Cmd {
	if p.processing {
		return nil
	}
	cmd, ok := p.queue.Dequeue()
	if !ok {
		return nil
	}
	p.processing = true
	return p.execute(cmd)
}
