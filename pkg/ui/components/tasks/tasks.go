// Package tasks provides the task management UI components.
//
// Source: components/tasks/BackgroundTasksDialog.tsx, taskStatusUtils.tsx,
//         BackgroundTask.tsx, BackgroundTaskStatus.tsx
//
// Shows background tasks (agents, shell tasks, dreams) with status, drill-down
// to detail, and task control (stop/view).
package tasks

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// TaskStatus is the lifecycle state of a background task.
type TaskStatus string

const (
	TaskRunning   TaskStatus = "running"
	TaskCompleted TaskStatus = "completed"
	TaskFailed    TaskStatus = "failed"
	TaskKilled    TaskStatus = "killed"
)

// IsTerminal returns true for finished states.
func (s TaskStatus) IsTerminal() bool {
	return s == TaskCompleted || s == TaskFailed || s == TaskKilled
}

// TaskType identifies the kind of background task.
type TaskType string

const (
	TaskTypeShell     TaskType = "shell"
	TaskTypeAgent     TaskType = "agent"
	TaskTypeTeammate  TaskType = "teammate"
	TaskTypeDream     TaskType = "dream"
	TaskTypeRemote    TaskType = "remote"
)

// Task describes a background task for display.
type Task struct {
	ID          string
	Type        TaskType
	Label       string
	Status      TaskStatus
	Activity    string // current activity description
	StartedAt   time.Time
	IsIdle      bool
	HasError    bool
	Color       string // agent color
}

// StatusIcon returns the display icon for a task's state.
func StatusIcon(status TaskStatus, isIdle, hasError bool) string {
	if hasError {
		return "✗"
	}
	switch status {
	case TaskRunning:
		if isIdle {
			return "…"
		}
		return "▶"
	case TaskCompleted:
		return "✓"
	case TaskFailed, TaskKilled:
		return "✗"
	default:
		return "·"
	}
}

// StatusColor returns the semantic color for a task's state.
func StatusColor(status TaskStatus, isIdle, hasError bool, colors theme.ColorScheme) string {
	if hasError {
		return colors.Error
	}
	if isIdle {
		return colors.TextMuted
	}
	switch status {
	case TaskCompleted:
		return colors.Success
	case TaskFailed:
		return colors.Error
	case TaskKilled:
		return colors.Warning
	default:
		return colors.TextSecondary
	}
}

// DoneMsg is sent when the task dialog is closed.
type DoneMsg struct{}

// ViewTaskMsg requests viewing a specific task's detail/output.
type ViewTaskMsg struct {
	TaskID string
}

// StopTaskMsg requests stopping a running task.
type StopTaskMsg struct {
	TaskID string
}

// Model is the task management dialog bubbletea model.
type Model struct {
	tasks    []Task
	cursor   int
	level    int // 0=list, 1=detail
	selected *Task
}

// New creates a task management dialog.
func New(tasks []Task) Model {
	return Model{tasks: tasks}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if m.level == 1 {
			return m.updateDetail(msg)
		}
		return m.updateList(msg)
	}
	return m, nil
}

func (m Model) updateList(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch msg.Code {
	case tea.KeyUp, 'k':
		if m.cursor > 0 {
			m.cursor--
		}
	case tea.KeyDown, 'j':
		if m.cursor < len(m.tasks)-1 {
			m.cursor++
		}
	case tea.KeyEnter:
		if m.cursor < len(m.tasks) {
			t := m.tasks[m.cursor]
			m.selected = &t
			m.level = 1
		}
	case 's':
		if m.cursor < len(m.tasks) && m.tasks[m.cursor].Status == TaskRunning {
			id := m.tasks[m.cursor].ID
			return m, func() tea.Msg { return StopTaskMsg{TaskID: id} }
		}
	case tea.KeyEscape, 'q':
		return m, func() tea.Msg { return DoneMsg{} }
	}
	return m, nil
}

