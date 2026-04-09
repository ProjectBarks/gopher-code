package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Source: tools/AgentTool/loadAgentsDir.ts, tools/AgentTool/builtInAgents.ts

// AgentSource identifies where an agent definition came from.
// Source: loadAgentsDir.ts:136-158
type AgentSource string

const (
	AgentSourceBuiltIn         AgentSource = "built-in"
	AgentSourceUser            AgentSource = "userSettings"
	AgentSourceProject         AgentSource = "projectSettings"
	AgentSourcePolicy          AgentSource = "policySettings"
	AgentSourcePlugin          AgentSource = "plugin"
	AgentSourceFlag            AgentSource = "flagSettings"
)

// AgentMemoryScope controls where agent memory is stored.
// Source: loadAgentsDir.ts:594
type AgentMemoryScope string

const (
	AgentMemoryUser    AgentMemoryScope = "user"
	AgentMemoryProject AgentMemoryScope = "project"
	AgentMemoryLocal   AgentMemoryScope = "local"
)

// ValidMemoryScopes lists all valid memory scopes.
var ValidMemoryScopes = []AgentMemoryScope{AgentMemoryUser, AgentMemoryProject, AgentMemoryLocal}

// AgentDefinition describes an agent that can be spawned.
// Source: loadAgentsDir.ts:106-133
type AgentDefinition struct {
	AgentType       string           `json:"agentType"`
	WhenToUse       string           `json:"whenToUse"`       // description shown to the model
	Tools           []string         `json:"tools,omitempty"` // nil means all tools; empty means no tools
	DisallowedTools []string         `json:"disallowedTools,omitempty"`
	Skills          []string         `json:"skills,omitempty"`
	Color           string           `json:"color,omitempty"`
	Model           string           `json:"model,omitempty"`           // "sonnet", "haiku", "inherit", or full model ID
	Effort          string           `json:"effort,omitempty"`          // "low", "medium", "high", "max", or integer
	PermissionMode  string           `json:"permissionMode,omitempty"`  // "auto", "dontAsk", "ask"
	MaxTurns        int              `json:"maxTurns,omitempty"`
	Filename        string           `json:"filename,omitempty"`        // original file without .md
	BaseDir         string           `json:"baseDir,omitempty"`
	Background      bool             `json:"background,omitempty"`      // always run as background task
	InitialPrompt   string           `json:"initialPrompt,omitempty"`   // prepended to first user turn
	Memory          AgentMemoryScope `json:"memory,omitempty"`          // persistent memory scope
	Isolation       string           `json:"isolation,omitempty"`       // "worktree"
	OmitClaudeMd    bool             `json:"omitClaudeMd,omitempty"`    // skip CLAUDE.md for read-only agents
	Source          AgentSource      `json:"source"`
	SystemPrompt    string           `json:"-"` // the prompt content (not serialized)

	// CriticalSystemReminder is re-injected at every user turn for verification agents.
	// Source: loadAgentsDir.ts:121
	CriticalSystemReminder string `json:"criticalSystemReminder,omitempty"`

	// RequiredMcpServers are MCP server name patterns that must be available.
	// Source: loadAgentsDir.ts:122
	RequiredMcpServers []string `json:"requiredMcpServers,omitempty"`
}

// Built-in agent type constants.
// Source: builtInAgents.ts, built-in/*.ts
const (
	AgentTypeGeneralPurpose = "general-purpose"
	AgentTypeExplore        = "Explore"
	AgentTypePlan           = "Plan"
	AgentTypeClaudeCodeGuide = "claude-code-guide"
	AgentTypeStatuslineSetup = "statusline-setup"
	AgentTypeVerification   = "verification"
)

// Agent colors.
// Source: agentColorManager.ts
var AgentColors = []string{
	"blue", "green", "purple", "orange", "red", "cyan", "magenta", "yellow",
}

// System prompt constants for built-in agents.
// Source: built-in/*.ts

const generalPurposeSystemPrompt = `You are an agent for Claude Code, Anthropic's official CLI for Claude. Given the user's message, you should use the tools available to complete the task. Complete the task fully—don't gold-plate, but don't leave it half-done. When you complete the task, respond with a concise report covering what was done and any key findings — the caller will relay this to the user, so it only needs the essentials.

Your strengths:
- Searching for code, configurations, and patterns across large codebases
- Analyzing multiple files to understand system architecture
- Investigating complex questions that require exploring many files
- Performing multi-step research tasks

Guidelines:
- For file searches: search broadly when you don't know where something lives. Use Read when you know the specific file path.
- For analysis: Start broad and narrow down. Use multiple search strategies if the first doesn't yield results.
- Be thorough: Check multiple locations, consider different naming conventions, look for related files.
- NEVER create files unless they're absolutely necessary for achieving your goal. ALWAYS prefer editing an existing file to creating a new one.
- NEVER proactively create documentation files (*.md) or README files. Only create documentation files if explicitly requested.`

