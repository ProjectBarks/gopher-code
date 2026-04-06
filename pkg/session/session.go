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
	TotalCostUSD     float64 `json:"total_cost_usd"`
	TotalAPIDuration float64 `json:"total_api_duration_ms"` // cumulative API call time in ms
	TotalToolDuration float64 `json:"total_tool_duration_ms"` // cumulative tool execution time in ms
	TotalLinesAdded  int     `json:"total_lines_added"`
	TotalLinesRemoved int    `json:"total_lines_removed"`

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
	return &SessionState{
		ID:          uuid.New().String(),
		Config:      config,
		Messages:    make([]message.Message, 0),
		CWD:         cwd,
		OriginalCWD: cwd,
		ProjectRoot: cwd,
		CreatedAt:   time.Now(),
		ModelUsage:  make(map[string]*ModelUsageEntry),
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

// AddLinesChanged records lines added/removed by code edits.
// Source: bootstrap/state.ts — addToTotalLinesChanged()
func (s *SessionState) AddLinesChanged(added, removed int) {
	s.TotalLinesAdded += added
	s.TotalLinesRemoved += removed
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
