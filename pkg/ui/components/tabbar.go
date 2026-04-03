package components

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/projectbarks/gopher-code/pkg/ui/core"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// Tab represents a single tab in the TabBar
type Tab struct {
	ID    string // Unique identifier
	Title string // Display name
	// Can add metadata like unseen message count, etc.
}

// TabBar displays multiple tabs with one active tab
type TabBar struct {
	tabs         []Tab
	activeIdx    int
	width        int
	height       int
	focused      bool
	th           theme.Theme
	onTabChanged func(tabID string) // Callback when tab is switched
}

// NewTabBar creates a new tab bar
func NewTabBar(th theme.Theme) *TabBar {
	return &TabBar{
		tabs:      make([]Tab, 0),
		activeIdx: 0,
		width:     80,
		height:    1,
		focused:   false,
		th:        th,
	}
}

// Init initializes the tab bar
func (tb *TabBar) Init() tea.Cmd {
	return nil
}

// Update handles navigation between tabs
func (tb *TabBar) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !tb.focused || len(tb.tabs) == 0 {
		return tb, nil
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return tb.handleKeyPress(msg)
	}

	return tb, nil
}

// handleKeyPress processes keyboard navigation
func (tb *TabBar) handleKeyPress(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "left":
		if tb.activeIdx > 0 {
			tb.activeIdx--
			tb.triggerTabChange()
		}

	case "right":
		if tb.activeIdx < len(tb.tabs)-1 {
			tb.activeIdx++
			tb.triggerTabChange()
		}

	case "home":
		tb.activeIdx = 0
		tb.triggerTabChange()

	case "end":
		if len(tb.tabs) > 0 {
			tb.activeIdx = len(tb.tabs) - 1
			tb.triggerTabChange()
		}
	}

	return tb, nil
}

// triggerTabChange calls the callback if set
func (tb *TabBar) triggerTabChange() {
	if tb.onTabChanged != nil && tb.activeIdx < len(tb.tabs) {
		tb.onTabChanged(tb.tabs[tb.activeIdx].ID)
	}
}

// View renders the tab bar
func (tb *TabBar) View() tea.View {
	if len(tb.tabs) == 0 {
		return tea.NewView("")
	}

	cs := tb.th.Colors()

	// Build tab line
	var tabsLine strings.Builder

	for i, tab := range tb.tabs {
		isActive := i == tb.activeIdx

		// Tab styling
		tabStyle := lipgloss.NewStyle().
			Padding(0, 1).
			Width(len(tab.Title) + 2)

		if isActive {
			// Active tab: highlighted
			tabStyle = tabStyle.
				Foreground(lipgloss.Color(cs.Surface)).
				Background(lipgloss.Color(cs.Primary)).
				Bold(true)
		} else {
			// Inactive tab: muted
			tabStyle = tabStyle.
				Foreground(lipgloss.Color(cs.TextSecondary)).
				Background(lipgloss.Color(cs.Surface))
		}

		// Render tab
		rendered := tabStyle.Render(tab.Title)
		tabsLine.WriteString(rendered)

		// Add separator between tabs
		if i < len(tb.tabs)-1 {
			sepStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color(cs.BorderSubtle))
			tabsLine.WriteString(sepStyle.Render(" │ "))
		}
	}

	// Container styling
	containerStyle := lipgloss.NewStyle().
		Width(tb.width).
		Height(tb.height).
		Foreground(lipgloss.Color(cs.TextPrimary)).
		Padding(0, 1)

	return tea.NewView(containerStyle.Render(tabsLine.String()))
}

// SetSize sets the tab bar dimensions
func (tb *TabBar) SetSize(width, height int) {
	tb.width = width
	tb.height = height
}

// Focus gives focus to the tab bar
func (tb *TabBar) Focus() {
	tb.focused = true
}

// Blur removes focus from the tab bar
func (tb *TabBar) Blur() {
	tb.focused = false
}

// Focused returns whether the tab bar is focused
func (tb *TabBar) Focused() bool {
	return tb.focused
}

// AddTab adds a new tab
func (tb *TabBar) AddTab(id, title string) {
	tab := Tab{ID: id, Title: title}
	tb.tabs = append(tb.tabs, tab)

	// If this is the first tab, make it active
	if len(tb.tabs) == 1 {
		tb.activeIdx = 0
	}
}

// RemoveTab removes a tab by ID
func (tb *TabBar) RemoveTab(id string) {
	for i, tab := range tb.tabs {
		if tab.ID == id {
			// Remove the tab
			tb.tabs = append(tb.tabs[:i], tb.tabs[i+1:]...)

			// Adjust active index if needed
			if i == tb.activeIdx && tb.activeIdx >= len(tb.tabs) && tb.activeIdx > 0 {
				tb.activeIdx--
			}

			// Trigger change if active tab was removed
			if i == tb.activeIdx && len(tb.tabs) > 0 {
				tb.triggerTabChange()
			}

			break
		}
	}
}

// SetActiveTab sets the active tab by ID
func (tb *TabBar) SetActiveTab(id string) bool {
	for i, tab := range tb.tabs {
		if tab.ID == id {
			tb.activeIdx = i
			tb.triggerTabChange()
			return true
		}
	}
	return false
}

// GetActiveTab returns the active tab
func (tb *TabBar) GetActiveTab() *Tab {
	if tb.activeIdx < 0 || tb.activeIdx >= len(tb.tabs) {
		return nil
	}
	return &tb.tabs[tb.activeIdx]
}

// GetActiveTabID returns the ID of the active tab
func (tb *TabBar) GetActiveTabID() string {
	if active := tb.GetActiveTab(); active != nil {
		return active.ID
	}
	return ""
}

// TabCount returns the number of tabs
func (tb *TabBar) TabCount() int {
	return len(tb.tabs)
}

// SetTabTitle updates a tab's title
func (tb *TabBar) SetTabTitle(id, newTitle string) bool {
	for i := range tb.tabs {
		if tb.tabs[i].ID == id {
			tb.tabs[i].Title = newTitle
			return true
		}
	}
	return false
}

// SetOnTabChanged sets the callback for tab changes
func (tb *TabBar) SetOnTabChanged(callback func(tabID string)) {
	tb.onTabChanged = callback
}

// GetTabByID returns a tab by ID
func (tb *TabBar) GetTabByID(id string) *Tab {
	for i, tab := range tb.tabs {
		if tab.ID == id {
			return &tb.tabs[i]
		}
	}
	return nil
}

// HasTab checks if a tab exists by ID
func (tb *TabBar) HasTab(id string) bool {
	return tb.GetTabByID(id) != nil
}

// Ensure TabBar implements core.Component and core.Focusable
var _ core.Component = (*TabBar)(nil)
var _ core.Focusable = (*TabBar)(nil)