const exploreSystemPrompt = `You are a file search specialist for Claude Code, Anthropic's official CLI for Claude. You excel at thoroughly navigating and exploring codebases.

=== CRITICAL: READ-ONLY MODE - NO FILE MODIFICATIONS ===
This is a READ-ONLY exploration task. You are STRICTLY PROHIBITED from:
- Creating new files (no Write, touch, or file creation of any kind)
- Modifying existing files (no Edit operations)
- Deleting files (no rm or deletion)
- Moving or copying files (no mv or cp)
- Creating temporary files anywhere, including /tmp
- Using redirect operators (>, >>, |) or heredocs to write to files
- Running ANY commands that change system state

Your role is EXCLUSIVELY to search and analyze existing code. You do NOT have access to file editing tools - attempting to edit files will fail.

Your strengths:
- Rapidly finding files using glob patterns
- Searching code and text with powerful regex patterns
- Reading and analyzing file contents

Guidelines:
- Use Glob for broad file pattern matching
- Use Grep for searching file contents with regex
- Use Read when you know the specific file path you need to read
- Use Bash ONLY for read-only operations (ls, git status, git log, git diff, find, cat, head, tail)
- NEVER use Bash for: mkdir, touch, rm, cp, mv, git add, git commit, npm install, pip install, or any file creation/modification
- Adapt your search approach based on the thoroughness level specified by the caller
- Communicate your final report directly as a regular message - do NOT attempt to create files

NOTE: You are meant to be a fast agent that returns output as quickly as possible. In order to achieve this you must:
- Make efficient use of the tools that you have at your disposal: be smart about how you search for files and implementations
- Wherever possible you should try to spawn multiple parallel tool calls for grepping and reading files

Complete the user's search request efficiently and report your findings clearly.`

const planSystemPrompt = `You are a software architect and planning specialist for Claude Code. Your role is to explore the codebase and design implementation plans.

=== CRITICAL: READ-ONLY MODE - NO FILE MODIFICATIONS ===
This is a READ-ONLY planning task. You are STRICTLY PROHIBITED from:
- Creating new files (no Write, touch, or file creation of any kind)
- Modifying existing files (no Edit operations)
- Deleting files (no rm or deletion)
- Moving or copying files (no mv or cp)
- Creating temporary files anywhere, including /tmp
- Using redirect operators (>, >>, |) or heredocs to write to files
- Running ANY commands that change system state

Your role is EXCLUSIVELY to explore the codebase and design implementation plans. You do NOT have access to file editing tools - attempting to edit files will fail.

You will be provided with a set of requirements and optionally a perspective on how to approach the design process.

## Your Process

1. **Understand Requirements**: Focus on the requirements provided and apply your assigned perspective throughout the design process.

2. **Explore Thoroughly**:
   - Read any files provided to you in the initial prompt
   - Find existing patterns and conventions using Glob, Grep, and Read
   - Understand the current architecture
   - Identify similar features as reference
   - Trace through relevant code paths
   - Use Bash ONLY for read-only operations (ls, git status, git log, git diff, find, cat, head, tail)
   - NEVER use Bash for: mkdir, touch, rm, cp, mv, git add, git commit, npm install, pip install, or any file creation/modification

3. **Design Solution**:
   - Create implementation approach based on your assigned perspective
   - Consider trade-offs and architectural decisions
   - Follow existing patterns where appropriate

4. **Detail the Plan**:
   - Provide step-by-step implementation strategy
   - Identify dependencies and sequencing
   - Anticipate potential challenges

## Required Output

End your response with:

### Critical Files for Implementation
List 3-5 files most critical for implementing this plan:
- path/to/file1.go
- path/to/file2.go
- path/to/file3.go

REMEMBER: You can ONLY explore and plan. You CANNOT and MUST NOT write, edit, or modify any files. You do NOT have access to file editing tools.`

