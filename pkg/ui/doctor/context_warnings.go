package doctor

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// ContextWarning represents a single context usage warning with details.
// Source: Doctor.tsx — ContextWarnings type: claudeMdWarning, agentWarning, mcpWarning
type ContextWarning struct {
	Message string   // e.g. "CLAUDE.md files use 45% of context"
	Details []string // e.g. individual file paths or server names
}

// ContextWarnings aggregates all context-related warnings for /doctor.
type ContextWarnings struct {
	ClaudeMDWarning        *ContextWarning // large CLAUDE.md files
	AgentWarning           *ContextWarning // agents consuming context
	MCPWarning             *ContextWarning // MCP servers consuming context
	UnreachableRulesWarning *ContextWarning // permission rules that can never match
}

// HasWarnings returns true if any context warnings exist.
func (cw *ContextWarnings) HasWarnings() bool {
	if cw == nil {
		return false
	}
	return cw.ClaudeMDWarning != nil ||
		cw.AgentWarning != nil ||
		cw.MCPWarning != nil ||
		cw.UnreachableRulesWarning != nil
}

// RenderContextWarnings renders the context warnings sections.
// Source: Doctor.tsx — "Context Usage Warnings" and "Unreachable Permission Rules" sections.
func RenderContextWarnings(cw *ContextWarnings) string {
	if cw == nil || !cw.HasWarnings() {
		return ""
	}

	t := theme.Current()
	bold := t.TextPrimary().Bold(true)
	warn := t.TextWarning()
	dim := lipgloss.NewStyle().Faint(true)

	var sections []string

	// Unreachable permission rules section
	if cw.UnreachableRulesWarning != nil {
		var lines []string
		lines = append(lines, bold.Render("Unreachable Permission Rules"))
		lines = append(lines, fmt.Sprintf("└ %s", warn.Render(cw.UnreachableRulesWarning.Message)))
		for _, detail := range cw.UnreachableRulesWarning.Details {
			lines = append(lines, dim.Render(fmt.Sprintf("  └ %s", detail)))
		}
		sections = append(sections, strings.Join(lines, "\n"))
	}

	// Context usage warnings section
	hasUsageWarnings := cw.ClaudeMDWarning != nil || cw.AgentWarning != nil || cw.MCPWarning != nil
	if hasUsageWarnings {
		var lines []string
		lines = append(lines, bold.Render("Context Usage Warnings"))

		renderWarning := func(w *ContextWarning, label string) {
			if w == nil {
				return
			}
			lines = append(lines, fmt.Sprintf("└ %s", warn.Render(w.Message)))
			if len(w.Details) > 0 {
				lines = append(lines, fmt.Sprintf("  └ %s:", label))
				for _, d := range w.Details {
					lines = append(lines, dim.Render(fmt.Sprintf("    └ %s", d)))
				}
			}
		}

		renderWarning(cw.ClaudeMDWarning, "Files")
		renderWarning(cw.AgentWarning, "Top contributors")
		renderWarning(cw.MCPWarning, "MCP servers")

		sections = append(sections, strings.Join(lines, "\n"))
	}

	return strings.Join(sections, "\n\n")
}
