package components

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/projectbarks/gopher-code/pkg/session"
	"github.com/projectbarks/gopher-code/pkg/ui/core"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// SidePanelMode determines which view is displayed
type SidePanelMode int

const (
	SidePanelModeSessions SidePanelMode = iota
	SidePanelModeTasks
	SidePanelModeFileTree
)

// SidePanel displays a collapsible side panel with multiple views (sessions, tasks, files)
type SidePanel struct {
	visible      bool                    // Whether panel is shown
	mode         SidePanelMode           // Current view mode
	width        int                     // Panel width
	height       int                     // Panel height
	scrollOffset int                     // Scroll position
	selectedIdx  int                     // Currently selected item
	focused      bool                    // Whether panel has focus
	th           theme.Theme             // Theme for styling
	sessions     []session.SessionMetadata
	tasks        []string // Task names
	files        []string // File paths
}

// NewSidePanel creates a new side panel
func NewSidePanel(th theme.Theme) *SidePanel {
	return &SidePanel{
		visible:      true,
		mode:         SidePanelModeSessions,
		width:        30,
		height:       24,
		scrollOffset: 0,
		selectedIdx:  0,
		focused:      false,
		th:           th,
		sessions:     make([]session.SessionMetadata, 0),
		tasks:        make([]string, 0),
		files:        make([]string, 0),
	}
}

// Init initializes the panel
func (sp *SidePanel) Init() tea.Cmd {
	return nil
}

// Update handles input and navigation
func (sp *SidePanel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !sp.visible || !sp.focused {
		return sp, nil
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return sp.handleKeyPress(msg)
	}

	return sp, nil
}

// handleKeyPress processes keyboard navigation
func (sp *SidePanel) handleKeyPress(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "up":
		if sp.selectedIdx > 0 {
			sp.selectedIdx--
			sp.ensureVisible()
		}

	case "down":
		itemCount := sp.itemCount()
		if sp.selectedIdx < itemCount-1 {
			sp.selectedIdx++
			sp.ensureVisible()
		}

	case "home":
		sp.selectedIdx = 0
		sp.scrollOffset = 0

	case "end":
		itemCount := sp.itemCount()
		if itemCount > 0 {
			sp.selectedIdx = itemCount - 1
			sp.ensureVisible()
		}

	case "tab":
		// Cycle through modes: Sessions -> Tasks -> FileTree -> Sessions
		sp.mode = (sp.mode + 1) % 3
		sp.selectedIdx = 0
		sp.scrollOffset = 0

	case "t":
		// Toggle visibility with 't' key
		sp.visible = !sp.visible
	}

	return sp, nil
}

// ensureVisible scrolls to keep selected item in view
func (sp *SidePanel) ensureVisible() {
	viewHeight := sp.height - 3 // Account for header and footer
	if viewHeight <= 0 {
		return
	}

	// Scroll up if selected is above visible area
	if sp.selectedIdx < sp.scrollOffset {
		sp.scrollOffset = sp.selectedIdx
	}

	// Scroll down if selected is below visible area
	if sp.selectedIdx >= sp.scrollOffset+viewHeight {
		sp.scrollOffset = sp.selectedIdx - viewHeight + 1
	}
}

// itemCount returns the number of items in current mode
func (sp *SidePanel) itemCount() int {
	switch sp.mode {
	case SidePanelModeSessions:
		return len(sp.sessions)
	case SidePanelModeTasks:
		return len(sp.tasks)
	case SidePanelModeFileTree:
		return len(sp.files)
	default:
		return 0
	}
}

// View renders the side panel
func (sp *SidePanel) View() tea.View {
	if !sp.visible {
		return tea.NewView("")
	}

	cs := sp.th.Colors()

	// Panel background and border styling
	panelStyle := lipgloss.NewStyle().
		Width(sp.width).
		Height(sp.height).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(cs.TextSecondary)).
		Padding(0, 1)

	// Header with mode indicator
	modeStr := sp.getModeString()
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.TextPrimary)).
		Bold(true).
		Width(sp.width - 2)

	header := headerStyle.Render(fmt.Sprintf("Panel: %s", modeStr))

	// Content area
	viewHeight := sp.height - 4 // Account for header, footer, and borders
	if viewHeight <= 0 {
		return tea.NewView(panelStyle.Render(header))
	}

	var content strings.Builder

	// Render items based on current mode
	items := sp.getVisibleItems(viewHeight)
	for i, item := range items {
		actualIdx := sp.scrollOffset + i
		isSelected := actualIdx == sp.selectedIdx

		style := lipgloss.NewStyle().
			Width(sp.width - 4).
			Foreground(lipgloss.Color(cs.TextSecondary))

		if isSelected {
			style = style.
				Background(lipgloss.Color(cs.Accent)).
				Foreground(lipgloss.Color(cs.Surface))
		}

		line := truncateField(item, sp.width-4)
		content.WriteString(style.Render(line))
		if i < len(items)-1 {
			content.WriteString("\n")
		}
	}

	// Render empty state if no items
	if sp.itemCount() == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(cs.TextSecondary))
		content.WriteString(emptyStyle.Render("(no items)"))
	}

	// Footer with hints
	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.TextMuted)).
		Width(sp.width - 2)
	footer := footerStyle.Render("Tab: switch | T: toggle")

	// Assemble panel
	panelContent := strings.Join([]string{header, content.String(), footer}, "\n")

	return tea.NewView(panelStyle.Render(panelContent))
}

