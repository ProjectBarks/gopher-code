package components

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// SessionInfo holds metadata about a saved session.
type SessionInfo struct {
	ID        string
	Name      string
	Model     string
	MessageCount int
	CreatedAt string
}

// SessionSelectedMsg is sent when a session is selected.
type SessionSelectedMsg struct {
	Session SessionInfo
}

// SessionPicker provides fuzzy search and selection of prior sessions.
type SessionPicker struct {
	sessions    []SessionInfo
	filtered    []SessionInfo
	searchText  string
	selected    int
	theme       theme.Theme
	width       int
	height      int
	focused     bool
}

// NewSessionPicker creates a new session picker.
func NewSessionPicker(t theme.Theme) *SessionPicker {
	return &SessionPicker{
		sessions: make([]SessionInfo, 0),
		filtered: make([]SessionInfo, 0),
		theme:    t,
		width:    80,
		height:   20,
	}
}

// SetSessions sets the available sessions.
func (sp *SessionPicker) SetSessions(sessions []SessionInfo) {
	sp.sessions = sessions
	sp.filterSessions()
}

// Init initializes the component.
func (sp *SessionPicker) Init() tea.Cmd { return nil }

// Update handles key presses.
func (sp *SessionPicker) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.Code {
		case tea.KeyUp:
			if sp.selected > 0 {
				sp.selected--
			}
		case tea.KeyDown:
			if sp.selected < len(sp.filtered)-1 {
				sp.selected++
			}
		case tea.KeyEnter:
			if len(sp.filtered) > 0 && sp.selected < len(sp.filtered) {
				sess := sp.filtered[sp.selected]
				return sp, func() tea.Msg {
					return SessionSelectedMsg{Session: sess}
				}
			}
		case tea.KeyBackspace:
			if len(sp.searchText) > 0 {
				sp.searchText = sp.searchText[:len(sp.searchText)-1]
				sp.filterSessions()
			}
		default:
			if msg.Text != "" {
				sp.searchText += msg.Text
				sp.filterSessions()
				sp.selected = 0
			}
		}
	}
	return sp, nil
}

// View renders the session picker.
func (sp *SessionPicker) View() tea.View {
	cs := sp.theme.Colors()
	var output []string

	// Title
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.TextPrimary)).Bold(true)
	output = append(output, titleStyle.Render("📂 Select Session"))

	// Search bar
	searchStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.Accent))
	searchDisplay := sp.searchText
	if searchDisplay == "" {
		searchDisplay = "type to search..."
	}
	output = append(output, searchStyle.Render("> "+searchDisplay))
	output = append(output, "")

	// Session list
	if len(sp.filtered) == 0 {
		dimStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(cs.TextSecondary))
		output = append(output, dimStyle.Render("No sessions found"))
	} else {
		maxShow := sp.height - 4
		if maxShow < 3 {
			maxShow = 3
		}
		for i, sess := range sp.filtered {
			if i >= maxShow {
				break
			}
			line := sp.renderSession(sess, i == sp.selected, cs)
			output = append(output, line)
		}
	}

	return tea.NewView(strings.Join(output, "\n"))
}

func (sp *SessionPicker) renderSession(sess SessionInfo, selected bool, cs theme.ColorScheme) string {
	nameStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.TextPrimary)).Bold(true)
	metaStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.TextSecondary))

	name := sess.Name
	if name == "" {
		name = sess.ID[:8]
	}

	line := fmt.Sprintf("%s  %s  %d msgs",
		nameStyle.Render(name),
		metaStyle.Render(sess.Model),
		sess.MessageCount,
	)

	if selected {
		selStyle := lipgloss.NewStyle().
			Background(lipgloss.Color(cs.Selection))
		line = selStyle.Render("▸ " + line)
	} else {
		line = "  " + line
	}

	return line
}

func (sp *SessionPicker) filterSessions() {
	if sp.searchText == "" {
		sp.filtered = make([]SessionInfo, len(sp.sessions))
		copy(sp.filtered, sp.sessions)
		return
	}

	search := strings.ToLower(sp.searchText)
	sp.filtered = make([]SessionInfo, 0)
	for _, sess := range sp.sessions {
		name := strings.ToLower(sess.Name + " " + sess.ID + " " + sess.Model)
		if strings.Contains(name, search) || FuzzyMatch(search, name) {
			sp.filtered = append(sp.filtered, sess)
		}
	}
}

// SearchText returns the current search text.
func (sp *SessionPicker) SearchText() string { return sp.searchText }

// Selected returns the currently selected session, or nil.
func (sp *SessionPicker) Selected() *SessionInfo {
	if sp.selected < len(sp.filtered) {
		return &sp.filtered[sp.selected]
	}
	return nil
}

// SetSize sets the dimensions.
func (sp *SessionPicker) SetSize(width, height int) {
	sp.width = width
	sp.height = height
}

func (sp *SessionPicker) Focus()        { sp.focused = true }
func (sp *SessionPicker) Blur()         { sp.focused = false }
func (sp *SessionPicker) Focused() bool { return sp.focused }

var _ tea.Model = (*SessionPicker)(nil)
