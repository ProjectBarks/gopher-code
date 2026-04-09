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

// GetBuiltInAgents returns the standard built-in agent definitions.
// Source: builtInAgents.ts:22-72
func GetBuiltInAgents() []AgentDefinition {
	agents := []AgentDefinition{
		// Source: built-in/generalPurposeAgent.ts
		{
			AgentType: AgentTypeGeneralPurpose,
			WhenToUse: "General-purpose agent for researching complex questions, searching for code, and executing multi-step tasks. When you are searching for a keyword or file and are not confident that you will find the right match in the first few tries use this agent to perform the search for you.",
			Source:    AgentSourceBuiltIn,
			BaseDir:   "built-in",
			// Tools: nil means all tools
		},
		// Source: built-in/statuslineSetup.ts
		{
			AgentType: AgentTypeStatuslineSetup,
			WhenToUse: "Use this agent to configure the user's Claude Code status line setting.",
			Tools:     []string{"Read", "Edit"},
			Model:     "sonnet",
			Color:     "orange",
			Source:    AgentSourceBuiltIn,
			BaseDir:   "built-in",
		},
		// Source: built-in/exploreAgent.ts
		{
			AgentType:    AgentTypeExplore,
			WhenToUse:    "Fast agent specialized for exploring codebases. Use this when you need to quickly find files by patterns, search code for keywords, or answer questions about the codebase.",
			Model:        "haiku",
			OmitClaudeMd: true,
			Source:       AgentSourceBuiltIn,
			BaseDir:      "built-in",
			// Tools: all except Agent, ExitPlanMode, FileEdit, FileWrite, NotebookEdit
			DisallowedTools: []string{"Agent", "ExitPlanMode", "Edit", "Write", "NotebookEdit"},
		},
		// Source: built-in/planAgent.ts
		{
			AgentType:    AgentTypePlan,
			WhenToUse:    "Software architect agent for designing implementation plans. Use this when you need to plan the implementation strategy for a task.",
			Model:        "inherit",
			OmitClaudeMd: true,
			Source:       AgentSourceBuiltIn,
			BaseDir:      "built-in",
			DisallowedTools: []string{"Agent", "ExitPlanMode", "Edit", "Write", "NotebookEdit"},
		},
		// Source: built-in/claudeCodeGuideAgent.ts
		{
			AgentType: AgentTypeClaudeCodeGuide,
			WhenToUse: "Use this agent when the user asks questions about Claude Code features, hooks, slash commands, MCP servers, settings, IDE integrations, keyboard shortcuts; Claude Agent SDK; or Claude API usage.",
			Model:     "haiku",
			Source:    AgentSourceBuiltIn,
			BaseDir:   "built-in",
			Tools:     []string{"Glob", "Grep", "Read", "WebFetch", "WebSearch"},
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
