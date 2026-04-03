package components

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// ErrorSeverity classifies error severity.
type ErrorSeverity int

const (
	SeverityInfo    ErrorSeverity = iota // Blue
	SeverityWarning                      // Yellow
	SeverityError                        // Red
)

// ErrorType classifies the kind of error.
type ErrorType string

const (
	ErrorTypeNetwork    ErrorType = "network"
	ErrorTypeAuth       ErrorType = "auth"
	ErrorTypePermission ErrorType = "permission"
	ErrorTypeTool       ErrorType = "tool"
	ErrorTypeRateLimit  ErrorType = "rate_limit"
	ErrorTypeGeneral    ErrorType = "general"
)

// ErrorInfo holds classified error information.
type ErrorInfo struct {
	Type        ErrorType
	Severity    ErrorSeverity
	Message     string
	Suggestions []string
}

// ErrorDisplay renders classified errors with suggestions.
type ErrorDisplay struct {
	errors  []ErrorInfo
	theme   theme.Theme
	width   int
	height  int
	focused bool
}

// NewErrorDisplay creates a new error display.
func NewErrorDisplay(t theme.Theme) *ErrorDisplay {
	return &ErrorDisplay{
		errors: make([]ErrorInfo, 0),
		theme:  t,
		width:  80,
	}
}

// AddError adds an error to the display.
func (ed *ErrorDisplay) AddError(info ErrorInfo) {
	ed.errors = append(ed.errors, info)
}

// Clear removes all errors.
func (ed *ErrorDisplay) Clear() {
	ed.errors = ed.errors[:0]
}

// HasErrors returns true if there are errors to display.
func (ed *ErrorDisplay) HasErrors() bool {
	return len(ed.errors) > 0
}

// Errors returns all current errors.
func (ed *ErrorDisplay) Errors() []ErrorInfo {
	return ed.errors
}

// Init initializes the component.
func (ed *ErrorDisplay) Init() tea.Cmd { return nil }

// Update handles messages.
func (ed *ErrorDisplay) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return ed, nil
}

// View renders all errors.
func (ed *ErrorDisplay) View() tea.View {
	if len(ed.errors) == 0 {
		return tea.NewView("")
	}

	var output []string
	for _, err := range ed.errors {
		output = append(output, ed.renderError(err))
	}
	return tea.NewView(strings.Join(output, "\n"))
}

func (ed *ErrorDisplay) renderError(info ErrorInfo) string {
	cs := ed.theme.Colors()

	var icon string
	var headerStyle lipgloss.Style

	switch info.Severity {
	case SeverityError:
		icon = "✗"
		headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(cs.Error)).Bold(true)
	case SeverityWarning:
		icon = "⚠"
		headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(cs.Warning)).Bold(true)
	default:
		icon = "ℹ"
		headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(cs.Info)).Bold(true)
	}

	lines := []string{
		headerStyle.Render(fmt.Sprintf("%s [%s] %s", icon, info.Type, info.Message)),
	}

	// Add suggestions
	if len(info.Suggestions) > 0 {
		sugStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(cs.TextSecondary))
		for _, sug := range info.Suggestions {
			lines = append(lines, sugStyle.Render("  → "+sug))
		}
	}

	return strings.Join(lines, "\n")
}

// SetSize sets the dimensions.
func (ed *ErrorDisplay) SetSize(width, height int) {
	ed.width = width
	ed.height = height
}

func (ed *ErrorDisplay) Focus()        { ed.focused = true }
func (ed *ErrorDisplay) Blur()         { ed.focused = false }
func (ed *ErrorDisplay) Focused() bool { return ed.focused }

// ClassifyError creates an ErrorInfo from a raw error message.
func ClassifyError(msg string) ErrorInfo {
	lower := strings.ToLower(msg)

	switch {
	case strings.Contains(lower, "rate limit") || strings.Contains(lower, "429"):
		return ErrorInfo{
			Type:     ErrorTypeRateLimit,
			Severity: SeverityWarning,
			Message:  msg,
			Suggestions: []string{
				"Wait a moment and retry",
				"Consider using a different model",
			},
		}
	case strings.Contains(lower, "auth") || strings.Contains(lower, "api key") || strings.Contains(lower, "401"):
		return ErrorInfo{
			Type:     ErrorTypeAuth,
			Severity: SeverityError,
			Message:  msg,
			Suggestions: []string{
				"Check your ANTHROPIC_API_KEY environment variable",
				"Ensure your API key is valid and not expired",
			},
		}
	case strings.Contains(lower, "permission") || strings.Contains(lower, "denied"):
		return ErrorInfo{
			Type:     ErrorTypePermission,
			Severity: SeverityWarning,
			Message:  msg,
			Suggestions: []string{
				"Grant permission when prompted",
				"Use --dangerously-skip-permissions for unattended use",
			},
		}
	case strings.Contains(lower, "network") || strings.Contains(lower, "connection") || strings.Contains(lower, "timeout"):
		return ErrorInfo{
			Type:     ErrorTypeNetwork,
			Severity: SeverityError,
			Message:  msg,
			Suggestions: []string{
				"Check your internet connection",
				"Verify the API endpoint is reachable",
			},
		}
	default:
		return ErrorInfo{
			Type:     ErrorTypeGeneral,
			Severity: SeverityError,
			Message:  msg,
		}
	}
}

var _ tea.Model = (*ErrorDisplay)(nil)
