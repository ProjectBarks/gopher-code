package doctor

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// AgentEntry describes an active agent definition.
// Source: Doctor.tsx — AgentInfo.activeAgents[].agentType / source
type AgentEntry struct {
	AgentType string // e.g. "custom", "built-in"
	Source    string // e.g. "user", "project", "built-in", "plugin"
}

// FailedAgentFile records a file that failed to parse.
type FailedAgentFile struct {
	Path  string
	Error string
}

// AgentInfo aggregates agent directory scan results for /doctor.
// Source: Doctor.tsx — AgentInfo type
type AgentInfo struct {
	ActiveAgents    []AgentEntry
	UserAgentsDir   string
	ProjectAgentsDir string
	UserDirExists   bool
	ProjectDirExists bool
	FailedFiles     []FailedAgentFile
}

// RenderAgents renders the agent directory scan section.
// Source: Doctor.tsx — agent info display + "Agent Parse Errors" section
func RenderAgents(info *AgentInfo) string {
	if info == nil {
		return ""
	}

	t := theme.Current()
	bold := t.TextPrimary().Bold(true)
	dim := lipgloss.NewStyle().Faint(true)
	errStyle := t.TextError()

	var sections []string

	// Agent directories
	var dirLines []string
	dirLines = append(dirLines, bold.Render("Agents"))
	dirLines = append(dirLines, fmt.Sprintf("└ Active agents: %d", len(info.ActiveAgents)))

	for _, a := range info.ActiveAgents {
		dirLines = append(dirLines, dim.Render(
			fmt.Sprintf("  └ %s (%s)", a.AgentType, a.Source),
		))
	}

	existsStr := func(exists bool) string {
		if exists {
			return "exists"
		}
		return "not found"
	}
	dirLines = append(dirLines, fmt.Sprintf("└ User agents dir: %s (%s)",
		info.UserAgentsDir, existsStr(info.UserDirExists)))
	dirLines = append(dirLines, fmt.Sprintf("└ Project agents dir: %s (%s)",
		info.ProjectAgentsDir, existsStr(info.ProjectDirExists)))

	sections = append(sections, strings.Join(dirLines, "\n"))

	// Agent parse errors (if any)
	if len(info.FailedFiles) > 0 {
		var errLines []string
		errLines = append(errLines, errStyle.Bold(true).Render("Agent Parse Errors"))
		errLines = append(errLines, errStyle.Render(
			fmt.Sprintf("└ Failed to parse %d agent file(s):", len(info.FailedFiles)),
		))
		for _, f := range info.FailedFiles {
			errLines = append(errLines, dim.Render(
				fmt.Sprintf("  └ %s: %s", f.Path, f.Error),
			))
		}
		sections = append(sections, strings.Join(errLines, "\n"))
	}

	return strings.Join(sections, "\n\n")
}
