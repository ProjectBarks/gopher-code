package components

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// ToolCallState represents the state of a tool call.
type ToolCallState string

const (
	ToolCallPending   ToolCallState = "pending"
	ToolCallComplete  ToolCallState = "complete"
	ToolCallError     ToolCallState = "error"
)

// ToolCallDisplay shows the status of a tool execution.
// It animates a spinner while the tool is running and displays results when complete.
type ToolCallDisplay struct {
	// Identification
	id   string
	name string

	// State tracking
	state       ToolCallState
	spinnerTick int // Counter for spinner animation (0-3)

	// Results
	resultSummary string
	errorMsg      string

	// Styling
	theme theme.Theme
	width int
}

// NewToolCallDisplay creates a new tool call display.
func NewToolCallDisplay(id, name string, t theme.Theme) *ToolCallDisplay {
	return &ToolCallDisplay{
		id:     id,
		name:   name,
		state:  ToolCallPending,
		theme:  t,
		width:  80,
	}
}

// SetPending marks the tool as pending (shows spinner).
func (tcd *ToolCallDisplay) SetPending() {
	tcd.state = ToolCallPending
	tcd.spinnerTick = 0
}

// SetComplete marks the tool as complete with a result summary.
func (tcd *ToolCallDisplay) SetComplete(summary string) {
	tcd.state = ToolCallComplete
	tcd.resultSummary = summary
}

// SetError marks the tool as having an error.
func (tcd *ToolCallDisplay) SetError(err string) {
	tcd.state = ToolCallError
	tcd.errorMsg = err
}

// Update handles spinner animation.
func (tcd *ToolCallDisplay) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case ToolCallDisplayTickMsg:
		if tcd.state == ToolCallPending {
			tcd.spinnerTick = (tcd.spinnerTick + 1) % 4
		}
	}
	return tcd, nil
}

// ToolCallDisplayTickMsg is used for spinner animation.
type ToolCallDisplayTickMsg struct{}

// View renders the tool call display.
func (tcd *ToolCallDisplay) View() tea.View {
	cs := tcd.theme.Colors()

	var content string

	switch tcd.state {
	case ToolCallPending:
		content = tcd.renderPending(cs)
	case ToolCallComplete:
		content = tcd.renderComplete(cs)
	case ToolCallError:
		content = tcd.renderError(cs)
	}

	return tea.NewView(content)
}

// renderPending renders a tool call with a spinning animation.
func (tcd *ToolCallDisplay) renderPending(cs theme.ColorScheme) string {
	spinnerChars := []string{"⠋", "⠙", "⠹", "⠸"}
	spinner := spinnerChars[tcd.spinnerTick%4]

	spinnerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.Spinner))

	toolStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.ToolName)).
		Bold(true)

	statusStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.Warning))

	header := fmt.Sprintf("%s %s [%s]",
		spinnerStyle.Render(spinner),
		toolStyle.Render(tcd.name),
		statusStyle.Render("running"),
	)

	subtext := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.TextSecondary)).
		Render(fmt.Sprintf("Tool ID: %s", tcd.id))

	return header + "\n" + subtext
}

// renderComplete renders a completed tool call with result summary.
func (tcd *ToolCallDisplay) renderComplete(cs theme.ColorScheme) string {
	checkStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.Success))

	toolStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.ToolName)).
		Bold(true)

	statusStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.Success))

	header := fmt.Sprintf("%s %s [%s]",
		checkStyle.Render("✓"),
		toolStyle.Render(tcd.name),
		statusStyle.Render("complete"),
	)

	lines := []string{header}

	// Add result summary if provided
	if tcd.resultSummary != "" {
		resultStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(cs.TextSecondary))

		// Truncate long summaries
		summary := tcd.resultSummary
		if len(summary) > 100 {
			summary = summary[:100] + "..."
		}

		lines = append(lines, resultStyle.Render("Result: "+summary))
	}

	return strings.Join(lines, "\n")
}

// renderError renders a failed tool call with error message.
func (tcd *ToolCallDisplay) renderError(cs theme.ColorScheme) string {
	xStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.Error))

	toolStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.ToolName)).
		Bold(true)

	statusStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.Error))

	header := fmt.Sprintf("%s %s [%s]",
		xStyle.Render("✗"),
		toolStyle.Render(tcd.name),
		statusStyle.Render("error"),
	)

	lines := []string{header}

	// Add error message if provided
	if tcd.errorMsg != "" {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(cs.Error))

		// Truncate long error messages
		errMsg := tcd.errorMsg
		if len(errMsg) > 100 {
			errMsg = errMsg[:100] + "..."
		}

		lines = append(lines, errorStyle.Render("Error: "+errMsg))
	}

	return strings.Join(lines, "\n")
}

// Init initializes the component.
func (tcd *ToolCallDisplay) Init() tea.Cmd {
	return nil
}

// SetSize sets the component size.
func (tcd *ToolCallDisplay) SetSize(width, height int) {
	tcd.width = width
}

// ID returns the tool ID.
func (tcd *ToolCallDisplay) ID() string {
	return tcd.id
}

// State returns the current tool call state.
func (tcd *ToolCallDisplay) State() ToolCallState {
	return tcd.state
}

// Ensure ToolCallDisplay implements tea.Model.
var _ tea.Model = (*ToolCallDisplay)(nil)
