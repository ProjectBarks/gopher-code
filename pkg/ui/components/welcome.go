package components

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	tea "charm.land/bubbletea/v2"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// WelcomeScreenWidth matches Claude Code's WELCOME_V2_WIDTH.
const WelcomeScreenWidth = 58

// Version is the current gopher version.
const Version = "0.2.0"

// WelcomeScreen renders the initial greeting with bordered box,
// mascot, model info, tips, and recent activity.
type WelcomeScreen struct {
	model   string
	cwd     string
	version string
	theme   theme.Theme
	width   int
	height  int
}

// NewWelcomeScreen creates a welcome screen with session context.
func NewWelcomeScreen(t theme.Theme, model, cwd string) *WelcomeScreen {
	return &WelcomeScreen{
		model:   model,
		cwd:     cwd,
		version: Version,
		theme:   t,
		width:   WelcomeScreenWidth,
		height:  14,
	}
}

// Init implements tea.Model.
func (ws *WelcomeScreen) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (ws *WelcomeScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return ws, nil
}

// View renders the full welcome screen.
func (ws *WelcomeScreen) View() tea.View {
	cs := ws.theme.Colors()
	boxWidth := ws.width
	if boxWidth < 40 {
		boxWidth = 40
	}

	// Build the bordered welcome box
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.Primary)).
		Bold(true)
	dimStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.TextMuted))
	accentStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.Accent))
	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.Warning)).
		Bold(true)
	textStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.TextPrimary))
	subtleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.TextSecondary))

	// Left panel content
	leftLines := []string{
		"",
		titleStyle.Render("  Welcome!"),
		"",
		ws.renderMascot(cs),
		"",
		subtleStyle.Render(fmt.Sprintf("  %s", ws.model)),
		subtleStyle.Render(fmt.Sprintf("  %s", abbreviateCWD(ws.cwd, 30))),
		"",
	}

	// Right panel content
	rightLines := []string{
		"",
		labelStyle.Render("Tips for getting started"),
		textStyle.Render("Run /init to create a CLAUDE.md"),
		textStyle.Render("file with project instructions"),
		"",
		labelStyle.Render("Recent activity"),
		subtleStyle.Render("No recent activity"),
		"",
	}

	// Merge into a two-column layout
	leftWidth := boxWidth / 2
	rightWidth := boxWidth - leftWidth

	var bodyLines []string
	maxLines := len(leftLines)
	if len(rightLines) > maxLines {
		maxLines = len(rightLines)
	}

	for i := 0; i < maxLines; i++ {
		left := ""
		if i < len(leftLines) {
			left = leftLines[i]
		}
		right := ""
		if i < len(rightLines) {
			right = rightLines[i]
		}

		// Pad left column to fixed width
		leftPad := lipgloss.NewStyle().Width(leftWidth).Render(left)
		rightPad := lipgloss.NewStyle().Width(rightWidth).Render(right)
		bodyLines = append(bodyLines, leftPad+rightPad)
	}

	body := strings.Join(bodyLines, "\n")

	// Wrap in a bordered box
	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(cs.BorderSubtle)).
		Width(boxWidth)

	boxContent := border.Render(body)

	// Prepend the title line above the box
	// Product name matches TS: "Claude Code" (not "Gopher")
	// Source: components/LogoV2/CondensedLogo.tsx — shows "Claude Code" branding
	titleLine := accentStyle.Render("── Claude Code ") + dimStyle.Render("v"+ws.version) + accentStyle.Render(" ──")

	return tea.NewView(titleLine + "\n" + boxContent)
}

// SetSize updates the screen dimensions.
func (ws *WelcomeScreen) SetSize(width, height int) {
	ws.width = width
	if ws.width > WelcomeScreenWidth {
		ws.width = WelcomeScreenWidth
	}
	ws.height = height
}

// renderMascot renders a small ASCII gopher using block characters.
func (ws *WelcomeScreen) renderMascot(cs theme.ColorScheme) string {
	bodyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.Accent))
	eyeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.TextPrimary))

	// Simple gopher face using block elements
	lines := []string{
		bodyStyle.Render("       ░░░░░░░"),
		bodyStyle.Render("      ░░") + eyeStyle.Render("█") + bodyStyle.Render("░░") + eyeStyle.Render("█") + bodyStyle.Render("░░"),
		bodyStyle.Render("      ░░░░░░░░░"),
		bodyStyle.Render("       ░░███░░"),
	}
	return strings.Join(lines, "\n")
}

// abbreviateCWD shortens a path for display.
func abbreviateCWD(path string, maxLen int) string {
	if len([]rune(path)) <= maxLen {
		return path
	}
	// Replace home dir prefix
	if strings.HasPrefix(path, "/Users/") {
		parts := strings.SplitN(path, "/", 4)
		if len(parts) >= 4 {
			path = "~/" + parts[3]
		}
	}
	runes := []rune(path)
	if len(runes) <= maxLen {
		return path
	}
	if maxLen <= 1 {
		return "…"
	}
	return "…" + string(runes[len(runes)-maxLen+1:])
}

var _ tea.Model = (*WelcomeScreen)(nil)
