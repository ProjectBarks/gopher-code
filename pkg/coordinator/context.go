package coordinator

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// MCPClient represents a connected MCP server with a name.
// Matches the TS ReadonlyArray<{ name: string }> parameter.
type MCPClient struct {
	Name string
}

// ScratchpadGateChecker is a function that checks whether the tengu_scratch
// statsig gate is enabled. Injected at init time; defaults to disabled.
// Duplicated from filesystem.ts to avoid circular dependencies.
// Source: coordinatorMode.ts:20-27 — isScratchpadGateEnabled()
var ScratchpadGateChecker func() bool

// isScratchpadGateEnabled returns whether the scratchpad gate is enabled.
func isScratchpadGateEnabled() bool {
	if ScratchpadGateChecker == nil {
		return false
	}
	return ScratchpadGateChecker()
}

// asyncAgentAllowedToolNames returns the sorted list of tool names from
// AsyncAgentAllowedTools, matching the TS source's Array.from(...).sort().
// We duplicate the set here rather than importing pkg/tools to avoid a
// circular dependency.
func asyncAgentAllowedToolNames() []string {
	// Source: constants/tools.ts:55-71 — ASYNC_AGENT_ALLOWED_TOOLS
	allowed := []string{
		"Bash",
		"Edit",
		"EnterWorktree",
		"ExitWorktree",
		"Glob",
		"Grep",
		"NotebookEdit",
		"Read",
		"Skill",
		"SyntheticOutput",
		"TodoWrite",
		"ToolSearch",
		"WebFetch",
		"WebSearch",
		"Write",
	}
	sort.Strings(allowed)
	return allowed
}

// GetCoordinatorUserContext returns context key-value pairs for coordinator mode.
// When coordinator mode is not active, returns nil.
//
// The returned map contains a "workerToolsContext" key describing available
// worker tools, MCP servers, and optional scratchpad directory.
//
// Source: coordinatorMode.ts:80-109 — getCoordinatorUserContext()
func GetCoordinatorUserContext(mcpClients []MCPClient, scratchpadDir string) map[string]string {
	if !IsCoordinatorMode() {
		return nil
	}

	var workerTools string
	if isEnvTruthy(os.Getenv("CLAUDE_CODE_SIMPLE")) {
		// Simple mode: limited tool set.
		simple := []string{"Bash", "Edit", "Read"}
		sort.Strings(simple)
		workerTools = strings.Join(simple, ", ")
	} else {
		// Full mode: ASYNC_AGENT_ALLOWED_TOOLS minus INTERNAL_WORKER_TOOLS.
		var filtered []string
		for _, name := range asyncAgentAllowedToolNames() {
			if !InternalWorkerTools[name] {
				filtered = append(filtered, name)
			}
		}
		workerTools = strings.Join(filtered, ", ")
	}

	content := fmt.Sprintf("Workers spawned via the %s tool have access to these tools: %s", agentToolName, workerTools)

	if len(mcpClients) > 0 {
		names := make([]string, len(mcpClients))
		for i, c := range mcpClients {
			names[i] = c.Name
		}
		content += "\n\nWorkers also have access to MCP tools from connected MCP servers: " + strings.Join(names, ", ")
	}

	if scratchpadDir != "" && isScratchpadGateEnabled() {
		content += fmt.Sprintf("\n\nScratchpad directory: %s\nWorkers can read and write here without permission prompts. Use this for durable cross-worker knowledge — structure files however fits the work.", scratchpadDir)
	}

	return map[string]string{"workerToolsContext": content}
}
