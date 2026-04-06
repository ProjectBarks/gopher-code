package screens

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/projectbarks/gopher-code/pkg/session"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// ResumeDoneMsg is sent when the user dismisses the resume screen without selecting.
type ResumeDoneMsg struct{}

// ResumeSelectMsg is sent when the user selects a session to resume.
type ResumeSelectMsg struct {
	SessionID string
}

// ResumeModel is the Bubbletea model for the /resume session picker.
// Source: ResumeConversation.tsx — selectable list of previous sessions
type ResumeModel struct {
	sessions []session.SessionMetadata
	cursor   int
	width    int
	height   int
	scroll   int // scroll offset for long lists
}

// NewResumeModel creates a new resume screen model.
func NewResumeModel(sessions []session.SessionMetadata) *ResumeModel {
	return &ResumeModel{
		sessions: sessions,
	}
}

// Init implements tea.Model.
func (m *ResumeModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m *ResumeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyPressMsg:
		switch msg.Code {
		case tea.KeyUp, 'k':
			if m.cursor > 0 {
				m.cursor--
				m.ensureVisible()
			}
			return m, nil
		case tea.KeyDown, 'j':
			if m.cursor < len(m.sessions)-1 {
				m.cursor++
				m.ensureVisible()
			}
			return m, nil
		case tea.KeyEnter:
			if len(m.sessions) > 0 && m.cursor < len(m.sessions) {
				sid := m.sessions[m.cursor].ID
				return m, func() tea.Msg { return ResumeSelectMsg{SessionID: sid} }
			}
			return m, nil
		case tea.KeyEscape:
			return m, func() tea.Msg { return ResumeDoneMsg{} }
		case 'c':
			if msg.Mod == tea.ModCtrl {
				return m, func() tea.Msg { return ResumeDoneMsg{} }
			}
		case 'd':
			if msg.Mod == tea.ModCtrl {
				return m, func() tea.Msg { return ResumeDoneMsg{} }
			}
		case 'q':
			return m, func() tea.Msg { return ResumeDoneMsg{} }
		}
	}
	return m, nil
}

// View implements tea.Model.
func (m *ResumeModel) View() tea.View {
	if m.width == 0 {
		return tea.NewView("Loading sessions...")
	}

	t := theme.Current()
	bold := t.TextPrimary().Bold(true)
	dim := lipgloss.NewStyle().Faint(true)

	var lines []string
	lines = append(lines, bold.Render("Resume Conversation"))
	lines = append(lines, "")

	if len(m.sessions) == 0 {
		lines = append(lines, dim.Render("No previous sessions found."))
		lines = append(lines, "")
		lines = append(lines, dim.Render("Press Escape to go back"))
		return tea.NewView(strings.Join(lines, "\n"))
	}

	lines = append(lines, dim.Render(fmt.Sprintf("%d session(s) available", len(m.sessions))))
	lines = append(lines, "")

	// Calculate visible area
	viewHeight := m.height - 6 // header + footer
	if viewHeight < 1 {
		viewHeight = 1
	}

	// Render session rows
	for i, sess := range m.sessions {
		if i < m.scroll {
			continue
		}
		if i-m.scroll >= viewHeight {
			break
		}
		row := m.renderSessionRow(sess, i == m.cursor)
		lines = append(lines, row)
	}

	// Footer
	lines = append(lines, "")
	hint := "↑/↓ navigate  Enter select  Esc cancel"
	lines = append(lines, dim.Render(hint))

	return tea.NewView(strings.Join(lines, "\n"))
}

// renderSessionRow renders a single session entry.
// Source: ResumeConversation.tsx — session row with cwd, preview, age, tokens
func (m *ResumeModel) renderSessionRow(meta session.SessionMetadata, selected bool) string {
	t := theme.Current()
	accent := t.TextAccent()
	dim := lipgloss.NewStyle().Faint(true)

	// Selection indicator
	prefix := "  "
	if selected {
		prefix = accent.Render("> ")
	}

	// Session name or ID prefix
	title := meta.Name
	if title == "" {
		title = shortID(meta.ID)
	}

	// Age display
	age := formatAge(meta.UpdatedAt)

	// CWD (truncated)
	cwd := meta.CWD
	maxCWD := 40
	if len(cwd) > maxCWD {
		cwd = "..." + cwd[len(cwd)-maxCWD+3:]
	}

	// Token count
	tokens := ""
	if meta.TurnCount > 0 {
		tokens = fmt.Sprintf(" (%d turns)", meta.TurnCount)
	}

	// Preview text (first user message)
	preview := ""
	if meta.Preview != "" {
		p := meta.Preview
		if len(p) > 60 {
			p = p[:57] + "..."
		}
		// Replace newlines with spaces for single-line display
		p = strings.ReplaceAll(p, "\n", " ")
		preview = fmt.Sprintf("\n%s  %s", "  ", dim.Italic(true).Render(p))
	}

	// Build row
	if selected {
		titleStr := accent.Bold(true).Render(title)
		return fmt.Sprintf("%s%s  %s  %s%s%s",
			prefix, titleStr, dim.Render(cwd), dim.Render(age), dim.Render(tokens), preview)
	}
	return fmt.Sprintf("%s%s  %s  %s%s%s",
		prefix, title, dim.Render(cwd), dim.Render(age), dim.Render(tokens), preview)
}

// ensureVisible adjusts scroll to keep cursor in view.
func (m *ResumeModel) ensureVisible() {
	viewHeight := m.height - 6
	if viewHeight < 1 {
		viewHeight = 1
	}
	if m.cursor < m.scroll {
		m.scroll = m.cursor
	}
	if m.cursor >= m.scroll+viewHeight {
		m.scroll = m.cursor - viewHeight + 1
	}
}

// shortID returns the first 8 chars of a UUID.
func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// formatAge returns a human-readable relative time string.
// Source: ResumeConversation.tsx — age display like "2 days ago"
func formatAge(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		mins := int(d.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	case d < 24*time.Hour:
		hours := int(d.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	case d < 7*24*time.Hour:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	case d < 30*24*time.Hour:
		weeks := int(d.Hours() / (24 * 7))
		if weeks == 1 {
			return "1 week ago"
		}
		return fmt.Sprintf("%d weeks ago", weeks)
	default:
		months := int(d.Hours() / (24 * 30))
		if months == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	}
}