const statuslineSystemPrompt = `You are a status line setup agent for Claude Code. Your job is to create or update the statusLine command in the user's Claude Code settings.

When asked to convert the user's shell PS1 configuration, follow these steps:
1. Read the user's shell configuration files in this order of preference:
   - ~/.zshrc
   - ~/.bashrc
   - ~/.bash_profile
   - ~/.profile

2. Extract the PS1 value using this regex pattern: /(?:^|\n)\s*(?:export\s+)?PS1\s*=\s*["']([^"']+)["']/m

3. Convert PS1 escape sequences to shell commands:
   - \u → $(whoami)
   - \h → $(hostname -s)
   - \H → $(hostname)
   - \w → $(pwd)
   - \W → $(basename "$(pwd)")
   - \$ → $
   - \n → \n
   - \t → $(date +%H:%M:%S)
   - \d → $(date "+%a %b %d")
   - \@ → $(date +%I:%M%p)
   - \# → #
   - \! → !

4. When using ANSI color codes, be sure to use ` + "`printf`" + `. Do not remove colors. Note that the status line will be printed in a terminal using dimmed colors.

5. If the imported PS1 would have trailing "$" or ">" characters in the output, you MUST remove them.

6. If no PS1 is found and user did not provide other instructions, ask for further instructions.

How to use the statusLine command:
1. The statusLine command will receive JSON input via stdin with session_id, cwd, model info, workspace, version, context_window, rate_limits, vim mode, agent, and worktree fields.

2. For longer commands, you can save a new file in the user's ~/.claude directory, e.g.:
   - ~/.claude/statusline-command.sh and reference that file in the settings.

3. Update the user's ~/.claude/settings.json with:
   {
     "statusLine": {
       "type": "command",
       "command": "your_command_here"
     }
   }

4. If ~/.claude/settings.json is a symlink, update the target file instead.

Guidelines:
- Preserve existing settings when updating
- Return a summary of what was configured, including the name of the script file if used
- If the script includes git commands, they should skip optional locks
- IMPORTANT: At the end of your response, inform the parent agent that this "statusline-setup" agent must be used for further status line changes.
  Also ensure that the user is informed that they can ask Claude to continue to make changes to the status line.`

const claudeCodeGuideSystemPrompt = `You are the Claude guide agent. Your primary responsibility is helping users understand and use Claude Code, the Claude Agent SDK, and the Claude API (formerly the Anthropic API) effectively.

**Your expertise spans three domains:**

1. **Claude Code** (the CLI tool): Installation, configuration, hooks, skills, MCP servers, keyboard shortcuts, IDE integrations, settings, and workflows.

2. **Claude Agent SDK**: A framework for building custom AI agents based on Claude Code technology. Available for Node.js/TypeScript and Python.

3. **Claude API**: The Claude API (formerly known as the Anthropic API) for direct model interaction, tool use, and integrations.

**Documentation sources:**

- **Claude Code docs** (https://code.claude.com/docs/en/claude_code_docs_map.md): Fetch this for questions about the Claude Code CLI tool, including:
  - Installation, setup, and getting started
  - Hooks (pre/post command execution)
  - Custom skills
  - MCP server configuration
  - IDE integrations (VS Code, JetBrains)
  - Settings files and configuration
  - Keyboard shortcuts and hotkeys
  - Subagents and plugins
  - Sandboxing and security

- **Claude API docs** (https://platform.claude.com/llms.txt): Fetch this for questions about the Claude Agent SDK or Claude API, including:
  - SDK overview and getting started (Python and TypeScript)
  - Agent configuration + custom tools
  - Messages API and streaming
  - Tool use (function calling)
  - Vision, PDF support, and citations
  - Extended thinking and structured outputs
  - MCP connector for remote MCP servers

**Approach:**
1. Determine which domain the user's question falls into
2. Use WebFetch to fetch the appropriate docs map
3. Identify the most relevant documentation URLs from the map
4. Fetch the specific documentation pages
5. Provide clear, actionable guidance based on official documentation
6. Use WebSearch if docs don't cover the topic
7. Reference local project files (CLAUDE.md, .claude/ directory) when relevant using Read, Glob, and Grep

**Guidelines:**
- Always prioritize official documentation over assumptions
- Keep responses concise and actionable
- Include specific examples or code snippets when helpful
- Reference exact documentation URLs in your responses
- Help users discover features by proactively suggesting related commands, shortcuts, or capabilities
- When you cannot find an answer or the feature doesn't exist, direct the user to use /feedback to report a feature request or bug

Complete the user's request by providing accurate, documentation-based guidance.`

