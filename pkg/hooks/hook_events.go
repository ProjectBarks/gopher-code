package hooks

import (
	"sync"
)

// Source: utils/hooks/hookEvents.ts

// AlwaysEmittedHookEvents are emitted regardless of the includeHookEvents option.
// Source: hookEvents.ts:18
var AlwaysEmittedHookEvents = []HookEvent{SessionStart, Setup}

// MaxPendingEvents is the max queued events before handler registration.
// Source: hookEvents.ts:20
const MaxPendingEvents = 100

// HookStartedEvent is emitted when a hook begins execution.
// Source: hookEvents.ts:22-27
type HookStartedEvent struct {
	Type      string `json:"type"` // "started"
	HookID    string `json:"hookId"`
	HookName  string `json:"hookName"`
	HookEvent string `json:"hookEvent"`
}

// HookProgressEvent is emitted periodically during hook execution.
// Source: hookEvents.ts:29-37
type HookProgressEvent struct {
	Type      string `json:"type"` // "progress"
	HookID    string `json:"hookId"`
	HookName  string `json:"hookName"`
	HookEvent string `json:"hookEvent"`
	Stdout    string `json:"stdout"`
	Stderr    string `json:"stderr"`
	Output    string `json:"output"`
}

// HookResponseEvent is emitted when a hook completes.
// Source: hookEvents.ts:39-49
type HookResponseEvent struct {
	Type      string `json:"type"` // "response"
	HookID    string `json:"hookId"`
	HookName  string `json:"hookName"`
	HookEvent string `json:"hookEvent"`
	Output    string `json:"output"`
	Stdout    string `json:"stdout"`
	Stderr    string `json:"stderr"`
	ExitCode  *int   `json:"exitCode,omitempty"`
	Outcome   string `json:"outcome"` // "success", "error", "cancelled"
}

// HookExecutionEvent is a union of started/progress/response events.
// Source: hookEvents.ts:51-52
type HookExecutionEvent struct {
	// Exactly one of these is non-nil.
	Started  *HookStartedEvent
	Progress *HookProgressEvent
	Response *HookResponseEvent
}

// HookEventHandler processes hook execution events.
// Source: hookEvents.ts:55
type HookEventHandler func(event HookExecutionEvent)

// HookEventEmitter manages hook event broadcasting with pre-registration queuing.
// Source: hookEvents.ts (module-level state)
type HookEventEmitter struct {
	mu                   sync.Mutex
	handler              HookEventHandler
	pendingEvents        []HookExecutionEvent
	allHookEventsEnabled bool
}

// NewHookEventEmitter creates a new emitter.
func NewHookEventEmitter() *HookEventEmitter {
	return &HookEventEmitter{}
}

// RegisterHandler sets the event handler and flushes any pending events.
// Pass nil to unregister.
// Source: hookEvents.ts:61-70
func (e *HookEventEmitter) RegisterHandler(handler HookEventHandler) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.handler = handler
	if handler != nil && len(e.pendingEvents) > 0 {
		pending := e.pendingEvents
		e.pendingEvents = nil
		for _, ev := range pending {
			handler(ev)
		}
	}
}

// emit sends an event to the handler or queues it.
// Source: hookEvents.ts:72-80
func (e *HookEventEmitter) emit(event HookExecutionEvent) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.handler != nil {
		e.handler(event)
		return
	}

	e.pendingEvents = append(e.pendingEvents, event)
	if len(e.pendingEvents) > MaxPendingEvents {
		e.pendingEvents = e.pendingEvents[1:]
	}
}

// shouldEmit checks whether a hook event should be emitted.
// Source: hookEvents.ts:83-91
func (e *HookEventEmitter) shouldEmit(hookEvent string) bool {
	for _, ev := range AlwaysEmittedHookEvents {
		if string(ev) == hookEvent {
			return true
		}
	}
	e.mu.Lock()
	enabled := e.allHookEventsEnabled
	e.mu.Unlock()

	if !enabled {
		return false
	}
	return IsHookEvent(hookEvent)
}

// EmitHookStarted emits a started event.
// Source: hookEvents.ts:93-106
func (e *HookEventEmitter) EmitHookStarted(hookID, hookName, hookEvent string) {
	if !e.shouldEmit(hookEvent) {
		return
	}
	e.emit(HookExecutionEvent{
		Started: &HookStartedEvent{
			Type:      "started",
			HookID:    hookID,
			HookName:  hookName,
			HookEvent: hookEvent,
		},
	})
}

// EmitHookProgress emits a progress event.
// Source: hookEvents.ts:108-122
func (e *HookEventEmitter) EmitHookProgress(hookID, hookName, hookEvent, stdout, stderr, output string) {
	if !e.shouldEmit(hookEvent) {
		return
	}
	e.emit(HookExecutionEvent{
		Progress: &HookProgressEvent{
			Type:      "progress",
			HookID:    hookID,
			HookName:  hookName,
			HookEvent: hookEvent,
			Stdout:    stdout,
			Stderr:    stderr,
			Output:    output,
		},
	})
}

// EmitHookResponse emits a response event.
// Source: hookEvents.ts:153-177
func (e *HookEventEmitter) EmitHookResponse(hookID, hookName, hookEvent, output, stdout, stderr string, exitCode *int, outcome string) {
	if !e.shouldEmit(hookEvent) {
		return
	}
	e.emit(HookExecutionEvent{
		Response: &HookResponseEvent{
			Type:      "response",
			HookID:    hookID,
			HookName:  hookName,
			HookEvent: hookEvent,
			Output:    output,
			Stdout:    stdout,
			Stderr:    stderr,
			ExitCode:  exitCode,
			Outcome:   outcome,
		},
	})
}

// SetAllHookEventsEnabled enables or disables emission of all hook event types.
// Source: hookEvents.ts:184-186
func (e *HookEventEmitter) SetAllHookEventsEnabled(enabled bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.allHookEventsEnabled = enabled
}

// Clear resets all emitter state.
// Source: hookEvents.ts:188-192
func (e *HookEventEmitter) Clear() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.handler = nil
	e.pendingEvents = nil
	e.allHookEventsEnabled = false
}

// PendingCount returns the number of queued events (for testing).
func (e *HookEventEmitter) PendingCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.pendingEvents)
}
