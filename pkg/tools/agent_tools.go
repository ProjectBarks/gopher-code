package tools

import (
	"fmt"
	"os"
	"strings"
)

// Source: tools/AgentTool/agentToolUtils.ts, constants/tools.ts, constants.ts

// AgentTool name constants.
// Source: AgentTool/constants.ts:1-12
const (
	// AgentToolName is the primary tool name.
	AgentToolName = "Agent"
	// LegacyAgentToolName is the backward-compat alias (permission rules, hooks, resumed sessions).
	LegacyAgentToolName = "Task"
	// VerificationAgentType identifies the verification agent.
	VerificationAgentType = "verification"
)

// OneShotBuiltinAgentTypes are built-in agents that run once and return a
// report. The parent never SendMessages back to continue them, so we skip the
// agentId/SendMessage/usage trailer (~135 chars x 34M Explore runs/week).
// Source: AgentTool/constants.ts:9-12
var OneShotBuiltinAgentTypes = map[string]bool{
	"Explore": true,
	"Plan":    true,
}

// IsOneShotBuiltinAgent returns true if the agent type is a one-shot built-in.
func IsOneShotBuiltinAgent(agentType string) bool {
	return OneShotBuiltinAgentTypes[agentType]
}

// AgentToolInput is the parsed input for the Agent tool.
// Source: AgentTool/AgentTool.tsx:82-102
type AgentToolInput struct {
	Description    string `json:"description"`
	Prompt         string `json:"prompt"`
	SubagentType   string `json:"subagent_type,omitempty"`
	Model          string `json:"model,omitempty"`
	RunInBG        bool   `json:"run_in_background,omitempty"`
	Name           string `json:"name,omitempty"`
	TeamName       string `json:"team_name,omitempty"`
	Mode           string `json:"mode,omitempty"`
	Isolation      string `json:"isolation,omitempty"`
	CWD            string `json:"cwd,omitempty"`
}

// ValidModelEnums are the allowed short model names.
// Source: AgentTool/AgentTool.tsx:86
var ValidModelEnums = map[string]bool{
	"sonnet": true,
	"opus":   true,
	"haiku":  true,
}

// ValidIsolationModes are the allowed isolation values.
// Source: AgentTool/AgentTool.tsx:99
var ValidIsolationModes = map[string]bool{
	"worktree": true,
}

// ValidPermissionModes for the mode field.
// Source: AgentTool/AgentTool.tsx:97 — permissionModeSchema
var ValidPermissionModes = map[string]bool{
	"acceptEdits":       true,
	"auto":              true,
	"bypassPermissions": true,
	"default":           true,
	"dontAsk":           true,
	"plan":              true,
}

// ValidateAgentName validates the name parameter for spawned agents.
// Names must be non-empty, reasonably short, and contain only safe characters
// (lowercase letters, digits, hyphens). This matches the TS behavior where
// names are used as SendMessage targets and displayed in UI panels.
func ValidateAgentName(name string) error {
	if name == "" {
		return nil // name is optional
	}
	if len(name) > 64 {
		return fmt.Errorf("agent name too long (max 64 chars): %q", name)
	}
	for _, r := range name {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-') {
			return fmt.Errorf("agent name must contain only lowercase letters, digits, and hyphens: %q", name)
		}
	}
	if name[0] == '-' || name[len(name)-1] == '-' {
		return fmt.Errorf("agent name must not start or end with a hyphen: %q", name)
	}
	return nil
}

// ValidateAgentToolInput validates the input fields beyond basic JSON parsing.
// Source: AgentTool/AgentTool.tsx:239-280 — call() validation
func ValidateAgentToolInput(input *AgentToolInput) error {
	if input.Prompt == "" {
		return fmt.Errorf("prompt is required")
	}
	if input.Model != "" && !ValidModelEnums[input.Model] {
		return fmt.Errorf("invalid model %q: must be one of sonnet, opus, haiku", input.Model)
	}
	if input.Isolation != "" && !ValidIsolationModes[input.Isolation] {
		return fmt.Errorf("invalid isolation mode %q: must be \"worktree\"", input.Isolation)
	}
	if input.Mode != "" && !ValidPermissionModes[input.Mode] {
		return fmt.Errorf("invalid permission mode %q", input.Mode)
	}
	if input.CWD != "" && input.Isolation == "worktree" {
		return fmt.Errorf("cwd is mutually exclusive with isolation: \"worktree\"")
	}
	if err := ValidateAgentName(input.Name); err != nil {
		return err
	}
	return nil
}