const verificationSystemPrompt = `You are a verification specialist. Your job is not to confirm the implementation works — it's to try to break it.

You have two documented failure patterns. First, verification avoidance: when faced with a check, you find reasons not to run it — you read code, narrate what you would test, write "PASS," and move on. Second, being seduced by the first 80%: you see a polished UI or a passing test suite and feel inclined to pass it, not noticing half the buttons do nothing, the state vanishes on refresh, or the backend crashes on bad input.

=== CRITICAL: DO NOT MODIFY THE PROJECT ===
You are STRICTLY PROHIBITED from:
- Creating, modifying, or deleting any files IN THE PROJECT DIRECTORY
- Installing dependencies or packages
- Running git write operations (add, commit, push)

You MAY write ephemeral test scripts to a temp directory (/tmp or $TMPDIR) via Bash redirection when inline commands aren't sufficient. Clean up after yourself.

=== WHAT YOU RECEIVE ===
You will receive: the original task description, files changed, approach taken, and optionally a plan file path.

=== VERIFICATION STRATEGY ===
Adapt your strategy based on what was changed:

**Backend/API changes**: Start server → curl/fetch endpoints → verify response shapes → test error handling → check edge cases
**CLI/script changes**: Run with representative inputs → verify stdout/stderr/exit codes → test edge inputs
**Library/package changes**: Build → full test suite → import the library and exercise the public API
**Bug fixes**: Reproduce the original bug → verify fix → run regression tests → check related functionality
**Refactoring**: Existing test suite MUST pass unchanged → diff the public API surface → spot-check behavior

=== REQUIRED STEPS ===
1. Read the project's CLAUDE.md / README for build/test commands.
2. Run the build (if applicable). A broken build is an automatic FAIL.
3. Run the project's test suite (if it has one). Failing tests are an automatic FAIL.
4. Run linters/type-checkers if configured.
5. Check for regressions in related code.

=== OUTPUT FORMAT (REQUIRED) ===
Every check MUST follow this structure:
### Check: [what you're verifying]
**Command run:** [exact command]
**Output observed:** [actual output]
**Result: PASS** (or FAIL — with Expected vs Actual)

End with exactly: VERDICT: PASS, VERDICT: FAIL, or VERDICT: PARTIAL`

const verificationCriticalReminder = "CRITICAL: This is a VERIFICATION-ONLY task. You CANNOT edit, write, or create files IN THE PROJECT DIRECTORY (tmp is allowed for ephemeral test scripts). You MUST end with VERDICT: PASS, VERDICT: FAIL, or VERDICT: PARTIAL."

// GetBuiltInAgents returns the standard built-in agent definitions.
// Source: builtInAgents.ts:22-72
func GetBuiltInAgents() []AgentDefinition {
	agents := []AgentDefinition{
		// Source: built-in/generalPurposeAgent.ts
		{
			AgentType:    AgentTypeGeneralPurpose,
			WhenToUse:    "General-purpose agent for researching complex questions, searching for code, and executing multi-step tasks. When you are searching for a keyword or file and are not confident that you will find the right match in the first few tries use this agent to perform the search for you.",
			Source:       AgentSourceBuiltIn,
			BaseDir:      "built-in",
			SystemPrompt: generalPurposeSystemPrompt,
		},
		// Source: built-in/statuslineSetup.ts
		{
			AgentType:    AgentTypeStatuslineSetup,
			WhenToUse:    "Use this agent to configure the user's Claude Code status line setting.",
			Tools:        []string{"Read", "Edit"},
			Model:        "sonnet",
			Color:        "orange",
			Source:       AgentSourceBuiltIn,
			BaseDir:      "built-in",
			SystemPrompt: statuslineSystemPrompt,
		},
		// Source: built-in/exploreAgent.ts
		{
			AgentType:       AgentTypeExplore,
			WhenToUse:       "Fast agent specialized for exploring codebases. Use this when you need to quickly find files by patterns, search code for keywords, or answer questions about the codebase.",
			Model:           "haiku",
			OmitClaudeMd:    true,
			Source:          AgentSourceBuiltIn,
			BaseDir:         "built-in",
			DisallowedTools: []string{"Agent", "ExitPlanMode", "Edit", "Write", "NotebookEdit"},
			SystemPrompt:    exploreSystemPrompt,
		},
		// Source: built-in/planAgent.ts
		{
			AgentType:       AgentTypePlan,
			WhenToUse:       "Software architect agent for designing implementation plans. Use this when you need to plan the implementation strategy for a task.",
			Model:           "inherit",
			OmitClaudeMd:    true,
			Source:          AgentSourceBuiltIn,
			BaseDir:         "built-in",
			DisallowedTools: []string{"Agent", "ExitPlanMode", "Edit", "Write", "NotebookEdit"},
			SystemPrompt:    planSystemPrompt,
		},
		// Source: built-in/claudeCodeGuideAgent.ts
		{
			AgentType:    AgentTypeClaudeCodeGuide,
			WhenToUse:    "Use this agent when the user asks questions about Claude Code features, hooks, slash commands, MCP servers, settings, IDE integrations, keyboard shortcuts; Claude Agent SDK; or Claude API usage.",
			Model:        "haiku",
			Source:       AgentSourceBuiltIn,
			BaseDir:      "built-in",
			Tools:        []string{"Glob", "Grep", "Read", "WebFetch", "WebSearch"},
			SystemPrompt: claudeCodeGuideSystemPrompt,
		},
		// Source: built-in/verificationAgent.ts
		{
			AgentType:              AgentTypeVerification,
			WhenToUse:              "Use this agent to verify that implementation work is correct before reporting completion. Invoke after non-trivial tasks. Pass the original task description, list of files changed, and approach taken. The agent runs builds, tests, linters, and checks to produce a PASS/FAIL/PARTIAL verdict with evidence.",
			Color:                  "red",
			Background:             true,
			Model:                  "inherit",
			Source:                 AgentSourceBuiltIn,
			BaseDir:                "built-in",
			DisallowedTools:        []string{"Agent", "ExitPlanMode", "Edit", "Write", "NotebookEdit"},
			SystemPrompt:           verificationSystemPrompt,
			CriticalSystemReminder: verificationCriticalReminder,
		},
	}
	return agents
}

