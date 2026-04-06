// Package handlers implements CLI subcommand handlers.
package handlers

import (
	"fmt"
	"io"
	"strings"

	"github.com/projectbarks/gopher-code/pkg/agents"
)

// AgentsHandler prints the list of configured agents grouped by source.
// Output goes to w. cwd is the project working directory used to locate
// project-local agent definitions.
func AgentsHandler(w io.Writer, cwd string) {
	AgentsHandlerWithDirs(w, agents.DefaultAgentDirs(cwd))
}

// AgentsHandlerWithDirs is like AgentsHandler but accepts explicit directory
// mappings. Useful for testing without touching the real filesystem.
func AgentsHandlerWithDirs(w io.Writer, dirs map[agents.Source]string) {
	all := agents.LoadAgents(dirs)
	active := agents.GetActiveAgents(all)
	resolved := agents.ResolveOverrides(all, active)

	var lines []string
	totalActive := 0

	for _, sg := range agents.SourceGroups {
		// Filter to this source group.
		var group []agents.ResolvedAgent
		for _, r := range resolved {
			if r.Source == sg.Source {
				group = append(group, r)
			}
		}
		if len(group) == 0 {
			continue
		}
		agents.SortByName(group)

		lines = append(lines, sg.Label+":")
		for _, a := range group {
			if a.OverriddenBy != "" {
				label := agents.OverrideSourceLabel(a.OverriddenBy)
				lines = append(lines, fmt.Sprintf("  (shadowed by %s) %s", label, agents.FormatAgent(a)))
			} else {
				lines = append(lines, "  "+agents.FormatAgent(a))
				totalActive++
			}
		}
		lines = append(lines, "")
	}

	if len(lines) == 0 {
		fmt.Fprintln(w, "No agents found.")
		return
	}

	fmt.Fprintf(w, "%d active agents\n\n", totalActive)
	fmt.Fprintln(w, strings.TrimRight(strings.Join(lines, "\n"), "\n"))
}
