package tools

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/projectbarks/gopher-code/pkg/compact"
	"github.com/projectbarks/gopher-code/pkg/message"
	"github.com/projectbarks/gopher-code/pkg/permissions"
	"github.com/projectbarks/gopher-code/pkg/provider"
	"github.com/projectbarks/gopher-code/pkg/session"
)

// QueryFunc is the function signature for running a query loop.
// This breaks the import cycle between tools and query: the caller
// injects query.Query at registration time.
type QueryFunc func(
	ctx context.Context,
	sess *session.SessionState,
	prov provider.ModelProvider,
	registry *ToolRegistry,
	orchestrator *ToolOrchestrator,
	onEvent func(text string),
) error

// AgentTool spawns a child query loop to handle a sub-task.
type AgentTool struct {
	provider provider.ModelProvider
	registry *ToolRegistry
	queryFn  QueryFunc
}

// NewAgentTool creates an AgentTool with the dependencies it needs.
// The queryFn parameter breaks the import cycle between tools and query.
func NewAgentTool(prov provider.ModelProvider, reg *ToolRegistry, queryFn QueryFunc) *AgentTool {
	return &AgentTool{provider: prov, registry: reg, queryFn: queryFn}
}

func (t *AgentTool) Name() string        { return "Agent" }
func (t *AgentTool) Description() string {
	return "Launch a sub-agent to handle a complex task. The agent runs autonomously and returns its result."
}
func (t *AgentTool) IsReadOnly() bool { return false }

func (t *AgentTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"prompt": {
				"type": "string",
				"description": "The task for the sub-agent to perform"
			},
			"description": {
				"type": "string",
				"description": "A short (3-5 word) description of what the agent will do"
			}
		},
		"required": ["prompt", "description"],
		"additionalProperties": false
	}`)
}

func (t *AgentTool) Execute(ctx context.Context, tc *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	var params struct {
		Prompt      string `json:"prompt"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return ErrorOutput("invalid input: " + err.Error()), nil
	}
	if params.Prompt == "" {
		return ErrorOutput("prompt is required"), nil
	}

	// Create child session with a fast model and limited turns.
	childCfg := session.SessionConfig{
		Model:          "claude-sonnet-4-20250514",
		SystemPrompt:   "You are a helpful sub-agent. Complete the task and report your findings concisely.",
		MaxTurns:       20,
		TokenBudget:    compact.DefaultBudget(),
		PermissionMode: permissions.AutoApprove,
	}
	childSess := session.New(childCfg, tc.CWD)
	childSess.PushMessage(message.UserMessage(params.Prompt))

	// Create a child orchestrator sharing the registry but not state.
	childOrch := NewOrchestrator(t.registry)

	// Collect text output from the sub-agent.
	var resultText strings.Builder
	onText := func(text string) {
		resultText.WriteString(text)
	}

	// Run the child query loop.
	err := t.queryFn(ctx, childSess, t.provider, t.registry, childOrch, onText)
	if err != nil {
		return ErrorOutput("agent error: " + err.Error()), nil
	}

	result := resultText.String()
	if result == "" {
		result = "(agent completed with no text output)"
	}
	return SuccessOutput(result), nil
}
