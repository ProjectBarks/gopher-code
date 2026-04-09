// Package memory provides the memory file selector UI component.
//
// Source: components/memory/MemoryFileSelector.tsx
//
// In TS this is a React component using CustomSelect with a list of
// discovered CLAUDE.md files. In Go it's a bubbletea model that renders
// a selectable list of memory files with type labels, existence markers,
// and display paths.
package memory

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	pkgmemory "github.com/projectbarks/gopher-code/pkg/memory"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// FileSelectedMsg is sent when the user selects a memory file.
type FileSelectedMsg struct {
	Path string
}

// FileCancelledMsg is sent when the user cancels the selector.
type FileCancelledMsg struct{}

// MemoryItem is a displayable memory file entry.
type MemoryItem struct {
	Path        string
	Type        pkgmemory.MemoryType
	DisplayPath string
	Exists      bool
	ModTime     time.Time
}

// Model is the bubbletea model for the memory file selector.
type Model struct {
	items    []MemoryItem
	cursor   int
	width    int
	cwd      string
}

// New creates a new memory file selector from discovered files.
func New(files []pkgmemory.FileInfo, cwd string, userHome string) Model {
	items := make([]MemoryItem, 0, len(files))
	for _, f := range files {
		items = append(items, MemoryItem{
			Path:        f.Path,
			Type:        f.Type,
			DisplayPath: displayPath(f.Path, cwd, userHome),
			Exists:      f.Content != "",
		})
	}
	return Model{items: items, cwd: cwd}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.Code {
		case tea.KeyUp, 'k':
			if m.cursor > 0 {
				m.cursor--
			}
		case tea.KeyDown, 'j':
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case tea.KeyEnter:
			if m.cursor >= 0 && m.cursor < len(m.items) {
				return m, func() tea.Msg {
					return FileSelectedMsg{Path: m.items[m.cursor].Path}
				}
			}
		case tea.KeyEscape, 'q':
			return m, func() tea.Msg { return FileCancelledMsg{} }
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
	}
	return m, nil
}

// View implements tea.Model.
func (m Model) View() string {
	colors := theme.Current().Colors()
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colors.Primary))
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colors.Accent))
	dimStyle := lipgloss.NewStyle().Faint(true)
	typeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Info))

	var b strings.Builder
	b.WriteString(titleStyle.Render("Memory Files"))
	b.WriteString("\n\n")

	if len(m.items) == 0 {
		b.WriteString(dimStyle.Render("  No memory files found"))
		return b.String()
	}

	for i, item := range m.items {
		cursor := "  "
		style := lipgloss.NewStyle()
		if i == m.cursor {
			cursor = "> "
			style = selectedStyle
		}

		label := formatLabel(item)
		typeBadge := typeStyle.Render(fmt.Sprintf("[%s]", item.Type))

		b.WriteString(cursor)
		b.WriteString(style.Render(label))
		b.WriteString(" ")
		b.WriteString(typeBadge)

		if !item.Exists {
			b.WriteString(dimStyle.Render(" (new)"))
		}

		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  ↑/↓ navigate · Enter select · Esc cancel"))

	return b.String()
}

// SetWidth sets the available width for rendering.
func (m *Model) SetWidth(w int) { m.width = w }

// Items returns the current list of memory items.
func (m *Model) Items() []MemoryItem { return m.items }

// SelectedItem returns the currently highlighted item, or nil.
func (m *Model) SelectedItem() *MemoryItem {
	if m.cursor < 0 || m.cursor >= len(m.items) {
		return nil
	}
	return &m.items[m.cursor]
}

// formatLabel returns the display label for a memory item.
func formatLabel(item MemoryItem) string {
	switch item.Type {
	case pkgmemory.TypeUser:
		if strings.HasSuffix(item.Path, "CLAUDE.md") && !strings.Contains(item.Path, "rules") {
			return "User memory"
		}
	case pkgmemory.TypeProject:
		if strings.HasSuffix(item.Path, "CLAUDE.md") {
			return "Project memory"
		}
	case pkgmemory.TypeLocal:
		return "Local memory"
	case pkgmemory.TypeManaged:
		return "Managed memory"
	}
	return item.DisplayPath
}

// displayPath returns a shortened display path relative to cwd or ~.
func displayPath(path, cwd, home string) string {
	// Try relative to cwd
	if cwd != "" {
		if rel, err := filepath.Rel(cwd, path); err == nil && !strings.HasPrefix(rel, "..") {
			return rel
		}
	}
	// Try ~ shorthand
	if home != "" && strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}
