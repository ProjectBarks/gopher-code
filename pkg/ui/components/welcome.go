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
	if boxWidth < 20 {
		boxWidth = 20
	}

	// Build the bordered welcome box
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.Primary)).
		Bold(true)
	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.Warning)).
		Bold(true)
	textStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.TextPrimary))
	subtleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.TextSecondary))

	// Left panel content — mascot is multi-line, so split into individual lines
	mascotLines := strings.Split(ws.renderMascot(cs), "\n")
	leftLines := []string{
		"",
		titleStyle.Render("  Welcome!"),
		"",
	}
	leftLines = append(leftLines, mascotLines...)
	leftLines = append(leftLines,
		"",
		subtleStyle.Render(fmt.Sprintf("  %s", ws.model)),
		subtleStyle.Render(fmt.Sprintf("  %s", abbreviateCWD(ws.cwd, 30))),
		"",
	)

	// Right panel content
	// Source: LogoV2 renders Tips, a ──── separator, then Recent activity
	sepLine := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.BorderSubtle)).
		Render(strings.Repeat("─", boxWidth/2-1))
	rightLines := []string{
		"",
		labelStyle.Render("Tips for getting started"),
		textStyle.Render("Run /init to create a CLAUDE.md"),
		textStyle.Render("file with project instructions"),
		sepLine,
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

		// Pad left column to fixed width, add │ separator, then right column
		// Claude renders: │ left-content │ right-content │
		sepStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(cs.BorderSubtle))
		leftPad := lipgloss.NewStyle().Width(leftWidth).Render(left)
		rightPad := lipgloss.NewStyle().Width(rightWidth - 1).Render(right) // -1 for separator
		bodyLines = append(bodyLines, leftPad+sepStyle.Render("│")+rightPad)
	}

	// Build a manually-drawn border with title integrated into the top line.
	// Claude renders: ╭─── Claude Code v2.1.92 ──...╮
	borderStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.BorderSubtle))

	// Top border with integrated title — total width = boxWidth + 2 (for ╭ and ╮)
	titleText := fmt.Sprintf(" Claude Code v%s ", ws.version)
	// Available space between ╭─── and ╮
	availForTitle := boxWidth - 3 // boxWidth minus "───" prefix
	if len([]rune(titleText)) > availForTitle {
		// Truncate title if box too narrow
		titleText = " Claude Code "
	}
	topPadding := boxWidth - 3 - len([]rune(titleText))
	if topPadding < 0 {
		topPadding = 0
	}
	topLine := borderStyle.Render("╭───" + titleText + strings.Repeat("─", topPadding) + "╮")

	// Body lines with │ borders
	var boxLines []string
	boxLines = append(boxLines, topLine)
	for _, bl := range bodyLines {
		boxLines = append(boxLines, borderStyle.Render("│")+bl+borderStyle.Render("│"))
	}

	// Bottom border
	bottomLine := borderStyle.Render("╰" + strings.Repeat("─", boxWidth) + "╯")
	boxLines = append(boxLines, bottomLine)

	return tea.NewView(strings.Join(boxLines, "\n"))
}

// SetSize updates the screen dimensions.
// The box adapts to the terminal width — it can be narrower than
// WelcomeScreenWidth for small terminals, or expand up to terminal width.
func (ws *WelcomeScreen) SetSize(width, height int) {
	// Box content width = terminal width - 2 (for │ borders)
	ws.width = width - 2
	if ws.width < 20 {
		ws.width = 20
	}
	ws.height = height
}

// renderMascot renders Claude's "Clawd" mascot using quadrant block characters.
// Source: components/LogoV2/Clawd.tsx — default pose uses ▗ ▖ for eyes, ▘▘ ▝▝ for mouth.
func (ws *WelcomeScreen) renderMascot(cs theme.ColorScheme) string {
	bodyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.Accent))

	// Clawd face using quadrant block elements (matching Claude Code)
	lines := []string{
		bodyStyle.Render("    ▗ ▗   ▖ ▖"),
		"",
		bodyStyle.Render("      ▘▘ ▝▝"),
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