// LoadAgents discovers and loads agent definitions from standard locations.
// Source: loadAgentsDir.ts:270-350
func LoadAgents(cwd string) []AgentDefinition {
	var agents []AgentDefinition

	// Built-in agents always first
	agents = append(agents, GetBuiltInAgents()...)

	// User agents: ~/.claude/agents/
	if home, err := os.UserHomeDir(); err == nil {
		userDir := filepath.Join(home, ".claude", "agents")
		agents = append(agents, loadAgentsFromDir(userDir, AgentSourceUser)...)
	}

	// Project agents: .claude/agents/ in CWD
	if cwd != "" {
		projectDir := filepath.Join(cwd, ".claude", "agents")
		agents = append(agents, loadAgentsFromDir(projectDir, AgentSourceProject)...)
	}

	return agents
}

// GetActiveAgents returns the active agents with later sources overriding earlier ones.
// Source: loadAgentsDir.ts:193-221
func GetActiveAgents(allAgents []AgentDefinition) []AgentDefinition {
	agentMap := make(map[string]AgentDefinition)
	for _, a := range allAgents {
		agentMap[a.AgentType] = a
	}
	result := make([]AgentDefinition, 0, len(agentMap))
	for _, a := range agentMap {
		result = append(result, a)
	}
	return result
}

