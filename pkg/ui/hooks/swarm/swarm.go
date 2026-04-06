// Package swarm provides bubbletea hooks for swarm mode: initialization,
// task watching, and permission polling. Each hook maps a React hook from
// src/hooks/useSwarmInitialization.ts, useTaskListWatcher.ts, and
// useSwarmPermissionPoller.ts into a bubbletea Cmd/Msg pattern.
//
// Source: src/hooks/useSwarmInitialization.ts, useTaskListWatcher.ts,
//
//	useSwarmPermissionPoller.ts
package swarm

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
)

// ---------------------------------------------------------------------------
// SwarmInit — one-shot startup Cmd
// Source: src/hooks/useSwarmInitialization.ts
// ---------------------------------------------------------------------------

// SwarmInitMsg is dispatched after swarm initialization completes.
type SwarmInitMsg struct {
	// TeamsDir is the path to the created teams directory.
	TeamsDir string
	// Teammates is the list of teammate IDs spawned during init.
	Teammates []string
	// Colors maps teammate IDs to their assigned color names.
	Colors map[string]string
	// Err is non-nil if initialization failed.
	Err error
}

// SwarmInit holds the configuration for swarm-mode bootstrap.
// In bubbletea this becomes a one-shot tea.Cmd dispatched at Init() time.
//
// Source: useSwarmInitialization — detects resumed vs fresh spawn,
// sets up teams dir, spawns initial teammates, assigns colors.
type SwarmInit struct {
	// Enabled gates the entire initialization (maps to the TS `enabled` prop).
	Enabled bool
	// TeamsDir is the base directory for team files. If empty, defaults to
	// $HOME/.claude/teams.
	TeamsDir string
	// Teammates is the list of teammate names to spawn at startup.
	Teammates []string
	// ColorPalette is the ordered color palette for round-robin assignment.
	// If nil, uses the default palette.
	ColorPalette []string
}

// defaultColorPalette matches AgentColors from session/teammate.go.
var defaultColorPalette = []string{
	"red", "blue", "green", "yellow",
	"purple", "orange", "pink", "cyan",
}

// Init returns a tea.Cmd that performs swarm initialization.
// It creates the teams directory, assigns colors to teammates, and returns
// a SwarmInitMsg. Safe to call even when Enabled is false (returns nil).
func (s *SwarmInit) Init() tea.Cmd {
	if !s.Enabled {
		return nil
	}

	teamsDir := s.TeamsDir
	if teamsDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return func() tea.Msg {
				return SwarmInitMsg{Err: fmt.Errorf("swarm init: %w", err)}
			}
		}
		teamsDir = filepath.Join(home, ".claude", "teams")
	}

	teammates := s.Teammates
	palette := s.ColorPalette
	if palette == nil {
		palette = defaultColorPalette
	}

	return func() tea.Msg {
		// Create teams directory (idempotent).
		if err := os.MkdirAll(teamsDir, 0o755); err != nil {
			return SwarmInitMsg{Err: fmt.Errorf("swarm init: create teams dir: %w", err)}
		}

		// Assign colors round-robin.
		colors := make(map[string]string, len(teammates))
		for i, t := range teammates {
			colors[t] = palette[i%len(palette)]
		}

		return SwarmInitMsg{
			TeamsDir:  teamsDir,
			Teammates: teammates,
			Colors:    colors,
		}
	}
}

// ---------------------------------------------------------------------------
// TaskWatcher — periodic tick that watches for task completion
// Source: src/hooks/useTaskListWatcher.ts
// ---------------------------------------------------------------------------

// TaskStatus represents the lifecycle state of a task.
type TaskStatus string

const (
	TaskPending   TaskStatus = "pending"
	TaskClaimed   TaskStatus = "claimed"
	TaskCompleted TaskStatus = "completed"
)

