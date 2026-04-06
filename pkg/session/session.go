package session

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/projectbarks/gopher-code/pkg/compact"
	"github.com/projectbarks/gopher-code/pkg/message"
	"github.com/projectbarks/gopher-code/pkg/permissions"
	"github.com/projectbarks/gopher-code/pkg/provider"
)

// PermissionPolicyProvider is the interface for permission checking.
// Duplicated here to avoid import cycles with the tools package.
type PermissionPolicyProvider interface {
	Check(ctx context.Context, toolName string, toolID string) permissions.PermissionDecision
}

// SessionConfig holds configuration for a session.
type SessionConfig struct {
	Model           string
	SystemPrompt    string
	MaxTurns        int
	TokenBudget     compact.TokenBudget
	PermissionMode  permissions.PermissionMode
	ThinkingEnabled bool
	ThinkingBudget  int     // default 10000
	JSONSchema        string  `json:"json_schema,omitempty"`
	MaxBudgetUSD      float64 `json:"max_budget_usd,omitempty"`
	TokenBudgetTarget int     `json:"token_budget_target,omitempty"` // +500k feature: output token target
	FallbackModel     string  `json:"fallback_model,omitempty"`      // --fallback-model: switch on 529 exhaustion
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() SessionConfig {
	return SessionConfig{
		Model:          "claude-sonnet-4-20250514",
		MaxTurns:       100,
		TokenBudget:    compact.DefaultBudget(),
		PermissionMode: permissions.AutoApprove,
	}
}

// ModelUsageEntry tracks token usage for a specific model.
// Source: bootstrap/state.ts — modelUsage
type ModelUsageEntry struct {
	InputTokens              int     `json:"input_tokens"`
	OutputTokens             int     `json:"output_tokens"`
	CacheCreationInputTokens int     `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int     `json:"cache_read_input_tokens"`
	CostUSD                  float64 `json:"cost_usd"`
}

// SessionState holds the mutable state of a conversation session.
// Source: bootstrap/state.ts — State type
type SessionState struct {
	ID                       string            `json:"id"`
	ParentSessionID          string            `json:"parent_session_id,omitempty"`
	Name                     string            `json:"name,omitempty"`
	Config                   SessionConfig     `json:"config"`
	Messages                 []message.Message `json:"messages"`
	CWD                      string            `json:"cwd"`
	OriginalCWD              string            `json:"original_cwd"`
	ProjectRoot              string            `json:"project_root"`
	TurnCount                int               `json:"turn_count"`
	TotalInputTokens         int               `json:"total_input_tokens"`
	TotalOutputTokens        int               `json:"total_output_tokens"`
	TotalCacheCreationTokens int               `json:"total_cache_creation_tokens"`
	TotalCacheReadTokens     int               `json:"total_cache_read_tokens"`
	LastInputTokens          int               `json:"last_input_tokens"`
	CreatedAt                time.Time         `json:"created_at"`

	// Cost & duration tracking — Source: bootstrap/state.ts lines 51-64
	TotalCostUSD                  float64 `json:"total_cost_usd"`
	TotalAPIDuration              float64 `json:"total_api_duration_ms"`                // cumulative API call time in ms
	TotalAPIDurationWithoutRetries float64 `json:"total_api_duration_without_retries_ms"` // T108: API duration excluding retries
	TotalToolDuration             float64 `json:"total_tool_duration_ms"`                // cumulative tool execution time in ms
	TotalLinesAdded               int     `json:"total_lines_added"`
	TotalLinesRemoved             int     `json:"total_lines_removed"`

	// Per-turn duration tracking — Source: bootstrap/state.ts lines 55-57 (T109)
	TurnHookDurationMs       float64 `json:"turn_hook_duration_ms"`
	TurnToolDurationMs       float64 `json:"turn_tool_duration_ms"`
	TurnClassifierDurationMs float64 `json:"turn_classifier_duration_ms"`

	// Per-turn count tracking — Source: bootstrap/state.ts lines 58-60 (T110)
	TurnToolCount       int `json:"turn_tool_count"`
	TurnHookCount       int `json:"turn_hook_count"`
	TurnClassifierCount int `json:"turn_classifier_count"`

	// T111: Monotonic start time, set once at session creation.
	// Source: bootstrap/state.ts line 61 — startTime
	StartTime time.Time `json:"start_time"`

	// T112: Last interaction time tracking. The dirty flag defers Date.now()
	// calls until a render frame flush, avoiding per-keypress overhead.
	// Source: bootstrap/state.ts lines 62, 665-689
	LastInteractionTime time.Time `json:"last_interaction_time"`
	interactionDirty    bool      `json:"-"`

	// T113: Flag set when the model's pricing isn't in the cost table.
	// Source: bootstrap/state.ts line 65
	HasUnknownModelCost bool `json:"has_unknown_model_cost,omitempty"`

	// T114: Model override for the session main loop.
	// Source: bootstrap/state.ts lines 68-69
	MainLoopModelOverride string `json:"main_loop_model_override,omitempty"`
	InitialMainLoopModel  string `json:"initial_main_loop_model,omitempty"`

	// T115: Cache of resolved model display strings.
	// Source: bootstrap/state.ts line 70
	ModelStrings map[string]string `json:"model_strings,omitempty"`

	// Per-model usage tracking — Source: bootstrap/state.ts line 67
	mu         sync.Mutex               `json:"-"`
	ModelUsage map[string]*ModelUsageEntry `json:"model_usage,omitempty"`

	// CoordinatorMode stores the session's coordinator mode for resume reconciliation.
	// Values: "coordinator", "normal", or "" (unset/legacy session).
	// Source: coordinatorMode.ts — sessionMode field
	CoordinatorMode string `json:"coordinator_mode,omitempty"`

	// IsInteractive distinguishes TUI/REPL mode from headless/pipe mode.
	// Source: bootstrap/state.ts line 71
	IsInteractive bool `json:"is_interactive"`

	// PermissionPolicy is the runtime permission policy (not serialized).
	PermissionPolicy interface{} `json:"-"`

	// StopHookRunner is an optional callback that runs after model turns.
	// It can prevent continuation or inject blocking errors.
	// Source: query/stopHooks.ts:60-63
	StopHookRunner interface{} `json:"-"`

	// PostSamplingHooks are fire-and-forget callbacks that run after model
	// sampling completes, before tool execution.
	// Source: utils/hooks/postSamplingHooks.ts:24-33
	PostSamplingHooks []interface{} `json:"-"`
}

// New creates a new SessionState with the given config and working directory.
// Source: bootstrap/state.ts — getInitialState()
func New(config SessionConfig, cwd string) *SessionState {
	now := time.Now()
	return &SessionState{
		ID:                  uuid.New().String(),
		Config:              config,
		Messages:            make([]message.Message, 0),
		CWD:                 cwd,
		OriginalCWD:         cwd,
		ProjectRoot:         cwd,
		CreatedAt:           now,
		StartTime:           now,
		LastInteractionTime: now,
		ModelUsage:          make(map[string]*ModelUsageEntry),
	}
}

// AddCost records a cost amount and per-model usage.
// Source: bootstrap/state.ts — addToTotalCostState()
func (s *SessionState) AddCost(model string, costUSD float64, usage provider.TokenUsage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TotalCostUSD += costUSD
	entry, ok := s.ModelUsage[model]
	if !ok {
		entry = &ModelUsageEntry{}
		s.ModelUsage[model] = entry
	}
	entry.InputTokens += usage.InputTokens
	entry.OutputTokens += usage.OutputTokens
	entry.CacheCreationInputTokens += usage.CacheCreationInputTokens
	entry.CacheReadInputTokens += usage.CacheReadInputTokens
	entry.CostUSD += costUSD
}

// AddAPIDurationWithoutRetries accumulates API call time excluding retries.
// Source: bootstrap/state.ts — addToTotalAPIDurationWithoutRetries()
func (s *SessionState) AddAPIDurationWithoutRetries(durationMs float64) {
	s.TotalAPIDurationWithoutRetries += durationMs
}

// ResetAPIDurationWithoutRetries zeros the running total.
// Source: bootstrap/state.ts — resetTotalAPIDurationWithoutRetries()
func (s *SessionState) ResetAPIDurationWithoutRetries() {
	s.TotalAPIDurationWithoutRetries = 0
}

// AddTurnToolDuration records tool execution time and increments the tool count.
// Source: bootstrap/state.ts — addToTurnToolDuration()
func (s *SessionState) AddTurnToolDuration(durationMs float64) {
	s.TurnToolDurationMs += durationMs
	s.TurnToolCount++
}

// ResetTurnToolMetrics zeros the per-turn tool duration and count.
// Source: bootstrap/state.ts — resetTurnToolDuration()
func (s *SessionState) ResetTurnToolMetrics() {
	s.TurnToolDurationMs = 0
	s.TurnToolCount = 0
}

// AddTurnHookDuration records hook execution time and increments the hook count.
// Source: bootstrap/state.ts — addToTurnHookDuration()
func (s *SessionState) AddTurnHookDuration(durationMs float64) {
	s.TurnHookDurationMs += durationMs
	s.TurnHookCount++
}

// ResetTurnHookMetrics zeros the per-turn hook duration and count.
// Source: bootstrap/state.ts — resetTurnHookDuration()
func (s *SessionState) ResetTurnHookMetrics() {
	s.TurnHookDurationMs = 0
	s.TurnHookCount = 0
}

// AddTurnClassifierDuration records classifier execution time and increments the count.
// Source: bootstrap/state.ts — addToTurnClassifierDuration()
func (s *SessionState) AddTurnClassifierDuration(durationMs float64) {
	s.TurnClassifierDurationMs += durationMs
	s.TurnClassifierCount++
}

// ResetTurnClassifierMetrics zeros the per-turn classifier duration and count.
// Source: bootstrap/state.ts — resetTurnClassifierDuration()
func (s *SessionState) ResetTurnClassifierMetrics() {
	s.TurnClassifierDurationMs = 0
	s.TurnClassifierCount = 0
}

// AddLinesChanged records lines added/removed by code edits.
// Source: bootstrap/state.ts — addToTotalLinesChanged()
func (s *SessionState) AddLinesChanged(added, removed int) {
	s.TotalLinesAdded += added
	s.TotalLinesRemoved += removed
}

// UpdateLastInteractionTime marks an interaction. When immediate is true the
// timestamp is updated right away; otherwise it is deferred until the next
// FlushInteractionTime call (batching many keypresses into one clock read).
// Source: bootstrap/state.ts — updateLastInteractionTime()
func (s *SessionState) UpdateLastInteractionTime(immediate bool) {
	if immediate {
		s.LastInteractionTime = time.Now()
		s.interactionDirty = false
	} else {
		s.interactionDirty = true
	}
}

// FlushInteractionTime updates the timestamp if an interaction was recorded
// since the last flush. Called by the render loop before each frame.
// Source: bootstrap/state.ts — flushInteractionTime()
func (s *SessionState) FlushInteractionTime() {
	if s.interactionDirty {
		s.LastInteractionTime = time.Now()
		s.interactionDirty = false
	}
}

// SetModelStrings replaces the cached model display strings.
// Source: bootstrap/state.ts — setModelStrings()
func (s *SessionState) SetModelStrings(m map[string]string) {
	s.ModelStrings = m
}

// ClearModelStrings resets the model strings cache to nil.
// Source: bootstrap/state.ts — clearModelStrings()
func (s *SessionState) ClearModelStrings() {
	s.ModelStrings = nil
}

// RegenerateSessionID creates a new session ID, optionally setting the current as parent.
// Source: bootstrap/state.ts — regenerateSessionId()
func (s *SessionState) RegenerateSessionID(setCurrentAsParent bool) string {
	if setCurrentAsParent {
		s.ParentSessionID = s.ID
	}
	s.ID = uuid.New().String()
	return s.ID
}

// FirstUserPreview extracts the first user message text, truncated to
// PreviewMaxLen characters. Used for the resume session picker.
// Source: ResumeConversation.tsx — first user message preview
func (s *SessionState) FirstUserPreview() string {
	for _, m := range s.Messages {
		if m.Role != message.RoleUser {
			continue
		}
		for _, b := range m.Content {
			if b.Type == message.ContentText && b.Text != "" {
				text := b.Text
				if len(text) > PreviewMaxLen {
					text = text[:PreviewMaxLen-3] + "..."
				}
				return text
			}
		}
	}
	return ""
}

// PushMessage appends a message to the session history.
func (s *SessionState) PushMessage(msg message.Message) {
	s.Messages = append(s.Messages, msg)
}

// ToRequestMessages converts session messages to the API request format.
func (s *SessionState) ToRequestMessages() []provider.RequestMessage {
	msgs := make([]provider.RequestMessage, 0, len(s.Messages))
	for _, m := range s.Messages {
		rm := provider.RequestMessage{
			Role:    string(m.Role),
			Content: make([]provider.RequestContent, 0, len(m.Content)),
		}
		for _, b := range m.Content {
			switch b.Type {
			case message.ContentText:
				rm.Content = append(rm.Content, provider.RequestContent{
					Type: "text",
					Text: b.Text,
				})
			case message.ContentToolUse:
				rm.Content = append(rm.Content, provider.RequestContent{
					Type:  "tool_use",
					ID:    b.ID,
					Name:  b.Name,
					Input: b.Input,
				})
			case message.ContentToolResult:
				var errPtr *bool
				if b.IsError {
					t := true
					errPtr = &t
				}
				rm.Content = append(rm.Content, provider.RequestContent{
					Type:      "tool_result",
					ToolUseID: b.ToolUseID,
					Content:   b.Content,
					IsError:   errPtr,
				})
			}
		}
		msgs = append(msgs, rm)
	}
	return msgs
}