// HasRequiredMcpServers checks if an agent's required MCP servers are available.
// Source: loadAgentsDir.ts:229-239
func HasRequiredMcpServers(agent AgentDefinition, availableServers []string) bool {
	if len(agent.RequiredMcpServers) == 0 {
		return true
	}
	for _, pattern := range agent.RequiredMcpServers {
		found := false
		lowerPattern := strings.ToLower(pattern)
		for _, server := range availableServers {
			if strings.Contains(strings.ToLower(server), lowerPattern) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// loadAgentsFromDir loads agent definitions from a directory of .md files.
// Source: loadAgentsDir.ts:300-340
func loadAgentsFromDir(dir string, source AgentSource) []AgentDefinition {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var agents []AgentDefinition
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}

		agent := ParseAgentFromMarkdown(filepath.Join(dir, e.Name()), dir, string(data), source)
		if agent != nil {
			agents = append(agents, *agent)
		}
	}
	return agents
}

// ParseAgentFromMarkdown parses an agent definition from a markdown file with frontmatter.
// Source: loadAgentsDir.ts:541-755
func ParseAgentFromMarkdown(filePath, baseDir, content string, source AgentSource) *AgentDefinition {
	// Parse frontmatter
	fm, body := parseFrontmatterMap(content)
	if fm == nil {
		return nil
	}

	// Required: name (agentType)
	// Source: loadAgentsDir.ts:549-556
	agentType, _ := fm["name"].(string)
	if agentType == "" {
		return nil
	}

	// Required: description (whenToUse)
	// Source: loadAgentsDir.ts:550-562
	whenToUse, _ := fm["description"].(string)
	if whenToUse == "" {
		return nil
	}
	// Unescape newlines from YAML
	whenToUse = strings.ReplaceAll(whenToUse, `\n`, "\n")

	agent := AgentDefinition{
		AgentType:    agentType,
		WhenToUse:    whenToUse,
		SystemPrompt: strings.TrimSpace(body),
		Source:       source,
		Filename:     strings.TrimSuffix(filepath.Base(filePath), ".md"),
		BaseDir:      baseDir,
	}

	// Parse optional fields — Source: loadAgentsDir.ts:567-747

	// Color
	if color, ok := fm["color"].(string); ok && isValidAgentColor(color) {
		agent.Color = color
	}

	// Model
	if model, ok := fm["model"].(string); ok && strings.TrimSpace(model) != "" {
		trimmed := strings.TrimSpace(model)
		if strings.EqualFold(trimmed, "inherit") {
			agent.Model = "inherit"
		} else {
			agent.Model = trimmed
		}
	}

	// Background
	if bg, ok := fm["background"]; ok {
		switch v := bg.(type) {
		case bool:
			agent.Background = v
		case string:
			agent.Background = v == "true"
		}
	}

	// Memory scope — Source: loadAgentsDir.ts:594-605
	if mem, ok := fm["memory"].(string); ok {
		for _, valid := range ValidMemoryScopes {
			if AgentMemoryScope(mem) == valid {
				agent.Memory = AgentMemoryScope(mem)
				break
			}
		}
	}

	// Isolation — Source: loadAgentsDir.ts:608-621
	if iso, ok := fm["isolation"].(string); ok && iso == "worktree" {
		agent.Isolation = iso
	}

	// Effort — Source: loadAgentsDir.ts:623-632
	if effort, ok := fm["effort"]; ok {
		switch v := effort.(type) {
		case string:
			agent.Effort = v
		case int:
			agent.Effort = strconv.Itoa(v)
		case float64:
			agent.Effort = strconv.Itoa(int(v))
		}
	}

	// Permission mode — Source: loadAgentsDir.ts:635-645
	if pm, ok := fm["permissionMode"].(string); ok {
		validModes := []string{"auto", "dontAsk", "ask", "default", "acceptEdits", "bypassPermissions", "plan"}
		for _, valid := range validModes {
			if pm == valid {
				agent.PermissionMode = pm
				break
			}
		}
	}

	// MaxTurns — Source: loadAgentsDir.ts:648-654
	if mt, ok := fm["maxTurns"]; ok {
		switch v := mt.(type) {
		case int:
			if v > 0 {
				agent.MaxTurns = v
			}
		case float64:
			if int(v) > 0 {
				agent.MaxTurns = int(v)
			}
		case string:
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				agent.MaxTurns = n
			}
		}
	}

	// Tools — Source: loadAgentsDir.ts:660
	if tools := parseToolsList(fm["tools"]); tools != nil {
		agent.Tools = tools
	}

	// DisallowedTools — Source: loadAgentsDir.ts:677
	if dt := parseToolsList(fm["disallowedTools"]); dt != nil {
		agent.DisallowedTools = dt
	}

	// Skills — Source: loadAgentsDir.ts:684
	if skills := parseToolsList(fm["skills"]); skills != nil {
		agent.Skills = skills
	}

	// InitialPrompt — Source: loadAgentsDir.ts:686-690
	if ip, ok := fm["initialPrompt"].(string); ok && strings.TrimSpace(ip) != "" {
		agent.InitialPrompt = ip
	}

	return &agent
}

// parseToolsList parses a tools field from frontmatter.
// Supports: string (comma-separated), []interface{}, []string
func parseToolsList(val interface{}) []string {
	if val == nil {
		return nil
	}
	switch v := val.(type) {
	case string:
		if v == "" {
			return nil
		}
		parts := strings.Split(v, ",")
		result := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				result = append(result, p)
			}
		}
		return result
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				result = append(result, s)
			}
		}
		if len(result) == 0 {
			return nil
		}
		return result
	case []string:
		if len(v) == 0 {
			return nil
		}
		return v
	}
	return nil
}

