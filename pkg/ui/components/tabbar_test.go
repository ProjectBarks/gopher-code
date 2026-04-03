package components

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

func TestTabBarCreation(t *testing.T) {
	th := theme.Current()
	tb := NewTabBar(th)

	if tb == nil {
		t.Fatal("TabBar should not be nil")
	}

	if tb.TabCount() != 0 {
		t.Error("TabBar should start with 0 tabs")
	}

	if tb.focused {
		t.Error("TabBar should not be focused initially")
	}
}

func TestTabBarAddTab(t *testing.T) {
	th := theme.Current()
	tb := NewTabBar(th)

	tb.AddTab("tab-1", "Tab 1")
	tb.AddTab("tab-2", "Tab 2")
	tb.AddTab("tab-3", "Tab 3")

	if tb.TabCount() != 3 {
		t.Errorf("Expected 3 tabs, got %d", tb.TabCount())
	}

	active := tb.GetActiveTab()
	if active == nil || active.ID != "tab-1" {
		t.Error("First tab should be active")
	}
}

func TestTabBarRemoveTab(t *testing.T) {
	th := theme.Current()
	tb := NewTabBar(th)

	tb.AddTab("tab-1", "Tab 1")
	tb.AddTab("tab-2", "Tab 2")
	tb.AddTab("tab-3", "Tab 3")

	tb.RemoveTab("tab-2")

	if tb.TabCount() != 2 {
		t.Errorf("Expected 2 tabs, got %d", tb.TabCount())
	}

	if tb.HasTab("tab-2") {
		t.Error("Removed tab should not exist")
	}
}

func TestTabBarSetActiveTab(t *testing.T) {
	th := theme.Current()
	tb := NewTabBar(th)

	tb.AddTab("tab-1", "Tab 1")
	tb.AddTab("tab-2", "Tab 2")
	tb.AddTab("tab-3", "Tab 3")

	success := tb.SetActiveTab("tab-2")
	if !success {
		t.Error("SetActiveTab should return true for existing tab")
	}

	active := tb.GetActiveTab()
	if active == nil || active.ID != "tab-2" {
		t.Error("Tab 2 should be active")
	}
}

func TestTabBarSetActiveTabInvalid(t *testing.T) {
	th := theme.Current()
	tb := NewTabBar(th)

	tb.AddTab("tab-1", "Tab 1")

	success := tb.SetActiveTab("nonexistent")
	if success {
		t.Error("SetActiveTab should return false for nonexistent tab")
	}
}

func TestTabBarGetActiveTabID(t *testing.T) {
	th := theme.Current()
	tb := NewTabBar(th)

	tb.AddTab("tab-1", "Tab 1")
	tb.AddTab("tab-2", "Tab 2")

	id := tb.GetActiveTabID()
	if id != "tab-1" {
		t.Errorf("Active tab ID should be 'tab-1', got '%s'", id)
	}

	tb.SetActiveTab("tab-2")
	id = tb.GetActiveTabID()
	if id != "tab-2" {
		t.Errorf("Active tab ID should be 'tab-2', got '%s'", id)
	}
}

func TestTabBarNavigateLeft(t *testing.T) {
	th := theme.Current()
	tb := NewTabBar(th)

	tb.AddTab("tab-1", "Tab 1")
	tb.AddTab("tab-2", "Tab 2")
	tb.AddTab("tab-3", "Tab 3")

	tb.SetActiveTab("tab-2")
	tb.Focus()

	msg := tea.KeyPressMsg{Code: tea.KeyLeft}
	tb.Update(msg)

	active := tb.GetActiveTab()
	if active == nil || active.ID != "tab-1" {
		t.Error("Should navigate left to tab-1")
	}
}

func TestTabBarNavigateRight(t *testing.T) {
	th := theme.Current()
	tb := NewTabBar(th)

	tb.AddTab("tab-1", "Tab 1")
	tb.AddTab("tab-2", "Tab 2")
	tb.AddTab("tab-3", "Tab 3")

	tb.Focus()

	msg := tea.KeyPressMsg{Code: tea.KeyRight}
	tb.Update(msg)

	active := tb.GetActiveTab()
	if active == nil || active.ID != "tab-2" {
		t.Error("Should navigate right to tab-2")
	}
}

func TestTabBarNavigateHome(t *testing.T) {
	th := theme.Current()
	tb := NewTabBar(th)

	tb.AddTab("tab-1", "Tab 1")
	tb.AddTab("tab-2", "Tab 2")
	tb.AddTab("tab-3", "Tab 3")

	tb.SetActiveTab("tab-3")
	tb.Focus()

	msg := tea.KeyPressMsg{Code: tea.KeyHome}
	tb.Update(msg)

	active := tb.GetActiveTab()
	if active == nil || active.ID != "tab-1" {
		t.Error("Should navigate to first tab")
	}
}