// Task represents a single item in the task list.
// Source: src/utils/tasks.ts Task type.
type Task struct {
	ID          string     `json:"id"`
	Subject     string     `json:"subject"`
	Description string     `json:"description,omitempty"`
	Status      TaskStatus `json:"status"`
	Owner       string     `json:"owner,omitempty"`
	BlockedBy   []string   `json:"blockedBy,omitempty"`
}

// TaskWatchMsg is dispatched each tick with the current task list state.
type TaskWatchMsg struct {
	// Tasks is the current snapshot of all tasks.
	Tasks []Task
	// Claimed is the task that was just claimed (nil if none).
	Claimed *Task
	// Completed is the list of task IDs that transitioned to completed
	// since the last tick.
	Completed []string
	// Err is non-nil if the tick failed.
	Err error
}

const (
	// DefaultTaskPollInterval is the default interval for task polling.
	// The TS source uses a 1s debounce on fs.watch events; in Go we
	// use a periodic tick at the same cadence.
	// Source: useTaskListWatcher.ts DEBOUNCE_MS = 1000
	DefaultTaskPollInterval = 1 * time.Second
)

// TaskListFunc is a callback that returns the current task list.
// This abstraction allows the watcher to be tested without filesystem access.
type TaskListFunc func() ([]Task, error)

// ClaimFunc attempts to claim a task. Returns true if the claim succeeded.
type ClaimFunc func(taskID, agentID string) (bool, error)

// SubmitFunc submits a claimed task as a prompt. Returns true if submission
// succeeded.
type SubmitFunc func(prompt string) bool

// TaskWatcher watches a task list for completion and claims available tasks.
// In bubbletea this maps to a periodic tea.Tick.
//
// Source: useTaskListWatcher.ts — watches tasks dir, auto-claims next
// available task when current completes, submits as prompt.
type TaskWatcher struct {
	// AgentID identifies this watcher for task claiming.
	AgentID string
	// Interval between ticks. Defaults to DefaultTaskPollInterval.
	Interval time.Duration

	// ListTasks returns the current task list snapshot.
	ListTasks TaskListFunc
	// ClaimTask attempts to claim a task for this agent.
	ClaimTask ClaimFunc
	// SubmitTask submits a task prompt. Returns true on success.
	SubmitTask SubmitFunc

	// mu guards currentTaskID.
	mu            sync.Mutex
	currentTaskID string
	// lastSeen tracks task statuses from the previous tick for diff detection.
	lastSeen map[string]TaskStatus
}

// Tick returns a tea.Cmd that fires after the configured interval,
// checks for task state changes, and claims available work.
func (w *TaskWatcher) Tick() tea.Cmd {
	interval := w.Interval
	if interval == 0 {
		interval = DefaultTaskPollInterval
	}

	return tea.Tick(interval, func(t time.Time) tea.Msg {
		return w.check()
	})
}

// check performs one poll cycle. Exported via Tick's closure.
func (w *TaskWatcher) check() TaskWatchMsg {
	if w.ListTasks == nil {
		return TaskWatchMsg{Err: fmt.Errorf("task watcher: ListTasks is nil")}
	}

	tasks, err := w.ListTasks()
	if err != nil {
		return TaskWatchMsg{Err: fmt.Errorf("task watcher: %w", err)}
	}

	// Detect completions by diffing against lastSeen.
	var completed []string
	if w.lastSeen != nil {
		for _, t := range tasks {
			prev, ok := w.lastSeen[t.ID]
			if ok && prev != TaskCompleted && t.Status == TaskCompleted {
				completed = append(completed, t.ID)
			}
		}
	}

	// Update lastSeen.
	seen := make(map[string]TaskStatus, len(tasks))
	for _, t := range tasks {
		seen[t.ID] = t.Status
	}
	w.lastSeen = seen

	// If current task completed, clear it.
	w.mu.Lock()
	curID := w.currentTaskID
	w.mu.Unlock()

	if curID != "" {
		for _, id := range completed {
			if id == curID {
				w.mu.Lock()
				w.currentTaskID = ""
				w.mu.Unlock()
				curID = ""
				break
			}
		}
	}

	// If no current task, try to claim an available one.
	var claimed *Task
	if curID == "" {
		available := findAvailableTask(tasks)
		if available != nil && w.ClaimTask != nil {
			ok, claimErr := w.ClaimTask(available.ID, w.AgentID)
			if claimErr == nil && ok {
				w.mu.Lock()
				w.currentTaskID = available.ID
				w.mu.Unlock()
				claimed = available

				// Submit the task as a prompt.
				prompt := formatTaskPrompt(available)
				if w.SubmitTask != nil {
					if !w.SubmitTask(prompt) {
						// Submission failed — release claim.
						w.mu.Lock()
						w.currentTaskID = ""
						w.mu.Unlock()
						claimed = nil
					}
				}
			}
		}
	}

	return TaskWatchMsg{
		Tasks:     tasks,
		Claimed:   claimed,
		Completed: completed,
	}
}

