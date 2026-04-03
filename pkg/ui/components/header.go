package components

import (
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// HeaderUpdateMsg updates the header display.
type HeaderUpdateMsg struct {
	Model       string
	SessionName string
	CWD         string
}

// Header displays the top bar with model, session, and cwd.
type Header struct {
	modelName   string
	sessionName string
	cwd         string
	theme       theme.Theme
	width       int
	height      int
	focused     bool
}

// NewHeader creates a new header component.
func NewHeader(t theme.Theme) *Header {
	return &Header{
		theme: t,
		width: 80,
		height: 1,
	}
}

// SetModel sets the model name display.
func (h *Header) SetModel(name string) { h.modelName = name }

// SetSessionName sets the session name display.
func (h *Header) SetSessionName(name string) { h.sessionName = name }

// SetCWD sets the current working directory display.
func (h *Header) SetCWD(cwd string) { h.cwd = cwd }

// Init initializes the component.
func (h *Header) Init() tea.Cmd { return nil }

// Update handles header update messages.
func (h *Header) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case HeaderUpdateMsg:
		if msg.Model != "" {
			h.modelName = msg.Model
		}
		if msg.SessionName != "" {
			h.sessionName = msg.SessionName
		}
		if msg.CWD != "" {
			h.cwd = msg.CWD
		}
	case tea.WindowSizeMsg:
		h.SetSize(msg.Width, msg.Height)
	}
	return h, nil
}

// View renders the header bar.
func (h *Header) View() tea.View {
	cs := h.theme.Colors()

	logoStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.Primary)).
		Bold(true)
	modelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.Accent))
	sessionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.TextSecondary))
	cwdStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.TextSecondary))

	var parts []string
	parts = append(parts, logoStyle.Render("🐿 Gopher"))

	if h.modelName != "" {
		parts = append(parts, modelStyle.Render(h.modelName))
	}
	if h.sessionName != "" {
		parts = append(parts, sessionStyle.Render(h.sessionName))
	}
	if h.cwd != "" {
		// Abbreviate home directory and long paths
		cwd := abbreviatePath(h.cwd, h.width/3)
		parts = append(parts, cwdStyle.Render(cwd))
	}

	content := strings.Join(parts, " │ ")

	// Pad to fill width
	if h.width > 0 {
		barStyle := lipgloss.NewStyle().
			Width(h.width)
		content = barStyle.Render(content)
	}

	return tea.NewView(content)
}

// ModelName returns the current model name.
func (h *Header) ModelName() string { return h.modelName }

// SessionName returns the current session name.
func (h *Header) SessionName() string { return h.sessionName }

// CWD returns the current working directory.
func (h *Header) CWD() string { return h.cwd }

// SetSize sets the dimensions.
func (h *Header) SetSize(width, height int) {
	h.width = width
	h.height = height
}

func (h *Header) Focus()        { h.focused = true }
func (h *Header) Blur()         { h.focused = false }
func (h *Header) Focused() bool { return h.focused }

// abbreviatePath shortens a path to fit within maxLen.
func abbreviatePath(path string, maxLen int) string {
	if maxLen <= 0 || len(path) <= maxLen {
		return path
	}
	// Try to show just the last N components
	parts := strings.Split(filepath.ToSlash(path), "/")
	for i := 1; i < len(parts); i++ {
		shortened := "…/" + strings.Join(parts[i:], "/")
		if len(shortened) <= maxLen {
			return shortened
		}
	}
	return truncateField(path, maxLen)
}

var _ tea.Model = (*Header)(nil)