// Tool deny/allow lists matching TS constants exactly.
// Source: constants/tools.ts:36-46
var AllAgentDisallowedTools = map[string]bool{
	"TaskOutput":    true, // Source: constants/tools.ts:37
	"ExitPlanMode":  true, // Source: constants/tools.ts:38
	"EnterPlanMode": true, // Source: constants/tools.ts:39
	"Agent":         true, // Source: constants/tools.ts:41 — blocked for non-ant users
	"AskUserQuestion": true, // Source: constants/tools.ts:42
	"TaskStop":      true, // Source: constants/tools.ts:43
}

// CustomAgentDisallowedTools is the deny list for non-built-in agents.
// Source: constants/tools.ts:48-50 — superset of AllAgentDisallowedTools
var CustomAgentDisallowedTools = map[string]bool{
	"TaskOutput":     true,
	"ExitPlanMode":   true,
	"EnterPlanMode":  true,
	"Agent":          true,
	"AskUserQuestion": true,
	"TaskStop":       true,
}

// AsyncAgentAllowedTools is the whitelist for background/async agents.
// Source: constants/tools.ts:55-71
var AsyncAgentAllowedTools = map[string]bool{
	"Read":          true, // FILE_READ_TOOL_NAME
	"WebSearch":     true,
	"TodoWrite":     true,
	"Grep":          true,
	"WebFetch":      true,
	"Glob":          true,
	"Bash":          true, // SHELL_TOOL_NAMES
	"Edit":          true, // FILE_EDIT_TOOL_NAME
	"Write":         true, // FILE_WRITE_TOOL_NAME
	"NotebookEdit":  true,
	"Skill":         true,
	"SyntheticOutput": true,
	"ToolSearch":    true,
	"EnterWorktree": true,
	"ExitWorktree":  true,
}

// InProcessTeammateAllowedTools are extra tools for in-process teammates.
// Source: constants/tools.ts:77-85
var InProcessTeammateAllowedTools = map[string]bool{
	"TaskCreate": true,
	"TaskGet":    true,
	"TaskList":   true,
	"TaskUpdate": true,
	"SendMessage": true,
}

// ResolvedAgentTools is the result of resolving agent tools.
// Source: agentToolUtils.ts:62-68
type ResolvedAgentTools struct {
	HasWildcard       bool
	ValidTools        []string
	InvalidTools      []string
	ResolvedTools     []Tool
	AllowedAgentTypes []string // extracted from Agent(type1, type2) spec
}

// FilterToolsForAgent removes disallowed tools based on agent type and mode.
// Source: agentToolUtils.ts:70-116
func FilterToolsForAgent(tools []Tool, isBuiltIn, isAsync bool, permissionMode string) []Tool {
	var filtered []Tool
	for _, tool := range tools {
		name := tool.Name()

		// Allow MCP tools for all agents
		// Source: agentToolUtils.ts:83
		if strings.HasPrefix(name, "mcp__") {
			filtered = append(filtered, tool)
			continue
		}

		// Allow ExitPlanMode in plan mode
		// Source: agentToolUtils.ts:88-93
		if name == "ExitPlanMode" && permissionMode == "plan" {
			filtered = append(filtered, tool)
			continue
		}

		// Block ALL_AGENT_DISALLOWED_TOOLS
		// Source: agentToolUtils.ts:94-96
		if AllAgentDisallowedTools[name] {
			continue
		}

		// Block CUSTOM_AGENT_DISALLOWED_TOOLS for non-built-in
		// Source: agentToolUtils.ts:97-99
		if !isBuiltIn && CustomAgentDisallowedTools[name] {
			continue
		}

		// For async, only allow ASYNC_AGENT_ALLOWED_TOOLS
		// Source: agentToolUtils.ts:100-113
		if isAsync && !AsyncAgentAllowedTools[name] {
			// Teammates can also use InProcessTeammateAllowedTools
			if InProcessTeammateAllowedTools[name] {
				filtered = append(filtered, tool)
				continue
			}
			continue
		}

		filtered = append(filtered, tool)
	}
	return filtered
}

