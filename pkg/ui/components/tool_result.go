package components

import (
	"encoding/json"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// ToolResultType represents the type of tool result.
type ToolResultType string

const (
	ToolResultText  ToolResultType = "text"
	ToolResultJSON  ToolResultType = "json"
	ToolResultError ToolResultType = "error"
)

// ToolResultDisplay shows the output of a tool execution.
type ToolResultDisplay struct {
	// Identification
	toolUseID string
	toolName  string

	// Content
	content    string
	resultType ToolResultType
	isError    bool

	// Styling
	theme theme.Theme
	width int
}

// NewToolResultDisplay creates a new tool result display.
func NewToolResultDisplay(toolUseID, toolName string, t theme.Theme) *ToolResultDisplay {
	return &ToolResultDisplay{
		toolUseID:  toolUseID,
		toolName:   toolName,
		theme:      t,
		width:      80,
		resultType: ToolResultText,
	}
}

// SetContent sets the result content and attempts to detect the type.
func (trd *ToolResultDisplay) SetContent(content string, isError bool) {
	trd.content = content
	trd.isError = isError

	if isError {
		trd.resultType = ToolResultError
	} else {
		trd.resultType = trd.detectType(content)
	}
}

// detectType tries to detect if content is JSON or plain text.
func (trd *ToolResultDisplay) detectType(content string) ToolResultType {
	trimmed := strings.TrimSpace(content)
	if (strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")) ||
		(strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]")) {
		// Try to parse as JSON
		var obj interface{}
		if err := json.Unmarshal([]byte(trimmed), &obj); err == nil {
			return ToolResultJSON
		}
	}
	return ToolResultText
}

// Update handles messages.
func (trd *ToolResultDisplay) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return trd, nil
}

// View renders the tool result.
func (trd *ToolResultDisplay) View() tea.View {
	cs := trd.theme.Colors()

	var content string

	switch trd.resultType {
	case ToolResultError:
		content = trd.renderError(cs)
	case ToolResultJSON:
		content = trd.renderJSON(cs)
	default:
		content = trd.renderText(cs)
	}

	return tea.NewView(content)
}

// renderText renders plain text results.
func (trd *ToolResultDisplay) renderText(cs theme.ColorScheme) string {
	textStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.TextPrimary))

	header := trd.renderHeader(cs, "Text Result")

	// Truncate very long content
	content := trd.content
	if len(content) > 500 {
		content = content[:500] + "\n...[truncated]"
	}

	lines := []string{header}
	if content != "" {
		lines = append(lines, textStyle.Render(content))
	}

	return strings.Join(lines, "\n")
}

// renderJSON renders pretty-printed JSON results.
func (trd *ToolResultDisplay) renderJSON(cs theme.ColorScheme) string {
	header := trd.renderHeader(cs, "JSON Result")

	// Parse and pretty-print JSON
	var obj interface{}
	err := json.Unmarshal([]byte(trd.content), &obj)
	if err != nil {
		// Fallback: show as text if parsing fails
		return trd.renderText(cs)
	}

	prettyJSON, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return trd.renderText(cs)
	}

	// Format JSON with color codes
	jsonStr := string(prettyJSON)
	jsonStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.Info))

	// Truncate if too long
	if len(jsonStr) > 500 {
		jsonStr = jsonStr[:500] + "\n...[truncated]"
	}

	lines := []string{header}
	lines = append(lines, jsonStyle.Render(jsonStr))

	return strings.Join(lines, "\n")
}

// renderError renders error results.
func (trd *ToolResultDisplay) renderError(cs theme.ColorScheme) string {
	header := trd.renderHeader(cs, "Error")

	errorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.Error))

	// Truncate very long error messages
	errMsg := trd.content
	if len(errMsg) > 300 {
		errMsg = errMsg[:300] + "\n...[truncated]"
	}

	lines := []string{header}
	if errMsg != "" {
		lines = append(lines, errorStyle.Render(errMsg))
	}

	return strings.Join(lines, "\n")
}

// renderHeader renders the result header with tool info.
func (trd *ToolResultDisplay) renderHeader(cs theme.ColorScheme, resultType string) string {
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.TextSecondary)).
		Bold(true)

	toolStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.ToolName))

	return headerStyle.Render(fmt.Sprintf("📋 %s [%s]", resultType, toolStyle.Render(trd.toolName)))
}

// Init initializes the component.
func (trd *ToolResultDisplay) Init() tea.Cmd {
	return nil
}

// SetSize sets the component size.
func (trd *ToolResultDisplay) SetSize(width, height int) {
	trd.width = width
}

// Content returns the result content.
func (trd *ToolResultDisplay) Content() string {
	return trd.content
}

// Type returns the result type.
func (trd *ToolResultDisplay) Type() ToolResultType {
	return trd.resultType
}

// IsError returns whether this is an error result.
func (trd *ToolResultDisplay) IsError() bool {
	return trd.isError
}

// Ensure ToolResultDisplay implements tea.Model.
var _ tea.Model = (*ToolResultDisplay)(nil)