// findAvailableTask returns the first pending, unowned, unblocked task.
// Source: useTaskListWatcher.ts findAvailableTask().
func findAvailableTask(tasks []Task) *Task {
	unresolved := make(map[string]struct{})
	for _, t := range tasks {
		if t.Status != TaskCompleted {
			unresolved[t.ID] = struct{}{}
		}
	}

	for i := range tasks {
		t := &tasks[i]
		if t.Status != TaskPending {
			continue
		}
		if t.Owner != "" {
			continue
		}
		// All blockers must be resolved.
		blocked := false
		for _, bid := range t.BlockedBy {
			if _, ok := unresolved[bid]; ok {
				blocked = true
				break
			}
		}
		if blocked {
			continue
		}
		return t
	}
	return nil
}

// formatTaskPrompt formats a task as a prompt for the agent.
// Source: useTaskListWatcher.ts formatTaskAsPrompt().
func formatTaskPrompt(t *Task) string {
	prompt := fmt.Sprintf("Complete all open tasks. Start with task #%s: \n\n %s", t.ID, t.Subject)
	if t.Description != "" {
		prompt += "\n\n" + t.Description
	}
	return prompt
}

// ---------------------------------------------------------------------------
// PermissionPoller — periodic tick that polls for permission responses
// Source: src/hooks/useSwarmPermissionPoller.ts
// ---------------------------------------------------------------------------

const (
	// DefaultPermissionPollInterval matches the TS POLL_INTERVAL_MS = 500.
	DefaultPermissionPollInterval = 500 * time.Millisecond
)

// PermissionDecision is the leader's response to a permission request.
type PermissionDecision string

const (
	PermissionApproved PermissionDecision = "approved"
	PermissionRejected PermissionDecision = "rejected"
)

// PermissionResponse is a single response from the leader.
type PermissionResponse struct {
	RequestID  string             `json:"requestId"`
	Decision   PermissionDecision `json:"decision"`
	Feedback   string             `json:"feedback,omitempty"`
	UpdatedInput map[string]any   `json:"updatedInput,omitempty"`
}

// PermissionPollMsg is dispatched each tick with any received responses.
type PermissionPollMsg struct {
	// Responses contains permission responses received this tick.
	Responses []PermissionResponse
	// Err is non-nil if the poll failed.
	Err error
}

// PollFunc checks for permission responses for a given request ID.
// Returns nil if no response is available yet.
type PollFunc func(requestID, agentName, teamName string) (*PermissionResponse, error)

// RemoveResponseFunc cleans up a processed response.
type RemoveResponseFunc func(requestID, agentName, teamName string) error

// PermissionCallback is invoked when a permission response arrives.
type PermissionCallback struct {
	RequestID string
	ToolUseID string
	OnAllow   func(updatedInput map[string]any, feedback string)
	OnReject  func(feedback string)
}

