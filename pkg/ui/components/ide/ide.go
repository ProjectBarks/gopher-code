// Package ide provides IDE-related UI components.
//
// Source: components/IdeOnboardingDialog.tsx, components/IdeStatusIndicator.tsx
package ide

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"charm.land/lipgloss/v2"

	pkgide "github.com/projectbarks/gopher-code/pkg/ide"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// ---------------------------------------------------------------------------
// IDE display name — Source: utils/ide.ts:toIDEDisplayName
// ---------------------------------------------------------------------------

// DisplayName returns the user-facing name for an IDE type.
func DisplayName(ide pkgide.IdeType) string {
	switch ide {
	case pkgide.IdeVSCode:
		return "VS Code"
	case pkgide.IdeCursor:
		return "Cursor"
	case pkgide.IdeWindsurf:
		return "Windsurf"
	case pkgide.IdeJetBrains:
		return "JetBrains"
	default:
		return "your IDE"
	}
}

// ---------------------------------------------------------------------------
// Onboarding dialog — Source: components/IdeOnboardingDialog.tsx
// ---------------------------------------------------------------------------

// OnboardingContent returns the rendered text for the IDE onboarding dialog.
// In TS this is a React component with Dialog wrapper; in Go it returns
// styled text that the caller wraps in whatever frame they want.
func OnboardingContent(ideType pkgide.IdeType, installedVersion string) string {
	colors := theme.Current().Colors()
	ideName := DisplayName(ideType)
	isJB := pkgide.IsJetBrainsIDE(ideType)

	pluginOrExt := "extension"
	if isJB {
		pluginOrExt = "plugin"
	}

	mentionShortcut := "Ctrl+Alt+K"
	if runtime.GOOS == "darwin" {
		mentionShortcut = "Cmd+Option+K"
	}

	ideColor := lipgloss.Color(colors.Accent)
	suggestionColor := lipgloss.Color(colors.Info)
	addColor := lipgloss.Color(colors.DiffAdded)
	removeColor := lipgloss.Color(colors.DiffRemoved)

	titleStyle := lipgloss.NewStyle().Foreground(ideColor).Bold(true)
	sugStyle := lipgloss.NewStyle().Foreground(suggestionColor)
	dimStyle := lipgloss.NewStyle().Faint(true)

	var b strings.Builder

	// Title line
	b.WriteString(titleStyle.Render("✻ "))
	b.WriteString(fmt.Sprintf("Welcome to Claude Code for %s\n", ideName))
	if installedVersion != "" {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  installed %s v%s", pluginOrExt, installedVersion)))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// Feature bullets
	b.WriteString(fmt.Sprintf("• Claude has context of %s and %s\n",
		sugStyle.Render("⧉ open files"),
		sugStyle.Render("⧉ selected lines"),
	))

	addStyle := lipgloss.NewStyle().Foreground(addColor)
	rmStyle := lipgloss.NewStyle().Foreground(removeColor)
	b.WriteString(fmt.Sprintf("• Review Claude Code's changes %s %s in the comfort of your IDE\n",
		addStyle.Render("+11"),
		rmStyle.Render("-22"),
	))

	b.WriteString(fmt.Sprintf("• Cmd+Esc%s\n", dimStyle.Render(" for Quick Launch")))
	b.WriteString(fmt.Sprintf("• %s%s\n", mentionShortcut, dimStyle.Render(" to reference files or lines in your input")))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("Press Enter to continue"))

	return b.String()
}

// ---------------------------------------------------------------------------
// Status indicator — Source: components/IdeStatusIndicator.tsx
// ---------------------------------------------------------------------------

// Selection describes what the user has selected in their IDE.
type Selection struct {
	FilePath  string
	Text      string
	LineCount int
}

// StatusIndicator returns the IDE selection status text.
// Returns empty string if there's nothing to show.
func StatusIndicator(connected bool, sel *Selection) string {
	if !connected || sel == nil {
		return ""
	}

	colors := theme.Current().Colors()
	ideColor := lipgloss.Color(colors.Accent)
	style := lipgloss.NewStyle().Foreground(ideColor)

	// Prefer selection text over file path
	if sel.Text != "" && sel.LineCount > 0 {
		noun := "lines"
		if sel.LineCount == 1 {
			noun = "line"
		}
		return style.Render(fmt.Sprintf("⧉ %d %s selected", sel.LineCount, noun))
	}

	if sel.FilePath != "" {
		return style.Render(fmt.Sprintf("⧉ In %s", filepath.Base(sel.FilePath)))
	}

	return ""
}
