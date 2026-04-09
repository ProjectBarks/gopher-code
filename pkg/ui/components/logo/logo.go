// Package logo provides the welcome screen and logo rendering.
// Source: components/LogoV2/ — WelcomeV2.tsx, LogoV2.tsx, Clawd.tsx
//
// The TS version has animated ASCII art with the Clawd mascot. In Go,
// we render a styled text-based welcome with version and model info.
package logo

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// Version is set at build time.
var Version = "0.1.0-dev"

// WelcomeWidth is the default width of the welcome screen.
const WelcomeWidth = 58

// RenderWelcome renders the full welcome screen shown at startup.
// Source: components/LogoV2/WelcomeV2.tsx
func RenderWelcome(model, cwd string) string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("12"))

	versionStyle := lipgloss.NewStyle().Faint(true)
	dimStyle := lipgloss.NewStyle().Faint(true)

	var sb strings.Builder

	// ASCII art asterisk (simplified from TS's block-character Clawd)
	sb.WriteString(renderAsterisk())
	sb.WriteString("\n")

	// Welcome text
	sb.WriteString(titleStyle.Render("Welcome to Claude Code"))
	sb.WriteString(" ")
	sb.WriteString(versionStyle.Render("v" + Version))
	sb.WriteString("\n\n")

	// Model and CWD info
	if model != "" {
		sb.WriteString(dimStyle.Render("Model: ") + model + "\n")
	}
	if cwd != "" {
		sb.WriteString(dimStyle.Render("CWD:   ") + cwd + "\n")
	}

	sb.WriteString("\n")
	sb.WriteString(dimStyle.Render("Type a message or use /help for commands"))

	return sb.String()
}

// RenderCondensedLogo renders a compact one-line logo for the header.
// Source: components/LogoV2/CondensedLogo.tsx
func RenderCondensedLogo(model string) string {
	nameStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	modelStyle := lipgloss.NewStyle().Faint(true)

	logo := nameStyle.Render("✻ Claude Code")
	if model != "" {
		logo += " " + modelStyle.Render("("+model+")")
	}
	return logo
}

// RenderSpinnerLogo renders the logo with a thinking indicator.
func RenderSpinnerLogo(model, verb string) string {
	nameStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	verbStyle := lipgloss.NewStyle().Faint(true).Italic(true)

	logo := nameStyle.Render("✻ Claude")
	if verb != "" {
		logo += " " + verbStyle.Render(verb+"...")
	}
	return logo
}

// renderAsterisk renders the ✻ asterisk logo art.
func renderAsterisk() string {
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	lines := []string{
		"        ✻",
		"      ✻ ✻ ✻",
		"    ✻ ✻ ✻ ✻ ✻",
		"      ✻ ✻ ✻",
		"        ✻",
	}
	var sb strings.Builder
	for _, line := range lines {
		sb.WriteString(style.Render(line))
		sb.WriteString("\n")
	}
	return sb.String()
}

// RenderEffortSuffix returns " with {level} effort" for display.
func RenderEffortSuffix(effortLevel string) string {
	if effortLevel == "" {
		return ""
	}
	return fmt.Sprintf(" with %s effort", effortLevel)
}