// PermissionPoller polls for pending permission request responses from the
// swarm leader. In bubbletea this maps to a periodic tea.Tick.
//
// Source: useSwarmPermissionPoller.ts — polls every 500ms for responses,
// invokes registered callbacks, cleans up processed responses.
type PermissionPoller struct {
	// AgentName is this worker's name.
	AgentName string
	// TeamName is the team this worker belongs to.
	TeamName string
	// Interval between polls. Defaults to DefaultPermissionPollInterval.
	Interval time.Duration

	// Poll checks for a response for a given request ID.
	Poll PollFunc
	// RemoveResponse cleans up a processed response file.
	RemoveResponse RemoveResponseFunc

	mu        sync.Mutex
	callbacks map[string]*PermissionCallback
	polling   bool
}

// Register adds a callback for a pending permission request.
// Source: registerPermissionCallback().
func (p *PermissionPoller) Register(cb *PermissionCallback) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.callbacks == nil {
		p.callbacks = make(map[string]*PermissionCallback)
	}
	p.callbacks[cb.RequestID] = cb
}

// Unregister removes a callback. Source: unregisterPermissionCallback().
func (p *PermissionPoller) Unregister(requestID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.callbacks, requestID)
}

// Has returns true if a callback is registered for the given request.
// Source: hasPermissionCallback().
func (p *PermissionPoller) Has(requestID string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	_, ok := p.callbacks[requestID]
	return ok
}

// Clear removes all registered callbacks. Source: clearAllPendingCallbacks().
func (p *PermissionPoller) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.callbacks = make(map[string]*PermissionCallback)
}

// PendingCount returns the number of registered callbacks.
func (p *PermissionPoller) PendingCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.callbacks)
}

// Tick returns a tea.Cmd that fires after the configured interval
// and polls for permission responses.
func (p *PermissionPoller) Tick() tea.Cmd {
	interval := p.Interval
	if interval == 0 {
		interval = DefaultPermissionPollInterval
	}

	return tea.Tick(interval, func(t time.Time) tea.Msg {
		return p.poll()
	})
}

// poll performs one poll cycle across all registered callbacks.
func (p *PermissionPoller) poll() PermissionPollMsg {
	if p.AgentName == "" || p.TeamName == "" {
		return PermissionPollMsg{}
	}

	p.mu.Lock()
	if p.polling || len(p.callbacks) == 0 {
		p.mu.Unlock()
		return PermissionPollMsg{}
	}
	p.polling = true

	// Snapshot request IDs to iterate outside the lock.
	ids := make([]string, 0, len(p.callbacks))
	for id := range p.callbacks {
		ids = append(ids, id)
	}
	p.mu.Unlock()

	defer func() {
		p.mu.Lock()
		p.polling = false
		p.mu.Unlock()
	}()

	if p.Poll == nil {
		return PermissionPollMsg{Err: fmt.Errorf("permission poller: Poll func is nil")}
	}

	var responses []PermissionResponse
	for _, reqID := range ids {
		resp, err := p.Poll(reqID, p.AgentName, p.TeamName)
		if err != nil {
			return PermissionPollMsg{Err: fmt.Errorf("permission poller: %w", err)}
		}
		if resp == nil {
			continue
		}

		responses = append(responses, *resp)

		// Invoke the registered callback.
		p.mu.Lock()
		cb, ok := p.callbacks[reqID]
		if ok {
			delete(p.callbacks, reqID)
		}
		p.mu.Unlock()

		if ok && cb != nil {
			switch resp.Decision {
			case PermissionApproved:
				if cb.OnAllow != nil {
					cb.OnAllow(resp.UpdatedInput, resp.Feedback)
				}
			case PermissionRejected:
				if cb.OnReject != nil {
					cb.OnReject(resp.Feedback)
				}
			}
		}

		// Clean up the response file.
		if p.RemoveResponse != nil {
			_ = p.RemoveResponse(reqID, p.AgentName, p.TeamName)
		}
	}

	return PermissionPollMsg{Responses: responses}
}