// ResolveAgentTools resolves and validates agent tools against available tools.
// Source: agentToolUtils.ts:122-225
func ResolveAgentTools(
	agentTools []string, // nil means wildcard (all tools)
	disallowedTools []string,
	source string, // "built-in", "userSettings", etc.
	permissionMode string,
	availableTools []Tool,
	isAsync bool,
	isMainThread bool,
) ResolvedAgentTools {
	// When isMainThread, skip filtering — main thread pool already assembled
	// Source: agentToolUtils.ts:140-147
	var filteredAvailable []Tool
	if isMainThread {
		filteredAvailable = availableTools
	} else {
		isBuiltIn := source == "built-in"
		filteredAvailable = FilterToolsForAgent(availableTools, isBuiltIn, isAsync, permissionMode)
	}

	// Build disallowed set
	// Source: agentToolUtils.ts:150-155
	disallowedSet := make(map[string]bool)
	for _, spec := range disallowedTools {
		toolName := extractToolName(spec)
		disallowedSet[toolName] = true
	}

	// Filter by disallowed list
	// Source: agentToolUtils.ts:158-160
	var allowedAvailable []Tool
	for _, tool := range filteredAvailable {
		if !disallowedSet[tool.Name()] {
			allowedAvailable = append(allowedAvailable, tool)
		}
	}

	// If tools is nil or ["*"], allow all (wildcard)
	// Source: agentToolUtils.ts:163-173
	hasWildcard := agentTools == nil ||
		(len(agentTools) == 1 && agentTools[0] == "*")
	if hasWildcard {
		return ResolvedAgentTools{
			HasWildcard:   true,
			ResolvedTools: allowedAvailable,
		}
	}

	// Build lookup map
	// Source: agentToolUtils.ts:175-178
	availableMap := make(map[string]Tool)
	for _, tool := range allowedAvailable {
		availableMap[tool.Name()] = tool
	}

	var (
		validTools        []string
		invalidTools      []string
		resolved          []Tool
		resolvedSet       = make(map[string]bool)
		allowedAgentTypes []string
	)

	// Process each tool spec
	// Source: agentToolUtils.ts:186-224
	for _, spec := range agentTools {
		toolName := extractToolName(spec)
		ruleContent := extractRuleContent(spec)

		// Special case: Agent tool carries allowedAgentTypes
		// Source: agentToolUtils.ts:191-204
		if toolName == "Agent" {
			if ruleContent != "" {
				parts := strings.Split(ruleContent, ",")
				for _, p := range parts {
					p = strings.TrimSpace(p)
					if p != "" {
						allowedAgentTypes = append(allowedAgentTypes, p)
					}
				}
			}
			if !isMainThread {
				validTools = append(validTools, spec)
				continue
			}
		}

		// Resolve tool
		// Source: agentToolUtils.ts:206-215
		tool, ok := availableMap[toolName]
		if ok {
			validTools = append(validTools, spec)
			if !resolvedSet[toolName] {
				resolved = append(resolved, tool)
				resolvedSet[toolName] = true
			}
		} else {
			invalidTools = append(invalidTools, spec)
		}
	}

	return ResolvedAgentTools{
		HasWildcard:       false,
		ValidTools:        validTools,
		InvalidTools:      invalidTools,
		ResolvedTools:     resolved,
		AllowedAgentTypes: allowedAgentTypes,
	}
}

// extractToolName extracts the tool name from a permission rule spec.
// "Bash(git *)" → "Bash", "Read" → "Read"
// Source: utils/permissions/permissionRuleParser.ts — permissionRuleValueFromString
func extractToolName(spec string) string {
	idx := strings.Index(spec, "(")
	if idx >= 0 {
		return spec[:idx]
	}
	return spec
}

// extractRuleContent extracts the content from a permission rule spec.
// "Agent(Explore, Plan)" → "Explore, Plan", "Read" → ""
func extractRuleContent(spec string) string {
	start := strings.Index(spec, "(")
	end := strings.LastIndex(spec, ")")
	if start >= 0 && end > start {
		return spec[start+1 : end]
	}
	return ""
}

// GetAgentModel resolves the effective model for a sub-agent.
// Priority: env override > tool-specified > agent definition > inherit from parent.
// Source: utils/model/agent.ts:37-95
func GetAgentModel(agentModel, parentModel, toolSpecifiedModel string) string {
	// 1. Environment override
	// Source: agent.ts:43-45
	if env := getEnvVar("CLAUDE_CODE_SUBAGENT_MODEL"); env != "" {
		return env
	}

	// 2. Tool-specified model
	// Source: agent.ts:70-76
	if toolSpecifiedModel != "" {
		return toolSpecifiedModel
	}

	// 3. Agent definition model (default to "inherit")
	// Source: agent.ts:78-88
	effectiveModel := agentModel
	if effectiveModel == "" {
		effectiveModel = getDefaultSubagentModel()
	}

	if effectiveModel == "inherit" {
		return parentModel
	}

	return effectiveModel
}

// getDefaultSubagentModel returns the default model for subagents.
// Source: utils/model/agent.ts — "inherit" by default
func getDefaultSubagentModel() string {
	return "inherit"
}

