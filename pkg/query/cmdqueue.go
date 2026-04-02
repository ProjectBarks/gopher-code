package query

import "sync"

// Source: utils/messageQueueManager.ts

// QueuePriority determines dequeue order: Now > Next > Later.
// Source: utils/messageQueueManager.ts:151-155
type QueuePriority int

const (
	PriorityNow   QueuePriority = 0
	PriorityNext  QueuePriority = 1
	PriorityLater QueuePriority = 2
)

// QueuedCommand is a command buffered while the agent is thinking.
// Source: utils/messageQueueManager.ts:53
type QueuedCommand struct {
	Value    string        `json:"value"`
	Priority QueuePriority `json:"priority"`
	IsMeta   bool          `json:"isMeta,omitempty"`   // System-generated, not user input
	AgentID  string        `json:"agentId,omitempty"`  // For agent-scoped filtering
}

// CommandQueue buffers user input while the agent is processing.
// Priority determines dequeue order: Now > Next > Later.
// Within the same priority, commands are FIFO.
// Source: utils/messageQueueManager.ts:40-547
type CommandQueue struct {
	mu        sync.Mutex
	items     []QueuedCommand
	listeners []func()
}

// NewCommandQueue creates an empty command queue.
func NewCommandQueue() *CommandQueue {
	return &CommandQueue{}
}

// Enqueue adds a command to the queue.
// Source: utils/messageQueueManager.ts:128-135
func (q *CommandQueue) Enqueue(cmd QueuedCommand) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.items = append(q.items, cmd)
	q.notify()
}

// EnqueueWithDefault adds a command, defaulting priority to Next.
// Source: utils/messageQueueManager.ts:129
func (q *CommandQueue) EnqueueWithDefault(cmd QueuedCommand) {
	if cmd.Priority == 0 {
		cmd.Priority = PriorityNext
	}
	q.Enqueue(cmd)
}

// Dequeue removes and returns the highest-priority command (FIFO within priority).
// Returns nil if empty.
// Source: utils/messageQueueManager.ts:167-193
func (q *CommandQueue) Dequeue() *QueuedCommand {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.dequeueInternal(nil)
}

// DequeueFiltered removes and returns the highest-priority command matching the filter.
// Source: utils/messageQueueManager.ts:167-193
func (q *CommandQueue) DequeueFiltered(filter func(QueuedCommand) bool) *QueuedCommand {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.dequeueInternal(filter)
}

func (q *CommandQueue) dequeueInternal(filter func(QueuedCommand) bool) *QueuedCommand {
	if len(q.items) == 0 {
		return nil
	}

	bestIdx := -1
	bestPriority := QueuePriority(999)
	for i, cmd := range q.items {
		if filter != nil && !filter(cmd) {
			continue
		}
		if cmd.Priority < bestPriority {
			bestIdx = i
			bestPriority = cmd.Priority
		}
	}

	if bestIdx == -1 {
		return nil
	}

	cmd := q.items[bestIdx]
	q.items = append(q.items[:bestIdx], q.items[bestIdx+1:]...)
	q.notify()
	return &cmd
}

// Peek returns the highest-priority command without removing it.
// Source: utils/messageQueueManager.ts:219-238
func (q *CommandQueue) Peek() *QueuedCommand {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) == 0 {
		return nil
	}

	bestIdx := 0
	for i, cmd := range q.items {
		if cmd.Priority < q.items[bestIdx].Priority {
			bestIdx = i
		}
	}
	cmd := q.items[bestIdx]
	return &cmd
}

// Len returns the number of commands in the queue.
// Source: utils/messageQueueManager.ts:97-99
func (q *CommandQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}

// HasCommands returns true if the queue has any commands.
// Source: utils/messageQueueManager.ts:104-106
func (q *CommandQueue) HasCommands() bool {
	return q.Len() > 0
}

// Clear removes all commands from the queue.
// Source: utils/messageQueueManager.ts:322-328
func (q *CommandQueue) Clear() {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) == 0 {
		return
	}
	q.items = nil
	q.notify()
}

// DequeueAll removes and returns all commands.
// Source: utils/messageQueueManager.ts:199-213
func (q *CommandQueue) DequeueAll() []QueuedCommand {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) == 0 {
		return nil
	}
	cmds := make([]QueuedCommand, len(q.items))
	copy(cmds, q.items)
	q.items = nil
	q.notify()
	return cmds
}

// Subscribe registers a listener for queue changes.
func (q *CommandQueue) Subscribe(fn func()) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.listeners = append(q.listeners, fn)
}

func (q *CommandQueue) notify() {
	for _, fn := range q.listeners {
		if fn != nil {
			fn()
		}
	}
}