// getModeString returns the string representation of current mode
func (sp *SidePanel) getModeString() string {
	switch sp.mode {
	case SidePanelModeSessions:
		return "Sessions"
	case SidePanelModeTasks:
		return "Tasks"
	case SidePanelModeFileTree:
		return "Files"
	default:
		return "Unknown"
	}
}

// getVisibleItems returns the items for display in the current view height
func (sp *SidePanel) getVisibleItems(viewHeight int) []string {
	var items []string

	switch sp.mode {
	case SidePanelModeSessions:
		for _, s := range sp.sessions {
			// Safely truncate ID to 8 chars
			id := s.ID
			if len(id) > 8 {
				id = id[:8]
			}
			label := fmt.Sprintf("📋 %s", id)
			if s.Model != "" {
				label = fmt.Sprintf("📋 %s (%s)", id, s.Model)
			}
			items = append(items, label)
		}
	case SidePanelModeTasks:
		for _, t := range sp.tasks {
			items = append(items, fmt.Sprintf("✓ %s", t))
		}
	case SidePanelModeFileTree:
		for _, f := range sp.files {
			items = append(items, fmt.Sprintf("📄 %s", f))
		}
	}

	// Limit to visible height
	if len(items) > viewHeight {
		items = items[sp.scrollOffset:]
		if len(items) > viewHeight {
			items = items[:viewHeight]
		}
	}

	return items
}

// SetSize sets the panel dimensions
func (sp *SidePanel) SetSize(width, height int) {
	sp.width = width
	sp.height = height
}

// Focus gives focus to the panel
func (sp *SidePanel) Focus() {
	sp.focused = true
}

// Blur removes focus from the panel
func (sp *SidePanel) Blur() {
	sp.focused = false
}

// Focused returns whether the panel is focused
func (sp *SidePanel) Focused() bool {
	return sp.focused && sp.visible
}

// Toggle toggles panel visibility
func (sp *SidePanel) Toggle() {
	sp.visible = !sp.visible
}

// IsVisible returns whether the panel is visible
func (sp *SidePanel) IsVisible() bool {
	return sp.visible
}

// SetSessions updates the session list
func (sp *SidePanel) SetSessions(sessions []session.SessionMetadata) {
	sp.sessions = sessions
	if sp.selectedIdx >= len(sessions) {
		sp.selectedIdx = 0
	}
}

// SetTasks updates the task list
func (sp *SidePanel) SetTasks(tasks []string) {
	sp.tasks = tasks
	if sp.selectedIdx >= len(tasks) {
		sp.selectedIdx = 0
	}
}

// SetFiles updates the file tree
func (sp *SidePanel) SetFiles(files []string) {
	sp.files = files
	if sp.selectedIdx >= len(files) {
		sp.selectedIdx = 0
	}
}

// GetSelectedSession returns the currently selected session
func (sp *SidePanel) GetSelectedSession() *session.SessionMetadata {
	if sp.mode != SidePanelModeSessions || sp.selectedIdx < 0 || sp.selectedIdx >= len(sp.sessions) {
		return nil
	}
	return &sp.sessions[sp.selectedIdx]
}

// GetSelectedTask returns the currently selected task
func (sp *SidePanel) GetSelectedTask() string {
	if sp.mode != SidePanelModeTasks || sp.selectedIdx < 0 || sp.selectedIdx >= len(sp.tasks) {
		return ""
	}
	return sp.tasks[sp.selectedIdx]
}

// GetSelectedFile returns the currently selected file
func (sp *SidePanel) GetSelectedFile() string {
	if sp.mode != SidePanelModeFileTree || sp.selectedIdx < 0 || sp.selectedIdx >= len(sp.files) {
		return ""
	}
	return sp.files[sp.selectedIdx]
}

// Ensure SidePanel implements core.Component and core.Focusable
var _ core.Component = (*SidePanel)(nil)
var _ core.Focusable = (*SidePanel)(nil)