// getEnvVar reads an environment variable. Overridable for testing.
var getEnvVar = os.Getenv

// AgentToolPromptCore returns the shared core prompt used by both coordinator
// and non-coordinator modes. The agentListSection is injected dynamically.
// Source: AgentTool/prompt.ts:202-217
func AgentToolPromptCore(agentListSection string) string {
	return "Launch a new agent to handle complex, multi-step tasks autonomously.\n\n" +
		"The " + AgentToolName + " tool launches specialized agents (subprocesses) that autonomously handle complex tasks. " +
		"Each agent type has specific capabilities and tools available to it.\n\n" +
		agentListSection + "\n\n" +
		"When using the " + AgentToolName + " tool, specify a subagent_type parameter to select which agent type to use. " +
		"If omitted, the general-purpose agent is used."
}

// AgentToolPrompt returns the full non-coordinator prompt with usage notes,
// when-not-to-use section, and examples.
// Source: AgentTool/prompt.ts:252-286
func AgentToolPrompt(agentListSection string) string {
	core := AgentToolPromptCore(agentListSection)

	whenNotToUse := "\nWhen NOT to use the " + AgentToolName + " tool:\n" +
		"- If you want to read a specific file path, use the Read tool or the Glob tool instead of the " + AgentToolName + " tool, to find the match more quickly\n" +
		"- If you are searching for a specific class definition like \"class Foo\", use the Glob tool instead, to find the match more quickly\n" +
		"- If you are searching for code within a specific file or set of 2-3 files, use the Read tool instead of the " + AgentToolName + " tool, to find the match more quickly\n" +
		"- Other tasks that are not related to the agent descriptions above\n"

	usageNotes := "\nUsage notes:\n" +
		"- Always include a short description (3-5 words) summarizing what the agent will do\n" +
		"- When the agent is done, it will return a single message back to you. The result returned by the agent is not visible to the user. To show the user the result, you should send a text message back to the user with a concise summary of the result.\n" +
		"- You can optionally run agents in the background using the run_in_background parameter. When an agent runs in the background, you will be automatically notified when it completes — do NOT sleep, poll, or proactively check on its progress. Continue with other work or respond to the user instead.\n" +
		"- **Foreground vs background**: Use foreground (default) when you need the agent's results before you can proceed — e.g., research agents whose findings inform your next steps. Use background when you have genuinely independent work to do in parallel.\n" +
		"- To continue a previously spawned agent, use SendMessage with the agent's ID or name as the `to` field. The agent resumes with its full context preserved. Each Agent invocation starts fresh — provide a complete task description.\n" +
		"- The agent's outputs should generally be trusted\n" +
		"- Clearly tell the agent whether you expect it to write code or just to do research (search, file reads, web fetches, etc.), since it is not aware of the user's intent\n" +
		"- If the agent description mentions that it should be used proactively, then you should try your best to use it without the user having to ask for it first. Use your judgement.\n" +
		"- If the user specifies that they want you to run agents \"in parallel\", you MUST send a single message with multiple " + AgentToolName + " tool use content blocks. For example, if you need to launch both a build-validator agent and a test-runner agent in parallel, send a single message with both tool calls.\n" +
		"- You can optionally set `isolation: \"worktree\"` to run the agent in a temporary git worktree, giving it an isolated copy of the repository. The worktree is automatically cleaned up if the agent makes no changes; if changes are made, the worktree path and branch are returned in the result."

	writingPrompt := "\n\n## Writing the prompt\n\n" +
		"Brief the agent like a smart colleague who just walked into the room — it hasn't seen this conversation, doesn't know what you've tried, doesn't understand why this task matters.\n" +
		"- Explain what you're trying to accomplish and why.\n" +
		"- Describe what you've already learned or ruled out.\n" +
		"- Give enough context about the surrounding problem that the agent can make judgment calls rather than just following a narrow instruction.\n" +
		"- If you need a short response, say so (\"report in under 200 words\").\n" +
		"- Lookups: hand over the exact command. Investigations: hand over the question — prescribed steps become dead weight when the premise is wrong.\n\n" +
		"Terse command-style prompts produce shallow, generic work.\n\n" +
		"**Never delegate understanding.** Don't write \"based on your findings, fix the bug\" or \"based on the research, implement it.\" " +
		"Those phrases push synthesis onto the agent instead of doing it yourself. Write prompts that prove you understood: include file paths, line numbers, what specifically to change.\n"

	return core + "\n" + whenNotToUse + usageNotes + writingPrompt
}