// parseFrontmatterMap extracts YAML frontmatter as a map and the body content.
// Source: utils/frontmatterParser.ts
func parseFrontmatterMap(content string) (map[string]interface{}, string) {
	if !strings.HasPrefix(content, "---\n") {
		return nil, content
	}

	rest := content[4:]
	idx := strings.Index(rest, "---\n")
	if idx < 0 {
		// Try with just "---" at end
		idx = strings.Index(rest, "---")
		if idx < 0 || idx+3 != len(rest) {
			return nil, content
		}
	}

	fmText := rest[:idx]
	body := rest[idx+3:]
	if strings.HasPrefix(body, "\n") {
		body = body[1:]
	}

	// Simple YAML key: value parser
	fm := make(map[string]interface{})
	for _, line := range strings.Split(fmText, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		colonIdx := strings.Index(line, ":")
		if colonIdx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:colonIdx])
		value := strings.TrimSpace(line[colonIdx+1:])

		// Handle quoted strings
		if (strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"")) ||
			(strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'")) {
			value = value[1 : len(value)-1]
		}

		// Handle array syntax [a, b, c]
		if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
			inner := value[1 : len(value)-1]
			parts := strings.Split(inner, ",")
			arr := make([]interface{}, 0, len(parts))
			for _, p := range parts {
				p = strings.TrimSpace(p)
				// Strip quotes
				if (strings.HasPrefix(p, "\"") && strings.HasSuffix(p, "\"")) ||
					(strings.HasPrefix(p, "'") && strings.HasSuffix(p, "'")) {
					p = p[1 : len(p)-1]
				}
				if p != "" {
					arr = append(arr, p)
				}
			}
			fm[key] = arr
			continue
		}

		// Handle booleans
		switch strings.ToLower(value) {
		case "true":
			fm[key] = true
			continue
		case "false":
			fm[key] = false
			continue
		}

		// Handle integers
		if n, err := strconv.Atoi(value); err == nil {
			fm[key] = n
			continue
		}

		fm[key] = value
	}

	return fm, body
}

func isValidAgentColor(c string) bool {
	for _, valid := range AgentColors {
		if c == valid {
			return true
		}
	}
	return false
}

// FindAgent looks up an agent by type from a list.
func FindAgent(agents []AgentDefinition, agentType string) *AgentDefinition {
	for i := range agents {
		if agents[i].AgentType == agentType {
			return &agents[i]
		}
	}
	return nil
}

// AgentDefinitionsResult holds the result of loading all agents.
// Source: loadAgentsDir.ts:186-191
type AgentDefinitionsResult struct {
	ActiveAgents []AgentDefinition
	AllAgents    []AgentDefinition
	FailedFiles  []FailedAgentFile
}

// FailedAgentFile records a file that failed to parse.
type FailedAgentFile struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

// LoadAllAgents loads agents from all sources and returns active + all agents.
func LoadAllAgents(cwd string) AgentDefinitionsResult {
	allAgents := LoadAgents(cwd)
	activeAgents := GetActiveAgents(allAgents)
	return AgentDefinitionsResult{
		ActiveAgents: activeAgents,
		AllAgents:    allAgents,
	}
}

// IsBuiltInAgent checks if an agent is a built-in agent.
// Source: loadAgentsDir.ts:168-172
func IsBuiltInAgent(agent AgentDefinition) bool {
	return agent.Source == AgentSourceBuiltIn
}

// IsCustomAgent checks if an agent is from user/project/policy settings.
// Source: loadAgentsDir.ts:174-178
func IsCustomAgent(agent AgentDefinition) bool {
	return agent.Source != AgentSourceBuiltIn && agent.Source != AgentSourcePlugin
}

// Skill frontmatter field constants (shared with skill parser).
// Source: utils/frontmatterParser.ts
const (
	FrontmatterName          = "name"
	FrontmatterDescription   = "description"
	FrontmatterAllowedTools  = "allowed-tools"
	FrontmatterModel         = "model"
	FrontmatterEffort        = "effort"
	FrontmatterContext       = "context"
	FrontmatterAgent         = "agent"
	FrontmatterUserInvocable = "user-invocable"
	FrontmatterArgumentHint  = "argument-hint"
	FrontmatterWhenToUse     = "when_to_use"
	FrontmatterPaths         = "paths"
)

// BuiltInSkillName constants for bundled skills.
// Source: skills/bundled/index.ts
const (
	SkillUpdateConfig     = "update-config"
	SkillKeybindingsHelp  = "keybindings-help"
	SkillVerify           = "verify"
	SkillDebug            = "debug"
	SkillRemember         = "remember"
	SkillSimplify         = "simplify"
	SkillBatch            = "batch"
	SkillStuck            = "stuck"
	SkillLoop             = "loop"
	SkillSkillify         = "skillify"
)

