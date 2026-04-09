// Package teams provides the teams/teammates UI components.
//
// Source: components/teams/TeamsDialog.tsx, utils/teamDiscovery.ts
//
// Shows a list of teammates in the current team with their status,
// agent type, and permission mode. Supports drill-down to teammate
// details. Used by the /teams command.
package teams

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// TeammateStatus describes a teammate's current state.
type TeammateStatus string

const (
	StatusRunning TeammateStatus = "running"
	StatusIdle    TeammateStatus = "idle"
	StatusUnknown TeammateStatus = "unknown"
)

// Teammate describes a single teammate in the swarm.
type Teammate struct {
	Name      string
	AgentID   string
	AgentType string
	Model     string
	Status    TeammateStatus
	Color     string
	Mode      string // permission mode
	CWD       string
	IsHidden  bool
	IdleSince time.Time
}

// TeamSummary describes a team overview.
type TeamSummary struct {
	Name         string
	MemberCount  int
	RunningCount int
	IdleCount    int
}

// DoneMsg is sent when the dialog is closed.
type DoneMsg struct{}

// ViewTeammateMsg is sent when a teammate is selected for detail view.
type ViewTeammateMsg struct {
	Name string
}

// viewLevel describes the current drill-down level.
type viewLevel int

const (
	levelList   viewLevel = iota
	levelDetail
)

// Model is the teams dialog bubbletea model.
type Model struct {
	level      viewLevel
	teammates  []Teammate
	teamName   string
	cursor     int
	selected   *Teammate // for detail view
}

// New creates a teams dialog with the given teammates.
func New(teamName string, teammates []Teammate) Model {
	return Model{
		level:     levelList,
		teammates: teammates,
		teamName:  teamName,
	}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch m.level {
		case levelList:
			switch msg.Code {
			case tea.KeyUp, 'k':
				if m.cursor > 0 {
					m.cursor--
				}
			case tea.KeyDown, 'j':
				if m.cursor < len(m.teammates)-1 {
					m.cursor++
				}
			case tea.KeyEnter:
				if m.cursor < len(m.teammates) {
					tm := m.teammates[m.cursor]
					m.selected = &tm
					m.level = levelDetail
				}
			case tea.KeyEscape, 'q':
				return m, func() tea.Msg { return DoneMsg{} }
			}
		case levelDetail:
			switch msg.Code {
			case tea.KeyEscape, 'q':
				m.level = levelList
				m.selected = nil
			}
		}
	}
	return m, nil
}

func (m Model) View() string {
	colors := theme.Current().Colors()
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colors.Primary))
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colors.Accent))
	dimStyle := lipgloss.NewStyle().Faint(true)

	switch m.level {
	case levelDetail:
		return m.viewDetail(titleStyle, dimStyle)
	default:
		return m.viewList(titleStyle, selectedStyle, dimStyle)
	}
}

func (m Model) viewList(titleStyle, selectedStyle, dimStyle lipgloss.Style) string {
	colors := theme.Current().Colors()
	var b strings.Builder

	title := "Teammates"
	if m.teamName != "" {
		title = fmt.Sprintf("Team: %s", m.teamName)
	}
	b.WriteString(titleStyle.Render(title))

	running := 0
	idle := 0
	for _, tm := range m.teammates {
		if tm.Status == StatusRunning {
			running++
		} else if tm.Status == StatusIdle {
			idle++
		}
	}
	b.WriteString(dimStyle.Render(fmt.Sprintf(" (%d members, %d running, %d idle)", len(m.teammates), running, idle)))
	b.WriteString("\n\n")

	if len(m.teammates) == 0 {
		b.WriteString(dimStyle.Render("  No teammates in this team"))
		b.WriteString("\n")
	} else {
		for i, tm := range m.teammates {
			cursor := "  "
			style := lipgloss.NewStyle()
			if i == m.cursor {
				cursor = "> "
				style = selectedStyle
			}

			// Status icon
			var statusIcon string
			var statusColor string
			switch tm.Status {
			case StatusRunning:
				statusIcon = "●"
				statusColor = colors.Success
			case StatusIdle:
				statusIcon = "○"
				statusColor = colors.Warning
			default:
				statusIcon = "?"
				statusColor = colors.TextMuted
			}
			iconStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(statusColor))

			name := tm.Name
			if tm.Color != "" {
				nameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(tm.Color))
				name = nameStyle.Render(tm.Name)
			}

			b.WriteString(fmt.Sprintf("%s%s %s", cursor, iconStyle.Render(statusIcon), style.Render(name)))

			if tm.AgentType != "" {
				b.WriteString(dimStyle.Render(fmt.Sprintf(" (%s)", tm.AgentType)))
			}
			if tm.IsHidden {
				b.WriteString(dimStyle.Render(" [hidden]"))
			}
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  ↑/↓ navigate · Enter view details · Esc close"))
	return b.String()
}

func (m Model) viewDetail(titleStyle, dimStyle lipgloss.Style) string {
	colors := theme.Current().Colors()
	keyStyle := lipgloss.NewStyle().Bold(true)

	if m.selected == nil {
		return ""
	}
	tm := m.selected

	var b strings.Builder
	name := tm.Name
	if tm.Color != "" {
		nameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(tm.Color))
		name = nameStyle.Render(tm.Name)
	}
	b.WriteString(titleStyle.Render(name))
	b.WriteString("\n\n")

	b.WriteString(fmt.Sprintf("  %s %s\n", keyStyle.Render("Status:"), statusLabel(tm.Status, colors)))
	if tm.AgentType != "" {
		b.WriteString(fmt.Sprintf("  %s %s\n", keyStyle.Render("Agent:"), tm.AgentType))
	}
	if tm.Model != "" {
		b.WriteString(fmt.Sprintf("  %s %s\n", keyStyle.Render("Model:"), tm.Model))
	}
	if tm.Mode != "" {
		b.WriteString(fmt.Sprintf("  %s %s\n", keyStyle.Render("Mode:"), tm.Mode))
	}
	if tm.CWD != "" {
		b.WriteString(fmt.Sprintf("  %s %s\n", keyStyle.Render("CWD:"), tm.CWD))
	}
	if !tm.IdleSince.IsZero() {
		dur := time.Since(tm.IdleSince)
		b.WriteString(fmt.Sprintf("  %s %s ago\n", keyStyle.Render("Idle since:"), formatDuration(dur)))
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  Esc back"))
	return b.String()
}

func statusLabel(status TeammateStatus, colors theme.ColorScheme) string {
	switch status {
	case StatusRunning:
		return lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Success)).Render("running")
	case StatusIdle:
		return lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Warning)).Render("idle")
	default:
		return lipgloss.NewStyle().Faint(true).Render("unknown")
	}
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
}
