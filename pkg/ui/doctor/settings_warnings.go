package doctor

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// SettingsError describes a single settings validation error.
// Source: Doctor.tsx — settings errors from useSettingsErrors
type SettingsError struct {
	Path    string // e.g. "model" or "permissions.allow"
	Message string
}

// KeybindingWarning describes a keybinding parse warning.
// Source: Doctor.tsx — KeybindingWarnings component
type KeybindingWarning struct {
	Key     string // e.g. "ctrl+shift+x"
	Message string // e.g. "unrecognized modifier"
}

// MCPParsingWarning describes an MCP config parse warning.
// Source: Doctor.tsx — McpParsingWarnings component
type MCPParsingWarning struct {
	Server  string // e.g. "my-server"
	Message string // e.g. "invalid command field"
}

// RenderSettingsErrors renders the settings validation errors section.
// Source: Doctor.tsx — ValidationErrorsList component
func RenderSettingsErrors(errors []SettingsError) string {
	if len(errors) == 0 {
		return ""
	}

	t := theme.Current()
	bold := t.TextPrimary().Bold(true)
	errStyle := t.TextError()

	var lines []string
	lines = append(lines, bold.Render("Settings Errors"))
	for _, e := range errors {
		if e.Path != "" {
			lines = append(lines, fmt.Sprintf("└ %s", errStyle.Render(e.Path+": "+e.Message)))
		} else {
			lines = append(lines, fmt.Sprintf("└ %s", errStyle.Render(e.Message)))
		}
	}
	return strings.Join(lines, "\n")
}

// RenderKeybindingWarnings renders the keybinding parse warnings section.
// Source: Doctor.tsx — KeybindingWarnings component
func RenderKeybindingWarnings(warnings []KeybindingWarning) string {
	if len(warnings) == 0 {
		return ""
	}

	t := theme.Current()
	bold := t.TextPrimary().Bold(true)
	warn := t.TextWarning()
	dim := lipgloss.NewStyle().Faint(true)

	var lines []string
	lines = append(lines, bold.Render("Keybinding Warnings"))
	for _, w := range warnings {
		lines = append(lines, fmt.Sprintf("└ %s", warn.Render(w.Key+": "+w.Message)))
		lines = append(lines, dim.Render(fmt.Sprintf("  └ Key: %s", w.Key)))
	}
	return strings.Join(lines, "\n")
}

// RenderMCPWarnings renders the MCP config parse warnings section.
// Source: Doctor.tsx — McpParsingWarnings component
func RenderMCPWarnings(warnings []MCPParsingWarning) string {
	if len(warnings) == 0 {
		return ""
	}

	t := theme.Current()
	bold := t.TextPrimary().Bold(true)
	warn := t.TextWarning()
	dim := lipgloss.NewStyle().Faint(true)

	var lines []string
	lines = append(lines, bold.Render("MCP Config Warnings"))
	for _, w := range warnings {
		lines = append(lines, fmt.Sprintf("└ %s", warn.Render(w.Server+": "+w.Message)))
		lines = append(lines, dim.Render(fmt.Sprintf("  └ Server: %s", w.Server)))
	}
	return strings.Join(lines, "\n")
}
