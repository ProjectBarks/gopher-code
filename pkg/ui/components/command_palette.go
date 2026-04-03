package components

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/projectbarks/gopher-code/pkg/ui/core"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// PaletteCommand represents a command in the palette
type PaletteCommand struct {
	ID          string // Unique identifier
	Title       string // Display name
	Description string // Help text
	Category    string // Command category
	Execute     func() // Callback to execute
}

// CommandPalette is a searchable command picker (Cmd+K)
type CommandPalette struct {
	commands      []PaletteCommand      // All available commands
	filteredIdx   []int          // Indices of commands matching search
	selectedIdx   int            // Currently selected in filtered list
	searchText    string         // Current search text
	visible       bool           // Whether palette is shown
	width         int            // Palette width
	height        int            // Palette height
	scrollOffset  int            // Scroll position
	th            theme.Theme    // Theme for styling
	onClose       func()         // Callback when closed
	onExecute     func(cmdID string) // Callback after execution
}

// NewCommandPalette creates a new command palette
func NewCommandPalette(th theme.Theme) *CommandPalette {
	cp := &CommandPalette{
		commands:    make([]PaletteCommand, 0),
		filteredIdx: make([]int, 0),
		selectedIdx: 0,
		searchText:  "",
		visible:     false,
		width:       60,
		height:      20,
		scrollOffset: 0,
		th:          th,
	}
	return cp
}

// Init initializes the palette
func (cp *CommandPalette) Init() tea.Cmd {
	return nil
}

// Update handles input
func (cp *CommandPalette) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !cp.visible {
		return cp, nil
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return cp.handleKeyPress(msg)
	}

	return cp, nil
}

// handleKeyPress processes keyboard input
func (cp *CommandPalette) handleKeyPress(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "escape", "esc":
		cp.Close()
		return cp, nil

	case "enter":
		if len(cp.filteredIdx) > 0 && cp.selectedIdx < len(cp.filteredIdx) {
			cmdIdx := cp.filteredIdx[cp.selectedIdx]
			cmd := cp.commands[cmdIdx]
			if cmd.Execute != nil {
				cmd.Execute()
			}
			if cp.onExecute != nil {
				cp.onExecute(cmd.ID)
			}
			cp.Close()
		}
		return cp, nil

	case "up":
		if cp.selectedIdx > 0 {
			cp.selectedIdx--
			cp.ensureVisible()
		}

	case "down":
		if cp.selectedIdx < len(cp.filteredIdx)-1 {
			cp.selectedIdx++
			cp.ensureVisible()
		}

	case "home":
		cp.selectedIdx = 0
		cp.scrollOffset = 0

	case "end":
		if len(cp.filteredIdx) > 0 {
			cp.selectedIdx = len(cp.filteredIdx) - 1
			cp.ensureVisible()
		}

	case "backspace":
		if len(cp.searchText) > 0 {
			cp.searchText = cp.searchText[:len(cp.searchText)-1]
			cp.selectedIdx = 0
			cp.scrollOffset = 0
			cp.updateFilter()
		}

	default:
		// Add character to search
		if len(key) == 1 && key[0] >= 32 && key[0] <= 126 {
			cp.searchText += key
			cp.selectedIdx = 0
			cp.scrollOffset = 0
			cp.updateFilter()
		}
	}

	return cp, nil
}

// updateFilter updates the filtered command list based on search text
func (cp *CommandPalette) updateFilter() {
	cp.filteredIdx = make([]int, 0)

	searchLower := strings.ToLower(cp.searchText)

	for i, cmd := range cp.commands {
		// Match against title and description
		titleMatch := FuzzyMatch(searchLower, strings.ToLower(cmd.Title))
		descMatch := FuzzyMatch(searchLower, strings.ToLower(cmd.Description))

		if titleMatch || descMatch || cp.searchText == "" {
			cp.filteredIdx = append(cp.filteredIdx, i)
		}
	}
}

// ensureVisible scrolls to keep selected item in view
func (cp *CommandPalette) ensureVisible() {
	viewHeight := cp.height - 6
	if viewHeight <= 0 {
		return
	}

	if cp.selectedIdx < cp.scrollOffset {
		cp.scrollOffset = cp.selectedIdx
	}

	if cp.selectedIdx >= cp.scrollOffset+viewHeight {
		cp.scrollOffset = cp.selectedIdx - viewHeight + 1
	}
}

