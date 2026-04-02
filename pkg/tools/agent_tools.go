package tools

import (
	"os"
	"strings"
)

// Source: tools/AgentTool/agentToolUtils.ts, constants/tools.ts

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