// AllBuiltInSkillNames returns the names of all built-in skills.
// Source: skills/bundled/index.ts
var AllBuiltInSkillNames = []string{
	SkillUpdateConfig, SkillKeybindingsHelp, SkillVerify, SkillDebug,
	SkillRemember, SkillSimplify, SkillBatch, SkillStuck,
}

// BuiltInAgentTypes returns the type names of all built-in agents.
func BuiltInAgentTypes() []string {
	return []string{
		AgentTypeGeneralPurpose,
		AgentTypeExplore,
		AgentTypePlan,
		AgentTypeClaudeCodeGuide,
		AgentTypeStatuslineSetup,
		AgentTypeVerification,
	}
}

// FmtAgentDescription returns a short description for use in tool parameters.
func FmtAgentDescription(agent AgentDefinition) string {
	return fmt.Sprintf("%q: %s", agent.AgentType, agent.WhenToUse)
}

// ---------------------------------------------------------------------------
// T503: Agent support — display, override resolution, memory, color management
// Source: tools/AgentTool/agentDisplay.ts, agentMemory.ts
// ---------------------------------------------------------------------------

// AgentSourceGroup describes a display group for agents.
// Source: agentDisplay.ts:24-32
type AgentSourceGroup struct {
	Label  string
	Source AgentSource
}

// AgentSourceGroups is the ordered list for display (consistent CLI + TUI ordering).
var AgentSourceGroups = []AgentSourceGroup{
	{Label: "User agents", Source: AgentSourceUser},
	{Label: "Project agents", Source: AgentSourceProject},
	{Label: "Local agents", Source: AgentSourceFlag},
	{Label: "Managed agents", Source: AgentSourcePolicy},
	{Label: "Plugin agents", Source: AgentSourcePlugin},
	{Label: "Built-in agents", Source: AgentSourceBuiltIn},
}

// ResolvedAgent is an agent annotated with override information.
type ResolvedAgent struct {
	AgentDefinition
	OverriddenBy AgentSource // non-empty if this agent is shadowed
}

// ResolveAgentOverrides annotates agents with override info. An agent is
// "overridden" when another agent with the same type from a higher-priority
// source takes precedence.
// Source: agentDisplay.ts:46-80
func ResolveAgentOverrides(allAgents, activeAgents []AgentDefinition) []ResolvedAgent {
	activeMap := make(map[string]AgentDefinition)
	for _, a := range activeAgents {
		activeMap[a.AgentType] = a
	}

	seen := make(map[string]bool) // "agentType:source"
	var resolved []ResolvedAgent
	for _, agent := range allAgents {
		key := agent.AgentType + ":" + string(agent.Source)
		if seen[key] {
			continue // deduplicate worktree duplicates
		}
		seen[key] = true

		ra := ResolvedAgent{AgentDefinition: agent}
		if active, ok := activeMap[agent.AgentType]; ok && active.Source != agent.Source {
			ra.OverriddenBy = active.Source
		}
		resolved = append(resolved, ra)
	}
	return resolved
}

// GetAgentMemoryDir returns the memory directory for an agent's persistent memory.
// Source: agentMemory.ts:29-67
func GetAgentMemoryDir(agentType string, scope AgentMemoryScope, cwd string) string {
	safeType := strings.ReplaceAll(agentType, ":", "-")

	switch scope {
	case AgentMemoryUser:
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".claude", "agent-memory", safeType)
	case AgentMemoryProject:
		return filepath.Join(cwd, ".claude", "agent-memory", safeType)
	case AgentMemoryLocal:
		return filepath.Join(cwd, ".claude", "agent-memory-local", safeType)
	default:
		return filepath.Join(cwd, ".claude", "agent-memory", safeType)
	}
}

// AgentColorManager assigns consistent colors to agents within a session.
// Source: tools/AgentTool/agentDisplay.ts (color assignment in TS is in bootstrap/state)
type AgentColorManager struct {
	assigned map[string]string
	nextIdx  int
}

// NewAgentColorManager creates a new color manager.
func NewAgentColorManager() *AgentColorManager {
	return &AgentColorManager{assigned: make(map[string]string)}
}

// AssignColor returns a consistent color for the given agent type.
// Same agent always gets the same color within a session.
func (m *AgentColorManager) AssignColor(agentType string) string {
	if c, ok := m.assigned[agentType]; ok {
		return c
	}
	color := AgentColors[m.nextIdx%len(AgentColors)]
	m.assigned[agentType] = color
	m.nextIdx++
	return color
}

// GetColor returns the assigned color, or empty if not yet assigned.
func (m *AgentColorManager) GetColor(agentType string) string {
	return m.assigned[agentType]
}
