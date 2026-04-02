package session

import (
	"context"
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

// SessionState holds the mutable state of a conversation session.
type SessionState struct {
	ID                       string            `json:"id"`
	Name                     string            `json:"name,omitempty"`
	Config                   SessionConfig     `json:"config"`
	Messages                 []message.Message `json:"messages"`
	CWD                      string            `json:"cwd"`
	TurnCount                int               `json:"turn_count"`
	TotalInputTokens         int               `json:"total_input_tokens"`
	TotalOutputTokens        int               `json:"total_output_tokens"`
	TotalCacheCreationTokens int               `json:"total_cache_creation_tokens"`
	TotalCacheReadTokens     int               `json:"total_cache_read_tokens"`
	LastInputTokens          int               `json:"last_input_tokens"`
	CreatedAt                time.Time         `json:"created_at"`

	// PermissionPolicy is the runtime permission policy (not serialized).
	PermissionPolicy interface{} `json:"-"`
}

// New creates a new SessionState with the given config and working directory.
func New(config SessionConfig, cwd string) *SessionState {
	return &SessionState{
		ID:        uuid.New().String(),
		Config:    config,
		Messages:  make([]message.Message, 0),
		CWD:       cwd,
		CreatedAt: time.Now(),
	}
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
