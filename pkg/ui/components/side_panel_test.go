package components

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/projectbarks/gopher-code/pkg/session"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

func TestSidePanelCreation(t *testing.T) {
	th := theme.Current()
	sp := NewSidePanel(th)

	if sp == nil {
		t.Fatal("SidePanel should not be nil")
	}

	if !sp.visible {
		t.Error("Panel should be visible initially")
	}

	if sp.mode != SidePanelModeSessions {
		t.Errorf("Initial mode should be Sessions, got %d", sp.mode)
	}

	if sp.selectedIdx != 0 {
		t.Error("Selected index should start at 0")
	}

	if sp.focused {
		t.Error("Panel should not be focused initially")
	}
}

func TestSidePanelToggle(t *testing.T) {
	th := theme.Current()
	sp := NewSidePanel(th)

	if !sp.IsVisible() {
		t.Error("Panel should be visible initially")
	}

	sp.Toggle()
	if sp.IsVisible() {
		t.Error("Panel should be hidden after toggle")
	}

	sp.Toggle()
	if !sp.IsVisible() {
		t.Error("Panel should be visible after second toggle")
	}
}

func TestSidePanelSetSessions(t *testing.T) {
	th := theme.Current()
	sp := NewSidePanel(th)

	sessions := []session.SessionMetadata{
		{ID: "session-1", Model: "claude-opus"},
		{ID: "session-2", Model: "claude-sonnet"},
		{ID: "session-3", Model: "claude-haiku"},
	}

	sp.SetSessions(sessions)

	if len(sp.sessions) != 3 {
		t.Errorf("Expected 3 sessions, got %d", len(sp.sessions))
	}

	selected := sp.GetSelectedSession()
	if selected == nil || selected.ID != "session-1" {
		t.Error("First session should be selected")
	}
}

func TestSidePanelSetTasks(t *testing.T) {
	th := theme.Current()
	sp := NewSidePanel(th)

	tasks := []string{"Task 1", "Task 2", "Task 3"}
	sp.SetTasks(tasks)

	if len(sp.tasks) != 3 {
		t.Errorf("Expected 3 tasks, got %d", len(sp.tasks))
	}

	// Switch to Tasks mode to select task
	sp.mode = SidePanelModeTasks
	if sp.GetSelectedTask() != "Task 1" {
		t.Error("First task should be selected")
	}
}

func TestSidePanelSetFiles(t *testing.T) {
	th := theme.Current()
	sp := NewSidePanel(th)

	files := []string{"file1.go", "file2.go", "file3.go"}
	sp.SetFiles(files)

	if len(sp.files) != 3 {
		t.Errorf("Expected 3 files, got %d", len(sp.files))
	}

	// Switch to FileTree mode to select file
	sp.mode = SidePanelModeFileTree
	if sp.GetSelectedFile() != "file1.go" {
		t.Error("First file should be selected")
	}
}

func TestSidePanelNavigateUp(t *testing.T) {
	th := theme.Current()
	sp := NewSidePanel(th)

	sessions := []session.SessionMetadata{
		{ID: "s1"},
		{ID: "s2"},
		{ID: "s3"},
	}
	sp.SetSessions(sessions)
	sp.selectedIdx = 2

	msg := tea.KeyPressMsg{Code: tea.KeyUp}
	sp.Focus()
	sp.Update(msg)

	if sp.selectedIdx != 1 {
		t.Errorf("Selected index should be 1, got %d", sp.selectedIdx)
	}
}

func TestSidePanelNavigateDown(t *testing.T) {
	th := theme.Current()
	sp := NewSidePanel(th)

	sessions := []session.SessionMetadata{
		{ID: "s1"},
		{ID: "s2"},
		{ID: "s3"},
	}
	sp.SetSessions(sessions)
	sp.selectedIdx = 0

	msg := tea.KeyPressMsg{Code: tea.KeyDown}
	sp.Focus()
	sp.Update(msg)

	if sp.selectedIdx != 1 {
		t.Errorf("Selected index should be 1, got %d", sp.selectedIdx)
	}
}

