// Package agents provides agent definition loading, override resolution,
// and display helpers for the `claude agents` subcommand.
package agents

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Source identifies where an agent definition was loaded from.
type Source string

const (
	SourceUser    Source = "userSettings"
	SourceProject Source = "projectSettings"
	SourceLocal   Source = "localSettings"
	SourceManaged Source = "policySettings"
	SourcePlugin  Source = "plugin"
	SourceFlag    Source = "flagSettings"
	SourceBuiltIn Source = "built-in"
)

// SourceGroup pairs a human-readable label with an agent source for ordered display.
type SourceGroup struct {
	Label  string
	Source Source
}

// SourceGroups is the canonical display ordering (matches TS AGENT_SOURCE_GROUPS).
var SourceGroups = []SourceGroup{
	{Label: "User agents", Source: SourceUser},
	{Label: "Project agents", Source: SourceProject},
	{Label: "Local agents", Source: SourceLocal},
	{Label: "Managed agents", Source: SourceManaged},
	{Label: "Plugin agents", Source: SourcePlugin},
	{Label: "CLI arg agents", Source: SourceFlag},
	{Label: "Built-in agents", Source: SourceBuiltIn},
}

// sourceDisplayNames maps Source to a human-readable label (title case).
var sourceDisplayNames = map[Source]string{
	SourceUser:    "User",
	SourceProject: "Project",
	SourceLocal:   "Local",
	SourceFlag:    "Flag",
	SourceManaged: "Managed",
	SourcePlugin:  "Plugin",
	SourceBuiltIn: "Built-in",
}

// OverrideSourceLabel returns a lowercase label for the given source,
// e.g. "user", "project".
func OverrideSourceLabel(s Source) string {
	if name, ok := sourceDisplayNames[s]; ok {
		return strings.ToLower(name)
	}
	return string(s)
}

// Agent is a minimal agent definition sufficient for listing.
type Agent struct {
	AgentType string // display name / type key
	Model     string // optional model identifier
	Memory    string // optional memory tier: "user", "project", "local"
	Source    Source
}

// ResolvedAgent is an Agent annotated with optional override information.
type ResolvedAgent struct {
	Agent
	OverriddenBy Source // non-empty when shadowed by a higher-priority source
}

// agentJSON is the on-disk JSON schema for agents.json files.
type agentJSON struct {
	Description string `json:"description"`
	Model       string `json:"model,omitempty"`
	Memory      string `json:"memory,omitempty"`
}

// LoadAgents reads agent definition files from the given directories and
// returns all discovered agents. Each directory is associated with a Source.
// Directories that don't exist are silently skipped.
func LoadAgents(dirs map[Source]string) []Agent {
	var all []Agent
	for source, dir := range dirs {
		agents := loadAgentsFromDir(dir, source)
		all = append(all, agents...)
	}
	return all
}

// loadAgentsFromDir reads *.md files and agents.json from a single directory.
func loadAgentsFromDir(dir string, source Source) []Agent {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil // directory doesn't exist or unreadable
	}

	var agents []Agent

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()

		if strings.HasSuffix(name, ".md") {
			agentType := strings.TrimSuffix(name, ".md")
			if agentType == "" {
				continue
			}
			agents = append(agents, Agent{
				AgentType: agentType,
				Source:    source,
			})
		}

		if name == "agents.json" {
			data, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				continue
			}
			var defs map[string]agentJSON
			if json.Unmarshal(data, &defs) != nil {
				continue
			}
			for agentType, def := range defs {
				agents = append(agents, Agent{
					AgentType: agentType,
					Model:     def.Model,
					Memory:    def.Memory,
					Source:    source,
				})
			}
		}
	}

	return agents
}

// GetActiveAgents returns the winning agent for each agentType, applying
// the TS priority order: built-in < plugin < user < project < flag < managed.
func GetActiveAgents(all []Agent) []Agent {
	// Priority order: later overrides earlier.
	priority := map[Source]int{
		SourceBuiltIn: 0,
		SourcePlugin:  1,
		SourceUser:    2,
		SourceProject: 3,
		SourceFlag:    4,
		SourceManaged: 5,
	}

	m := make(map[string]Agent)
	for _, a := range all {
		existing, ok := m[a.AgentType]
		if !ok || priority[a.Source] >= priority[existing.Source] {
			m[a.AgentType] = a
		}
	}

	out := make([]Agent, 0, len(m))
	for _, a := range m {
		out = append(out, a)
	}
	return out
}

// ResolveOverrides annotates all agents with override info and deduplicates
// by (agentType, source).
func ResolveOverrides(all, active []Agent) []ResolvedAgent {
	activeMap := make(map[string]Agent)
	for _, a := range active {
		activeMap[a.AgentType] = a
	}

	seen := make(map[string]bool)
	var resolved []ResolvedAgent

	for _, a := range all {
		key := a.AgentType + ":" + string(a.Source)
		if seen[key] {
			continue
		}
		seen[key] = true

		r := ResolvedAgent{Agent: a}
		if winner, ok := activeMap[a.AgentType]; ok && winner.Source != a.Source {
			r.OverriddenBy = winner.Source
		}
		resolved = append(resolved, r)
	}

	return resolved
}

// SortByName sorts agents alphabetically by AgentType (case-insensitive).
func SortByName(agents []ResolvedAgent) {
	sort.Slice(agents, func(i, j int) bool {
		return strings.ToLower(agents[i].AgentType) < strings.ToLower(agents[j].AgentType)
	})
}

// FormatAgent builds the display string: "agentType · model · memory memory".
func FormatAgent(a ResolvedAgent) string {
	parts := []string{a.AgentType}
	if a.Model != "" {
		parts = append(parts, a.Model)
	}
	if a.Memory != "" {
		parts = append(parts, a.Memory+" memory")
	}
	return strings.Join(parts, " \u00b7 ")
}

// DefaultAgentDirs returns the standard agent directories to scan:
//   - ~/.claude/agents  (user)
//   - <cwd>/.claude/agents (project)
func DefaultAgentDirs(cwd string) map[Source]string {
	dirs := make(map[Source]string)

	home, err := os.UserHomeDir()
	if err == nil {
		dirs[SourceUser] = filepath.Join(home, ".claude", "agents")
	}

	if cwd != "" {
		dirs[SourceProject] = filepath.Join(cwd, ".claude", "agents")
	}

	return dirs
}