func TestTabBarNavigateEnd(t *testing.T) {
	th := theme.Current()
	tb := NewTabBar(th)

	tb.AddTab("tab-1", "Tab 1")
	tb.AddTab("tab-2", "Tab 2")
	tb.AddTab("tab-3", "Tab 3")

	tb.Focus()

	msg := tea.KeyPressMsg{Code: tea.KeyEnd}
	tb.Update(msg)

	active := tb.GetActiveTab()
	if active == nil || active.ID != "tab-3" {
		t.Error("Should navigate to last tab")
	}
}

func TestTabBarNavigationBoundaries(t *testing.T) {
	th := theme.Current()
	tb := NewTabBar(th)

	tb.AddTab("tab-1", "Tab 1")
	tb.AddTab("tab-2", "Tab 2")
	tb.Focus()

	// Try to go left from first tab
	msg := tea.KeyPressMsg{Code: tea.KeyLeft}
	tb.Update(msg)

	if tb.activeIdx != 0 {
		t.Error("Should not go left from first tab")
	}

	// Navigate to last tab and try to go right
	tb.SetActiveTab("tab-2")
	msg = tea.KeyPressMsg{Code: tea.KeyRight}
	tb.Update(msg)

	if tb.activeIdx != 1 {
		t.Error("Should not go right past last tab")
	}
}

func TestTabBarFocusBlur(t *testing.T) {
	th := theme.Current()
	tb := NewTabBar(th)

	if tb.Focused() {
		t.Error("Should not be focused initially")
	}

	tb.Focus()
	if !tb.Focused() {
		t.Error("Should be focused after Focus()")
	}

	tb.Blur()
	if tb.Focused() {
		t.Error("Should not be focused after Blur()")
	}
}

func TestTabBarView(t *testing.T) {
	th := theme.Current()
	tb := NewTabBar(th)

	tb.AddTab("tab-1", "Tab 1")
	tb.AddTab("tab-2", "Tab 2")
	tb.SetSize(80, 1)

	view := tb.View()

	if view.Content == "" {
		t.Error("View should not be empty")
	}

	if !strings.Contains(view.Content, "Tab 1") {
		t.Error("View should contain tab title")
	}
}

func TestTabBarViewEmpty(t *testing.T) {
	th := theme.Current()
	tb := NewTabBar(th)

	tb.SetSize(80, 1)
	view := tb.View()

	if view.Content != "" {
		t.Error("View should be empty when no tabs")
	}
}

func TestTabBarSetTabTitle(t *testing.T) {
	th := theme.Current()
	tb := NewTabBar(th)

	tb.AddTab("tab-1", "Original")

	success := tb.SetTabTitle("tab-1", "Updated")
	if !success {
		t.Error("SetTabTitle should return true")
	}

	tab := tb.GetTabByID("tab-1")
	if tab == nil || tab.Title != "Updated" {
		t.Error("Tab title should be updated")
	}
}

func TestTabBarSetTabTitleInvalid(t *testing.T) {
	th := theme.Current()
	tb := NewTabBar(th)

	tb.AddTab("tab-1", "Tab")

	success := tb.SetTabTitle("nonexistent", "New Title")
	if success {
		t.Error("SetTabTitle should return false for nonexistent tab")
	}
}

func TestTabBarCallback(t *testing.T) {
	th := theme.Current()
	tb := NewTabBar(th)

	tb.AddTab("tab-1", "Tab 1")
	tb.AddTab("tab-2", "Tab 2")

	var lastChangedTab string
	tb.SetOnTabChanged(func(tabID string) {
		lastChangedTab = tabID
	})

	tb.Focus()
	msg := tea.KeyPressMsg{Code: tea.KeyRight}
	tb.Update(msg)

	if lastChangedTab != "tab-2" {
		t.Errorf("Callback should be called with 'tab-2', got '%s'", lastChangedTab)
	}
}

func TestTabBarRemoveActiveTab(t *testing.T) {
	th := theme.Current()
	tb := NewTabBar(th)

	tb.AddTab("tab-1", "Tab 1")
	tb.AddTab("tab-2", "Tab 2")
	tb.AddTab("tab-3", "Tab 3")

	tb.SetActiveTab("tab-2")
	tb.RemoveTab("tab-2")

	if tb.TabCount() != 2 {
		t.Error("Should have 2 tabs after removal")
	}

	// Active should be adjusted
	active := tb.GetActiveTab()
	if active == nil {
		t.Fatal("Active tab should not be nil")
	}

	if active.ID != "tab-1" && active.ID != "tab-3" {
		t.Error("Active tab should be adjusted after removal")
	}
}

func TestTabBarRemoveLastTab(t *testing.T) {
	th := theme.Current()
	tb := NewTabBar(th)

	tb.AddTab("tab-1", "Tab 1")
	tb.AddTab("tab-2", "Tab 2")

	tb.SetActiveTab("tab-2")
	tb.RemoveTab("tab-2")

	if tb.TabCount() != 1 {
		t.Error("Should have 1 tab")
	}

	active := tb.GetActiveTab()
	if active == nil || active.ID != "tab-1" {
		t.Error("Should fall back to first tab")
	}
}