func TestSidePanelNavigateHome(t *testing.T) {
	th := theme.Current()
	sp := NewSidePanel(th)

	sessions := []session.SessionMetadata{
		{ID: "s1"},
		{ID: "s2"},
		{ID: "s3"},
	}
	sp.SetSessions(sessions)
	sp.selectedIdx = 2
	sp.scrollOffset = 1

	msg := tea.KeyPressMsg{Code: tea.KeyHome}
	sp.Focus()
	sp.Update(msg)

	if sp.selectedIdx != 0 {
		t.Errorf("Selected index should be 0, got %d", sp.selectedIdx)
	}

	if sp.scrollOffset != 0 {
		t.Errorf("Scroll offset should be 0, got %d", sp.scrollOffset)
	}
}

func TestSidePanelNavigateEnd(t *testing.T) {
	th := theme.Current()
	sp := NewSidePanel(th)

	sessions := []session.SessionMetadata{
		{ID: "s1"},
		{ID: "s2"},
		{ID: "s3"},
	}
	sp.SetSessions(sessions)
	sp.selectedIdx = 0

	msg := tea.KeyPressMsg{Code: tea.KeyEnd}
	sp.Focus()
	sp.Update(msg)

	if sp.selectedIdx != 2 {
		t.Errorf("Selected index should be 2, got %d", sp.selectedIdx)
	}
}

func TestSidePanelModeSwitch(t *testing.T) {
	th := theme.Current()
	sp := NewSidePanel(th)

	sessions := []session.SessionMetadata{{ID: "s1"}}
	tasks := []string{"Task 1"}
	files := []string{"file1.go"}

	sp.SetSessions(sessions)
	sp.SetTasks(tasks)
	sp.SetFiles(files)

	// Start in Sessions mode
	if sp.mode != SidePanelModeSessions {
		t.Error("Should start in Sessions mode")
	}

	// Switch to Tasks
	msg := tea.KeyPressMsg{Code: tea.KeyTab}
	sp.Focus()
	sp.Update(msg)

	if sp.mode != SidePanelModeTasks {
		t.Errorf("Mode should be Tasks, got %d", sp.mode)
	}

	if sp.selectedIdx != 0 {
		t.Error("Selected index should reset to 0 on mode switch")
	}

	// Switch to FileTree
	sp.Update(msg)
	if sp.mode != SidePanelModeFileTree {
		t.Errorf("Mode should be FileTree, got %d", sp.mode)
	}

	// Cycle back to Sessions
	sp.Update(msg)
	if sp.mode != SidePanelModeSessions {
		t.Errorf("Mode should cycle back to Sessions, got %d", sp.mode)
	}
}

func TestSidePanelDirectToggle(t *testing.T) {
	th := theme.Current()
	sp := NewSidePanel(th)

	if !sp.visible {
		t.Error("Should be visible initially")
	}

	sp.Toggle()
	if sp.visible {
		t.Error("Should be hidden after toggle")
	}

	sp.Toggle()
	if !sp.visible {
		t.Error("Should be visible after second toggle")
	}
}

func TestSidePanelFocusBlur(t *testing.T) {
	th := theme.Current()
	sp := NewSidePanel(th)

	if sp.Focused() {
		t.Error("Should not be focused initially")
	}

	sp.Focus()
	if !sp.Focused() {
		t.Error("Should be focused after Focus()")
	}

	sp.Blur()
	if sp.Focused() {
		t.Error("Should not be focused after Blur()")
	}
}

func TestSidePanelFocusWhenHidden(t *testing.T) {
	th := theme.Current()
	sp := NewSidePanel(th)

	sp.Focus()
	sp.Toggle() // Hide

	if sp.Focused() {
		t.Error("Should not be focused when hidden")
	}
}

