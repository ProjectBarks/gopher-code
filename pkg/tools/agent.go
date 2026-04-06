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

func (t *AgentTool) Name() string { return AgentToolName }
func (t *AgentTool) Description() string {
	return "Launch a new agent"
}
func (t *AgentTool) IsReadOnly() bool { return false }

// Aliases returns the legacy tool name for backward compatibility.
// Source: AgentTool/AgentTool.tsx:228
func (t *AgentTool) Aliases() []string { return []string{LegacyAgentToolName} }

// SearchHint returns the search hint for tool discovery.
// Source: AgentTool/AgentTool.tsx:227
func (t *AgentTool) SearchHint() string { return "delegate work to a subagent" }

// MaxResultSizeChars returns the max result size for agent output.
// Source: AgentTool/AgentTool.tsx:229
func (t *AgentTool) MaxResultSizeChars() int { return 100_000 }

// Prompt returns the system-prompt section for the Agent tool.
// Source: AgentTool/prompt.ts:66-286
func (t *AgentTool) Prompt() string {
	// TODO(T502): build agentListSection from loaded agent definitions
	agentListSection := "Available agent types are listed in <system-reminder> messages in the conversation."
	return AgentToolPrompt(agentListSection)
}

// Source: AgentTool/AgentTool.tsx:82-102 inputSchema
func (t *AgentTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"description": {
				"type": "string",
				"description": "A short (3-5 word) description of the task"
			},
			"prompt": {
				"type": "string",
				"description": "The task for the agent to perform"
			},
			"subagent_type": {
				"type": "string",
				"description": "The type of specialized agent to use for this task"
			},
			"model": {
				"type": "string",
				"description": "Optional model override for this agent. Takes precedence over the agent definition's model frontmatter. If omitted, uses the agent definition's model, or inherits from the parent.",
				"enum": ["sonnet", "opus", "haiku"]
			},
			"run_in_background": {
				"type": "boolean",
				"description": "Set to true to run this agent in the background. You will be notified when it completes."
			},
			"name": {
				"type": "string",
				"description": "Name for the spawned agent. Makes it addressable via SendMessage({to: name}) while running."
			},
			"team_name": {
				"type": "string",
				"description": "Team name for spawning. Uses current team context if omitted."
			},
			"mode": {
				"type": "string",
				"description": "Permission mode for spawned teammate (e.g., \"plan\" to require plan approval).",
				"enum": ["acceptEdits", "auto", "bypassPermissions", "default", "dontAsk", "plan"]
			},
			"isolation": {
				"type": "string",
				"description": "Isolation mode. \"worktree\" creates a temporary git worktree so the agent works on an isolated copy of the repo.",
				"enum": ["worktree"]
			},
			"cwd": {
				"type": "string",
				"description": "Absolute path to run the agent in. Overrides the working directory for all filesystem and shell operations within this agent. Mutually exclusive with isolation: \"worktree\"."
			}
		},
		"required": ["description", "prompt"],
		"additionalProperties": false
	}`)
}

// AgentMaxTurns is the default max turns for subagents.
// Source: AgentTool/runAgent.ts — agents get fewer turns than main loop
const AgentMaxTurns = 30

// agentModelAliases maps short model names to full IDs.
// Source: AgentTool prompt.ts — model enum options
var agentModelAliases = map[string]string{
	"haiku":  "claude-haiku-4-5-20251001",
	"sonnet": "claude-sonnet-4-6",
	"opus":   "claude-opus-4-6",
}

func (t *AgentTool) Execute(ctx context.Context, tc *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	var params AgentToolInput
	if err := json.Unmarshal(input, &params); err != nil {
		return ErrorOutput("invalid input: " + err.Error()), nil
	}
	if err := ValidateAgentToolInput(&params); err != nil {
		return ErrorOutput(err.Error()), nil
	}

	// Resolve model: explicit override > parent session model > default
	// Source: utils/model/agent.ts — getAgentModel()
	model := "claude-sonnet-4-6"
	if params.Model != "" {
		if resolved, ok := agentModelAliases[params.Model]; ok {
			model = resolved
		} else {
			model = params.Model
		}
	}

	// Create child session with agent-appropriate config.
	// Source: AgentTool/runAgent.ts:135-160
	childCfg := session.SessionConfig{
		Model:          model,
		SystemPrompt:   "You are a helpful sub-agent. Complete the task and report your findings concisely.",
		MaxTurns:       AgentMaxTurns,
		TokenBudget:    compact.DefaultBudget(),
		PermissionMode: permissions.AutoApprove,
	}
	childSess := session.New(childCfg, tc.CWD)
	childSess.ParentSessionID = tc.SessionID
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