// View renders the command palette
func (cp *CommandPalette) View() tea.View {
	if !cp.visible {
		return tea.NewView("")
	}

	cs := cp.th.Colors()

	// Container styling
	containerStyle := lipgloss.NewStyle().
		Width(cp.width).
		Height(cp.height).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(cs.BorderFocused)).
		Padding(1, 1).
		Background(lipgloss.Color(cs.SurfaceOverlay))

	var content strings.Builder

	// Search bar
	searchStyle := lipgloss.NewStyle().
		Width(cp.width - 4).
		Padding(0, 1).
		Foreground(lipgloss.Color(cs.TextPrimary)).
		Background(lipgloss.Color(cs.Surface)).
		Border(lipgloss.RoundedBorder(), false, false, true, false).
		BorderForeground(lipgloss.Color(cs.BorderSubtle))

	searchDisplay := cp.searchText
	if searchDisplay == "" {
		searchDisplay = "Type to search..."
	}

	content.WriteString(searchStyle.Render(searchDisplay))
	content.WriteString("\n\n")

	// Command list
	viewHeight := cp.height - 8

	for i := cp.scrollOffset; i < len(cp.filteredIdx) && i < cp.scrollOffset+viewHeight; i++ {
		cmdIdx := cp.filteredIdx[i]
		cmd := cp.commands[cmdIdx]
		isSelected := i == cp.selectedIdx

		itemStyle := lipgloss.NewStyle().
			Width(cp.width - 4).
			Padding(0, 1)

		if isSelected {
			itemStyle = itemStyle.
				Background(lipgloss.Color(cs.Accent)).
				Foreground(lipgloss.Color(cs.Surface))
		} else {
			itemStyle = itemStyle.
				Foreground(lipgloss.Color(cs.TextSecondary))
		}

		// Format as "Title - Description"
		display := cmd.Title
		if cmd.Description != "" {
			display = cmd.Title + " — " + cmd.Description
		}

		display = truncateField(display, cp.width-4)
		content.WriteString(itemStyle.Render(display))

		if i < len(cp.filteredIdx)-1 && i < cp.scrollOffset+viewHeight-1 {
			content.WriteString("\n")
		}
	}

	// Empty state
	if len(cp.filteredIdx) == 0 && cp.searchText != "" {
		emptyStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(cs.TextMuted)).
			Padding(0, 1)
		content.WriteString(emptyStyle.Render("No commands found"))
	}

	// Footer
	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.TextMuted)).
		Padding(1, 1)

	content.WriteString("\n" + footerStyle.Render("Enter to execute • Esc to close"))

	return tea.NewView(containerStyle.Render(content.String()))
}

// SetSize sets the palette dimensions
func (cp *CommandPalette) SetSize(width, height int) {
	cp.width = width
	cp.height = height
}

// Focus is a no-op (palette manages its own visibility)
func (cp *CommandPalette) Focus() {
	cp.visible = true
	cp.searchText = ""
	cp.selectedIdx = 0
	cp.scrollOffset = 0
	cp.updateFilter()
}

// Blur is a no-op
func (cp *CommandPalette) Blur() {
	cp.visible = false
}

// Focused returns whether palette is visible
func (cp *CommandPalette) Focused() bool {
	return cp.visible
}

// Open shows the command palette
func (cp *CommandPalette) Open() {
	cp.Focus()
}

// Close hides the command palette
func (cp *CommandPalette) Close() {
	cp.Blur()
	if cp.onClose != nil {
		cp.onClose()
	}
}

// IsOpen returns whether the palette is visible
func (cp *CommandPalette) IsOpen() bool {
	return cp.visible
}

// AddCommand adds a command to the palette
func (cp *CommandPalette) AddCommand(id, title, description, category string, execute func()) {
	cmd := PaletteCommand{
		ID:          id,
		Title:       title,
		Description: description,
		Category:    category,
		Execute:     execute,
	}
	cp.commands = append(cp.commands, cmd)
	cp.updateFilter()
}

// RemoveCommand removes a command by ID
func (cp *CommandPalette) RemoveCommand(id string) bool {
	for i, cmd := range cp.commands {
		if cmd.ID == id {
			cp.commands = append(cp.commands[:i], cp.commands[i+1:]...)
			cp.updateFilter()
			if cp.selectedIdx >= len(cp.filteredIdx) && cp.selectedIdx > 0 {
				cp.selectedIdx--
			}
			return true
		}
	}
	return false
}

// GetCommand returns a command by ID
func (cp *CommandPalette) GetCommand(id string) *PaletteCommand {
	for i, cmd := range cp.commands {
		if cmd.ID == id {
			return &cp.commands[i]
		}
	}
	return nil
}

// CommandCount returns the total number of commands
func (cp *CommandPalette) CommandCount() int {
	return len(cp.commands)
}

// FilteredCount returns the number of commands matching current search
func (cp *CommandPalette) FilteredCount() int {
	return len(cp.filteredIdx)
}

// SetOnClose sets the callback when palette closes
func (cp *CommandPalette) SetOnClose(callback func()) {
	cp.onClose = callback
}

// SetOnExecute sets the callback after command execution
func (cp *CommandPalette) SetOnExecute(callback func(cmdID string)) {
	cp.onExecute = callback
}

// GetSelectedCommand returns the currently selected command
func (cp *CommandPalette) GetSelectedCommand() *PaletteCommand {
	if cp.selectedIdx < 0 || cp.selectedIdx >= len(cp.filteredIdx) {
		return nil
	}
	cmdIdx := cp.filteredIdx[cp.selectedIdx]
	if cmdIdx < 0 || cmdIdx >= len(cp.commands) {
		return nil
	}
	return &cp.commands[cmdIdx]
}

// GetSearchText returns the current search text
func (cp *CommandPalette) GetSearchText() string {
	return cp.searchText
}

// SetSearchText sets the search text programmatically
func (cp *CommandPalette) SetSearchText(text string) {
	cp.searchText = text
	cp.selectedIdx = 0
	cp.scrollOffset = 0
	cp.updateFilter()
}

// Ensure CommandPalette implements core.Component
var _ core.Component = (*CommandPalette)(nil)