func TestSidePanelView(t *testing.T) {
	th := theme.Current()
	sp := NewSidePanel(th)

	sessions := []session.SessionMetadata{
		{ID: "session-1", Model: "claude-opus"},
	}
	sp.SetSessions(sessions)
	sp.SetSize(30, 10)

	view := sp.View()

	if view.Content == "" {
		t.Error("View should not be empty")
	}

	if !strings.Contains(view.Content, "Sessions") {
		t.Error("View should contain mode name")
	}
}

func TestSidePanelViewWhenHidden(t *testing.T) {
	th := theme.Current()
	sp := NewSidePanel(th)

	sp.Toggle() // Hide

	view := sp.View()

	if view.Content != "" {
		t.Error("View should be empty when hidden")
	}
}

func TestSidePanelViewEmpty(t *testing.T) {
	th := theme.Current()
	sp := NewSidePanel(th)

	sp.SetSize(30, 10)

	view := sp.View()

	if view.Content == "" {
		t.Error("View should not be empty")
	}

	if !strings.Contains(view.Content, "(no items)") {
		t.Error("View should show empty state")
	}
}

func TestSidePanelSetSize(t *testing.T) {
	th := theme.Current()
	sp := NewSidePanel(th)

	sp.SetSize(40, 20)

	if sp.width != 40 {
		t.Errorf("Width should be 40, got %d", sp.width)
	}

	if sp.height != 20 {
		t.Errorf("Height should be 20, got %d", sp.height)
	}
}

func TestSidePanelInit(t *testing.T) {
	th := theme.Current()
	sp := NewSidePanel(th)

	cmd := sp.Init()

	if cmd != nil {
		t.Error("Init should return nil")
	}
}

func TestSidePanelUpdateWhenNotFocused(t *testing.T) {
	th := theme.Current()
	sp := NewSidePanel(th)

	sessions := []session.SessionMetadata{{ID: "s1"}}
	sp.SetSessions(sessions)

	// Update without focus
	originalIdx := sp.selectedIdx
	msg := tea.KeyPressMsg{Code: tea.KeyDown}
	sp.Update(msg)

	if sp.selectedIdx != originalIdx {
		t.Error("Should not process input when not focused")
	}
}

func TestSidePanelNavigationBoundaries(t *testing.T) {
	th := theme.Current()
	sp := NewSidePanel(th)

	sessions := []session.SessionMetadata{
		{ID: "s1"},
		{ID: "s2"},
	}
	sp.SetSessions(sessions)
	sp.Focus()

	// Try to go up from first item
	msg := tea.KeyPressMsg{Code: tea.KeyUp}
	sp.Update(msg)

	if sp.selectedIdx != 0 {
		t.Error("Should not go above first item")
	}

	// Try to go down past last item
	sp.selectedIdx = 1
	msg = tea.KeyPressMsg{Code: tea.KeyDown}
	sp.Update(msg)

	if sp.selectedIdx != 1 {
		t.Error("Should not go past last item")
	}
}

func TestSidePanelGetSelectedWrongMode(t *testing.T) {
	th := theme.Current()
	sp := NewSidePanel(th)

	sessions := []session.SessionMetadata{{ID: "s1"}}
	sp.SetSessions(sessions)
	sp.mode = SidePanelModeTasks // Switch mode without setting tasks

	selected := sp.GetSelectedSession()
	if selected != nil {
		t.Error("Should return nil when in wrong mode")
	}
}

func TestSidePanelGetSelectedInvalidIndex(t *testing.T) {
	th := theme.Current()
	sp := NewSidePanel(th)

	sessions := []session.SessionMetadata{{ID: "s1"}}
	sp.SetSessions(sessions)
	sp.selectedIdx = 10 // Out of bounds

	selected := sp.GetSelectedSession()
	if selected != nil {
		t.Error("Should return nil for out of bounds index")
	}
}

func TestSidePanelModeString(t *testing.T) {
	th := theme.Current()
	sp := NewSidePanel(th)

	sp.mode = SidePanelModeSessions
	if sp.getModeString() != "Sessions" {
		t.Error("Mode string should be Sessions")
	}

	sp.mode = SidePanelModeTasks
	if sp.getModeString() != "Tasks" {
		t.Error("Mode string should be Tasks")
	}

	sp.mode = SidePanelModeFileTree
	if sp.getModeString() != "Files" {
		t.Error("Mode string should be Files")
	}
}