func (m Model) updateDetail(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch msg.Code {
	case 'v':
		if m.selected != nil {
			id := m.selected.ID
			return m, func() tea.Msg { return ViewTaskMsg{TaskID: id} }
		}
	case 's':
		if m.selected != nil && m.selected.Status == TaskRunning {
			id := m.selected.ID
			return m, func() tea.Msg { return StopTaskMsg{TaskID: id} }
		}
	case tea.KeyEscape:
		m.level = 0
		m.selected = nil
	}
	return m, nil
}

func (m Model) View() string {
	if m.level == 1 && m.selected != nil {
		return m.viewDetail()
	}
	return m.viewList()
}

func (m Model) viewList() string {
	colors := theme.Current().Colors()
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colors.Primary))
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colors.Accent))
	dimStyle := lipgloss.NewStyle().Faint(true)

	running := 0
	for _, t := range m.tasks {
		if t.Status == TaskRunning {
			running++
		}
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render("Background Tasks"))
	b.WriteString(dimStyle.Render(fmt.Sprintf(" (%d tasks, %d running)", len(m.tasks), running)))
	b.WriteString("\n\n")

	if len(m.tasks) == 0 {
		b.WriteString(dimStyle.Render("  No background tasks"))
		b.WriteString("\n")
	} else {
		for i, task := range m.tasks {
			cursor := "  "
			style := lipgloss.NewStyle()
			if i == m.cursor {
				cursor = "> "
				style = selectedStyle
			}

			icon := StatusIcon(task.Status, task.IsIdle, task.HasError)
			iconColor := StatusColor(task.Status, task.IsIdle, task.HasError, colors)
			iconStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(iconColor))

			label := task.Label
			if task.Color != "" {
				labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(task.Color))
				label = labelStyle.Render(task.Label)
			}

			b.WriteString(fmt.Sprintf("%s%s %s", cursor, iconStyle.Render(icon), style.Render(label)))

			if task.Activity != "" {
				b.WriteString(dimStyle.Render(" — " + task.Activity))
			}

			elapsed := time.Since(task.StartedAt)
			if elapsed > 0 && !task.StartedAt.IsZero() {
				b.WriteString(dimStyle.Render(fmt.Sprintf(" (%s)", formatDuration(elapsed))))
			}
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  ↑/↓ navigate · Enter details · s stop · Esc close"))
	return b.String()
}

func (m Model) viewDetail() string {
	if m.selected == nil {
		return ""
	}
	t := m.selected
	colors := theme.Current().Colors()
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colors.Primary))
	keyStyle := lipgloss.NewStyle().Bold(true)
	dimStyle := lipgloss.NewStyle().Faint(true)

	var b strings.Builder
	b.WriteString(titleStyle.Render(t.Label))
	b.WriteString("\n\n")

	icon := StatusIcon(t.Status, t.IsIdle, t.HasError)
	iconColor := StatusColor(t.Status, t.IsIdle, t.HasError, colors)
	iconStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(iconColor))
	b.WriteString(fmt.Sprintf("  %s %s %s\n", keyStyle.Render("Status:"), iconStyle.Render(icon), string(t.Status)))
	b.WriteString(fmt.Sprintf("  %s %s\n", keyStyle.Render("Type:"), string(t.Type)))
	b.WriteString(fmt.Sprintf("  %s %s\n", keyStyle.Render("ID:"), t.ID))

	if t.Activity != "" {
		b.WriteString(fmt.Sprintf("  %s %s\n", keyStyle.Render("Activity:"), t.Activity))
	}
	if !t.StartedAt.IsZero() {
		b.WriteString(fmt.Sprintf("  %s %s ago\n", keyStyle.Render("Started:"), formatDuration(time.Since(t.StartedAt))))
	}

	b.WriteString("\n")
	hints := []string{"Esc back"}
	if t.Status == TaskRunning {
		hints = append(hints, "s stop")
	}
	hints = append(hints, "v view output")
	b.WriteString(dimStyle.Render("  " + strings.Join(hints, " · ")))
	return b.String()
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
}
