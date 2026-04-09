package permissions

// Source: components/permissions/FileEditPermissionRequest/FileEditPermissionRequest.tsx,
//         FileWritePermissionRequest/FileWritePermissionRequest.tsx,
//         FilesystemPermissionRequest/FilesystemPermissionRequest.tsx,
//         FilePermissionDialog/FilePermissionDialog.tsx
//
// Permission prompts for file operations: edit (old→new string), write (new file),
// and filesystem read. Shows file path, diff preview for edits, and allow options.

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// FileEditPermissionModel shows the edit permission with a diff preview.
type FileEditPermissionModel struct {
	ToolUseID  string
	FilePath   string
	OldString  string
	NewString  string
	ReplaceAll bool
	selected   int
}

// NewFileEditPermission creates a file edit permission prompt.
func NewFileEditPermission(toolUseID, filePath, oldString, newString string, replaceAll bool) FileEditPermissionModel {
	return FileEditPermissionModel{
		ToolUseID:  toolUseID,
		FilePath:   filePath,
		OldString:  oldString,
		NewString:  newString,
		ReplaceAll: replaceAll,
	}
}

func (m FileEditPermissionModel) Init() tea.Cmd { return nil }

func (m FileEditPermissionModel) Update(msg tea.Msg) (FileEditPermissionModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.Code {
		case tea.KeyUp, 'k':
			if m.selected > 0 {
				m.selected--
			}
		case tea.KeyDown, 'j':
			if m.selected < 2 {
				m.selected++
			}
		case 'y':
			return m, m.decide(DecisionAllow)
		case 'n':
			return m, m.decide(DecisionDeny)
		case 'a':
			return m, m.decide(DecisionAlwaysAllow)
		case tea.KeyEnter:
			decisions := []Decision{DecisionAllow, DecisionDeny, DecisionAlwaysAllow}
			return m, m.decide(decisions[m.selected])
		case tea.KeyEscape:
			return m, m.decide(DecisionDeny)
		}
	}
	return m, nil
}

func (m FileEditPermissionModel) decide(d Decision) tea.Cmd {
	return func() tea.Msg {
		return PermissionDecisionMsg{
			ToolUseID: m.ToolUseID,
			Decision:  d,
			Request: Request{
				Type:     RequestEdit,
				ToolName: "Edit",
				FilePath: m.FilePath,
			},
		}
	}
}

func (m FileEditPermissionModel) View() string {
	colors := theme.Current().Colors()
	titleStyle := lipgloss.NewStyle().Bold(true)
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Warning))
	pathStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Info))
	addStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.DiffAdded))
	rmStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.DiffRemoved))
	dimStyle := lipgloss.NewStyle().Faint(true)
	selStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colors.Accent))

	var b strings.Builder

	b.WriteString(warnStyle.Render("⚠ Claude wants to edit a file"))
	b.WriteString("\n\n")

	// File path
	b.WriteString(titleStyle.Render("  File: "))
	b.WriteString(pathStyle.Render(filepath.Base(m.FilePath)))
	b.WriteString(dimStyle.Render(" (" + m.FilePath + ")"))
	b.WriteString("\n")

	if m.ReplaceAll {
		b.WriteString(dimStyle.Render("  (replace all occurrences)"))
		b.WriteString("\n")
	}

	// Diff preview
	b.WriteString("\n")
	if m.OldString != "" {
		for _, line := range strings.Split(m.OldString, "\n") {
			b.WriteString("  " + rmStyle.Render("- "+line) + "\n")
		}
	}
	if m.NewString != "" {
		for _, line := range strings.Split(m.NewString, "\n") {
			b.WriteString("  " + addStyle.Render("+ "+line) + "\n")
		}
	}

	b.WriteString("\n")
	renderFileOptions(&b, m.selected, selStyle, dimStyle)
	return b.String()
}

// FileWritePermissionModel shows the write permission with content preview.
type FileWritePermissionModel struct {
	ToolUseID string
	FilePath  string
	Content   string
	IsNew     bool // true if creating a new file
	selected  int
}

// NewFileWritePermission creates a file write permission prompt.
func NewFileWritePermission(toolUseID, filePath, content string, isNew bool) FileWritePermissionModel {
	return FileWritePermissionModel{
		ToolUseID: toolUseID,
		FilePath:  filePath,
		Content:   content,
		IsNew:     isNew,
	}
}

func (m FileWritePermissionModel) Init() tea.Cmd { return nil }

func (m FileWritePermissionModel) Update(msg tea.Msg) (FileWritePermissionModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.Code {
		case tea.KeyUp, 'k':
			if m.selected > 0 {
				m.selected--
			}
		case tea.KeyDown, 'j':
			if m.selected < 2 {
				m.selected++
			}
		case 'y':
			return m, m.decide(DecisionAllow)
		case 'n':
			return m, m.decide(DecisionDeny)
		case 'a':
			return m, m.decide(DecisionAlwaysAllow)
		case tea.KeyEnter:
			decisions := []Decision{DecisionAllow, DecisionDeny, DecisionAlwaysAllow}
			return m, m.decide(decisions[m.selected])
		case tea.KeyEscape:
			return m, m.decide(DecisionDeny)
		}
	}
	return m, nil
}

func (m FileWritePermissionModel) decide(d Decision) tea.Cmd {
	return func() tea.Msg {
		return PermissionDecisionMsg{
			ToolUseID: m.ToolUseID,
			Decision:  d,
			Request: Request{
				Type:     RequestWrite,
				ToolName: "Write",
				FilePath: m.FilePath,
			},
		}
	}
}

func (m FileWritePermissionModel) View() string {
	colors := theme.Current().Colors()
	titleStyle := lipgloss.NewStyle().Bold(true)
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Warning))
	pathStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Info))
	addStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.DiffAdded))
	dimStyle := lipgloss.NewStyle().Faint(true)
	selStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colors.Accent))

	var b strings.Builder

	action := "write to"
	if m.IsNew {
		action = "create"
	}
	b.WriteString(warnStyle.Render(fmt.Sprintf("⚠ Claude wants to %s a file", action)))
	b.WriteString("\n\n")

	b.WriteString(titleStyle.Render("  File: "))
	b.WriteString(pathStyle.Render(filepath.Base(m.FilePath)))
	b.WriteString(dimStyle.Render(" (" + m.FilePath + ")"))
	b.WriteString("\n")

	// Content preview (truncated)
	if m.Content != "" {
		b.WriteString("\n")
		lines := strings.Split(m.Content, "\n")
		maxLines := 10
		for i, line := range lines {
			if i >= maxLines {
				b.WriteString(dimStyle.Render(fmt.Sprintf("  ... (%d more lines)", len(lines)-maxLines)))
				b.WriteString("\n")
				break
			}
			b.WriteString("  " + addStyle.Render("+ "+line) + "\n")
		}
	}

	b.WriteString("\n")
	renderFileOptions(&b, m.selected, selStyle, dimStyle)
	return b.String()
}

// renderFileOptions renders the allow/deny/always-allow options.
func renderFileOptions(b *strings.Builder, selected int, selStyle, dimStyle lipgloss.Style) {
	options := []struct {
		label string
		key   string
	}{
		{"Allow once", "y"},
		{"Deny", "n"},
		{"Always allow for this session", "a"},
	}
	for i, opt := range options {
		cursor := "  "
		style := lipgloss.NewStyle()
		if i == selected {
			cursor = "> "
			style = selStyle
		}
		b.WriteString(fmt.Sprintf("%s%s  %s\n",
			cursor,
			style.Render(opt.label),
			dimStyle.Render("("+opt.key+")")))
	}
}