func TestSidePanelScrolling(t *testing.T) {
	th := theme.Current()
	sp := NewSidePanel(th)
	sp.SetSize(30, 5) // Small height to force scrolling

	// Add many items
	files := make([]string, 20)
	for i := 0; i < 20; i++ {
		files[i] = "file" + string(rune(i)) + ".go"
	}
	sp.SetFiles(files)
	sp.mode = SidePanelModeFileTree
	sp.Focus()

	// Navigate to last item
	for i := 0; i < 19; i++ {
		msg := tea.KeyPressMsg{Code: tea.KeyDown}
		sp.Update(msg)
	}

	if sp.selectedIdx != 19 {
		t.Errorf("Should be at item 19, got %d", sp.selectedIdx)
	}

	// Check that scroll offset allows visibility
	if sp.scrollOffset < 0 {
		t.Error("Scroll offset should not be negative")
	}
}

func TestSidePanelItemCount(t *testing.T) {
	th := theme.Current()
	sp := NewSidePanel(th)

	count := sp.itemCount()
	if count != 0 {
		t.Errorf("Initial count should be 0, got %d", count)
	}

	sessions := []session.SessionMetadata{{ID: "s1"}, {ID: "s2"}}
	sp.SetSessions(sessions)

	count = sp.itemCount()
	if count != 2 {
		t.Errorf("Count should be 2, got %d", count)
	}
}

func TestSidePanelViewWithMultipleModes(t *testing.T) {
	th := theme.Current()
	sp := NewSidePanel(th)

	sessions := []session.SessionMetadata{{ID: "s1", Model: "claude"}}
	tasks := []string{"Task A"}
	files := []string{"main.go"}

	sp.SetSessions(sessions)
	sp.SetTasks(tasks)
	sp.SetFiles(files)
	sp.SetSize(30, 10)

	// Test Sessions view
	sp.mode = SidePanelModeSessions
	view := sp.View()
	if !strings.Contains(view.Content, "Sessions") {
		t.Error("Sessions view should show Sessions mode")
	}

	// Test Tasks view
	sp.mode = SidePanelModeTasks
	view = sp.View()
	if !strings.Contains(view.Content, "Tasks") {
		t.Error("Tasks view should show Tasks mode")
	}

	// Test FileTree view
	sp.mode = SidePanelModeFileTree
	view = sp.View()
	if !strings.Contains(view.Content, "Files") {
		t.Error("FileTree view should show Files mode")
	}
}

func TestSidePanelSessionUpdate(t *testing.T) {
	th := theme.Current()
	sp := NewSidePanel(th)

	sessions1 := []session.SessionMetadata{{ID: "s1"}}
	sp.SetSessions(sessions1)
	sp.selectedIdx = 0

	// Update with different sessions
	sessions2 := []session.SessionMetadata{
		{ID: "new1"},
		{ID: "new2"},
		{ID: "new3"},
	}
	sp.SetSessions(sessions2)

	if len(sp.sessions) != 3 {
		t.Errorf("Sessions should be updated, expected 3 got %d", len(sp.sessions))
	}

	if sp.selectedIdx != 0 {
		t.Error("Selected index should stay valid after update")
	}
}

func TestSidePanelSelectedOutOfBoundsAfterSetItems(t *testing.T) {
	th := theme.Current()
	sp := NewSidePanel(th)

	sessions := []session.SessionMetadata{
		{ID: "s1"},
		{ID: "s2"},
		{ID: "s3"},
	}
	sp.SetSessions(sessions)
	sp.selectedIdx = 2

	// Update with fewer sessions
	newSessions := []session.SessionMetadata{{ID: "only-one"}}
	sp.SetSessions(newSessions)

	if sp.selectedIdx >= len(sp.sessions) {
		t.Error("Selected index should be adjusted to valid range")
	}
}