func TestTabBarGetTabByID(t *testing.T) {
	th := theme.Current()
	tb := NewTabBar(th)

	tb.AddTab("tab-1", "Tab 1")
	tb.AddTab("tab-2", "Tab 2")

	tab := tb.GetTabByID("tab-1")
	if tab == nil || tab.Title != "Tab 1" {
		t.Error("Should get correct tab by ID")
	}

	tab = tb.GetTabByID("nonexistent")
	if tab != nil {
		t.Error("Should return nil for nonexistent tab")
	}
}

func TestTabBarHasTab(t *testing.T) {
	th := theme.Current()
	tb := NewTabBar(th)

	tb.AddTab("tab-1", "Tab 1")

	if !tb.HasTab("tab-1") {
		t.Error("HasTab should return true for existing tab")
	}

	if tb.HasTab("nonexistent") {
		t.Error("HasTab should return false for nonexistent tab")
	}
}

func TestTabBarSetSize(t *testing.T) {
	th := theme.Current()
	tb := NewTabBar(th)

	tb.SetSize(100, 2)

	if tb.width != 100 {
		t.Errorf("Width should be 100, got %d", tb.width)
	}

	if tb.height != 2 {
		t.Errorf("Height should be 2, got %d", tb.height)
	}
}

func TestTabBarInit(t *testing.T) {
	th := theme.Current()
	tb := NewTabBar(th)

	cmd := tb.Init()

	if cmd != nil {
		t.Error("Init should return nil")
	}
}

func TestTabBarUpdateWhenNotFocused(t *testing.T) {
	th := theme.Current()
	tb := NewTabBar(th)

	tb.AddTab("tab-1", "Tab 1")
	tb.AddTab("tab-2", "Tab 2")

	originalIdx := tb.activeIdx

	// Update without focus
	msg := tea.KeyPressMsg{Code: tea.KeyRight}
	tb.Update(msg)

	if tb.activeIdx != originalIdx {
		t.Error("Should not process input when not focused")
	}
}

func TestTabBarUpdateWithNoTabs(t *testing.T) {
	th := theme.Current()
	tb := NewTabBar(th)

	tb.Focus()

	// Should not panic
	msg := tea.KeyPressMsg{Code: tea.KeyRight}
	tb.Update(msg)

	if tb.TabCount() != 0 {
		t.Error("Tab count should still be 0")
	}
}

func TestTabBarMultipleAddsAndRemoves(t *testing.T) {
	th := theme.Current()
	tb := NewTabBar(th)

	// Add tabs
	for i := 1; i <= 5; i++ {
		id := "tab-" + string(rune('0'+i))
		title := "Tab " + string(rune('0'+i))
		tb.AddTab(id, title)
	}

	if tb.TabCount() != 5 {
		t.Errorf("Expected 5 tabs, got %d", tb.TabCount())
	}

	// Remove some tabs
	tb.RemoveTab("tab-2")
	tb.RemoveTab("tab-4")

	if tb.TabCount() != 3 {
		t.Errorf("Expected 3 tabs, got %d", tb.TabCount())
	}

	// Remaining tabs should be 1, 3, 5
	if !tb.HasTab("tab-1") || !tb.HasTab("tab-3") || !tb.HasTab("tab-5") {
		t.Error("Expected tabs 1, 3, 5 to remain")
	}
}

func TestTabBarActiveIndexAfterRemoval(t *testing.T) {
	th := theme.Current()
	tb := NewTabBar(th)

	tb.AddTab("tab-1", "Tab 1")
	tb.AddTab("tab-2", "Tab 2")
	tb.AddTab("tab-3", "Tab 3")

	// Set active to middle tab and remove it
	tb.activeIdx = 1 // tab-2
	tb.RemoveTab("tab-2")

	// activeIdx should be adjusted
	if tb.activeIdx >= tb.TabCount() {
		t.Error("ActiveIdx should be adjusted after removal")
	}
}

func TestTabBarGetActiveTabWhenNone(t *testing.T) {
	th := theme.Current()
	tb := NewTabBar(th)

	active := tb.GetActiveTab()
	if active != nil {
		t.Error("GetActiveTab should return nil when no tabs")
	}
}

func TestTabBarNavigationWithFewTabs(t *testing.T) {
	th := theme.Current()
	tb := NewTabBar(th)

	tb.AddTab("tab-1", "Only Tab")
	tb.Focus()

	// Try to navigate left and right with only one tab
	msg := tea.KeyPressMsg{Code: tea.KeyLeft}
	tb.Update(msg)

	if tb.activeIdx != 0 {
		t.Error("Should stay at first tab with only one tab")
	}

	msg = tea.KeyPressMsg{Code: tea.KeyRight}
	tb.Update(msg)

	if tb.activeIdx != 0 {
		t.Error("Should stay at first tab with only one tab")
	}
}
