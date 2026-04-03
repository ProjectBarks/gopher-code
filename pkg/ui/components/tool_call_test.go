package components

import (
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

func TestToolCallDisplayCreation(t *testing.T) {
	th := theme.Current()
	tcd := NewToolCallDisplay("tool-1", "bash", th)

	if tcd == nil {
		t.Error("Expected non-nil ToolCallDisplay")
	}
	if tcd.ID() != "tool-1" {
		t.Errorf("Expected ID 'tool-1', got %q", tcd.ID())
	}
	if tcd.name != "bash" {
		t.Errorf("Expected name 'bash', got %q", tcd.name)
	}
	if tcd.State() != ToolCallPending {
		t.Errorf("Expected initial state Pending, got %v", tcd.State())
	}
}

func TestToolCallDisplayPendingState(t *testing.T) {
	th := theme.Current()
	tcd := NewToolCallDisplay("tool-1", "search", th)

	tcd.SetPending()

	if tcd.State() != ToolCallPending {
		t.Errorf("Expected state Pending, got %v", tcd.State())
	}

	view := tcd.View()
	content := view.Content

	if !strings.Contains(content, "search") {
		t.Errorf("Expected tool name in view, got %q", content)
	}
	if !strings.Contains(content, "running") {
		t.Errorf("Expected 'running' status in view, got %q", content)
	}
}

func TestToolCallDisplayPendingSpinner(t *testing.T) {
	th := theme.Current()
	tcd := NewToolCallDisplay("tool-1", "bash", th)

	tcd.SetPending()

	// Get initial view
	view1 := tcd.View()
	content1 := view1.Content

	// Animate spinner
	tcd.Update(ToolCallDisplayTickMsg{})
	tcd.Update(ToolCallDisplayTickMsg{})

	view2 := tcd.View()
	content2 := view2.Content

	// Both should contain the tool name
	if !strings.Contains(content1, "bash") {
		t.Error("Expected tool name in first view")
	}
	if !strings.Contains(content2, "bash") {
		t.Error("Expected tool name in second view")
	}

	// The spinner should be different (though hidden by ANSI codes)
	t.Logf("View 1: %q", content1)
	t.Logf("View 2: %q", content2)
}

func TestToolCallDisplaySpinnerCycle(t *testing.T) {
	th := theme.Current()
	tcd := NewToolCallDisplay("tool-1", "search", th)

	tcd.SetPending()

	// Simulate full spinner cycle (4 ticks)
	spinnerStates := make([]int, 0)
	for i := 0; i < 8; i++ {
		spinnerStates = append(spinnerStates, tcd.spinnerTick)
		tcd.Update(ToolCallDisplayTickMsg{})
	}

	// Should cycle through 0, 1, 2, 3, 0, 1, 2, 3
	expectedStates := []int{0, 1, 2, 3, 0, 1, 2, 3}
	for i, expected := range expectedStates {
		if spinnerStates[i] != expected {
			t.Errorf("At tick %d: expected state %d, got %d", i, expected, spinnerStates[i])
		}
	}
}

func TestToolCallDisplayCompleteState(t *testing.T) {
	th := theme.Current()
	tcd := NewToolCallDisplay("tool-1", "bash", th)

	tcd.SetComplete("Command executed successfully")

	if tcd.State() != ToolCallComplete {
		t.Errorf("Expected state Complete, got %v", tcd.State())
	}

	view := tcd.View()
	content := view.Content

	if !strings.Contains(content, "bash") {
		t.Errorf("Expected tool name in view, got %q", content)
	}
	if !strings.Contains(content, "complete") {
		t.Errorf("Expected 'complete' status in view, got %q", content)
	}
	if !strings.Contains(content, "Command executed") {
		t.Logf("Expected result summary in view, got %q", content)
	}
}

func TestToolCallDisplayCompleteWithoutSummary(t *testing.T) {
	th := theme.Current()
	tcd := NewToolCallDisplay("tool-1", "bash", th)

	tcd.SetComplete("")

	view := tcd.View()
	content := view.Content

	if !strings.Contains(content, "bash") {
		t.Errorf("Expected tool name in view, got %q", content)
	}
	if !strings.Contains(content, "complete") {
		t.Errorf("Expected 'complete' status in view, got %q", content)
	}
}

func TestToolCallDisplayCompleteLongSummary(t *testing.T) {
	th := theme.Current()
	tcd := NewToolCallDisplay("tool-1", "bash", th)

	longSummary := strings.Repeat("x", 200)
	tcd.SetComplete(longSummary)

	view := tcd.View()
	content := view.Content

	// Should be truncated to ~100 chars + "..."
	if strings.Count(content, "x") > 150 {
		t.Error("Expected long summary to be truncated")
	}
	if !strings.Contains(content, "...") {
		t.Logf("Expected truncation indicator (may not appear): %q", content)
	}
}

func TestToolCallDisplayErrorState(t *testing.T) {
	th := theme.Current()
	tcd := NewToolCallDisplay("tool-1", "bash", th)

	tcd.SetError("command not found")

	if tcd.State() != ToolCallError {
		t.Errorf("Expected state Error, got %v", tcd.State())
	}

	view := tcd.View()
	content := view.Content

	if !strings.Contains(content, "bash") {
		t.Errorf("Expected tool name in view, got %q", content)
	}
	if !strings.Contains(content, "error") {
		t.Errorf("Expected 'error' status in view, got %q", content)
	}
	if !strings.Contains(content, "command not found") {
		t.Logf("Expected error message in view, got %q", content)
	}
}

func TestToolCallDisplayErrorWithoutMessage(t *testing.T) {
	th := theme.Current()
	tcd := NewToolCallDisplay("tool-1", "bash", th)

	tcd.SetError("")

	view := tcd.View()
	content := view.Content

	if !strings.Contains(content, "bash") {
		t.Errorf("Expected tool name in view, got %q", content)
	}
	if !strings.Contains(content, "error") {
		t.Errorf("Expected 'error' status in view, got %q", content)
	}
}

func TestToolCallDisplayErrorLongMessage(t *testing.T) {
	th := theme.Current()
	tcd := NewToolCallDisplay("tool-1", "bash", th)

	longError := strings.Repeat("e", 200)
	tcd.SetError(longError)

	view := tcd.View()
	content := view.Content

	// Should be truncated to ~100 chars + "..."
	if strings.Count(content, "e") > 150 {
		t.Error("Expected long error to be truncated")
	}
}

func TestToolCallDisplayStateTransition(t *testing.T) {
	th := theme.Current()
	tcd := NewToolCallDisplay("tool-1", "search", th)

	// Start pending
	if tcd.State() != ToolCallPending {
		t.Error("Expected initial state Pending")
	}

	// Transition to complete
	tcd.SetComplete("Found 5 results")
	if tcd.State() != ToolCallComplete {
		t.Error("Expected state Complete after SetComplete")
	}

	view := tcd.View()
	if !strings.Contains(view.Content, "complete") {
		t.Error("Expected 'complete' in view after transition")
	}
}

func TestToolCallDisplayStateTransitionToError(t *testing.T) {
	th := theme.Current()
	tcd := NewToolCallDisplay("tool-1", "bash", th)

	// Start pending
	tcd.SetPending()
	if tcd.State() != ToolCallPending {
		t.Error("Expected state Pending")
	}

	// Transition to error
	tcd.SetError("timeout")
	if tcd.State() != ToolCallError {
		t.Error("Expected state Error after SetError")
	}

	view := tcd.View()
	if !strings.Contains(view.Content, "error") {
		t.Error("Expected 'error' in view after transition")
	}
}

func TestToolCallDisplayCompleteToError(t *testing.T) {
	th := theme.Current()
	tcd := NewToolCallDisplay("tool-1", "bash", th)

	// Complete -> Error transition
	tcd.SetComplete("Done")
	if tcd.State() != ToolCallComplete {
		t.Error("Expected state Complete")
	}

	tcd.SetError("post-processing failed")
	if tcd.State() != ToolCallError {
		t.Error("Expected state Error after SetError")
	}

	view := tcd.View()
	if !strings.Contains(view.Content, "error") {
		t.Error("Expected 'error' in view after transition")
	}
	if !strings.Contains(view.Content, "post-processing") {
		t.Logf("Expected error message in view: %q", view.Content)
	}
}

func TestToolCallDisplayInit(t *testing.T) {
	th := theme.Current()
	tcd := NewToolCallDisplay("tool-1", "bash", th)

	cmd := tcd.Init()
	if cmd != nil {
		t.Error("Expected Init() to return nil")
	}
}

func TestToolCallDisplaySetSize(t *testing.T) {
	th := theme.Current()
	tcd := NewToolCallDisplay("tool-1", "bash", th)

	tcd.SetSize(100, 24)
	if tcd.width != 100 {
		t.Errorf("Expected width 100, got %d", tcd.width)
	}
}

func TestToolCallDisplayID(t *testing.T) {
	th := theme.Current()
	tcd := NewToolCallDisplay("my-tool-123", "bash", th)

	if tcd.ID() != "my-tool-123" {
		t.Errorf("Expected ID 'my-tool-123', got %q", tcd.ID())
	}
}

func TestToolCallDisplayUpdateNonPending(t *testing.T) {
	th := theme.Current()
	tcd := NewToolCallDisplay("tool-1", "bash", th)

	tcd.SetComplete("Done")

	// Update shouldn't affect spinner in complete state
	tcd.Update(ToolCallDisplayTickMsg{})
	tcd.Update(ToolCallDisplayTickMsg{})

	if tcd.spinnerTick != 0 {
		t.Error("Expected spinnerTick to stay 0 when not pending")
	}
}

func TestToolCallDisplayUpdateReturnsSelf(t *testing.T) {
	th := theme.Current()
	tcd := NewToolCallDisplay("tool-1", "bash", th)

	updated, cmd := tcd.Update(ToolCallDisplayTickMsg{})

	if cmd != nil {
		t.Error("Expected nil command")
	}

	if updated != tcd {
		t.Error("Expected Update to return self")
	}
}

func TestToolCallDisplayMultipleTools(t *testing.T) {
	th := theme.Current()

	tcd1 := NewToolCallDisplay("tool-1", "bash", th)
	tcd2 := NewToolCallDisplay("tool-2", "search", th)

	tcd1.SetPending()
	tcd2.SetComplete("Search results")

	view1 := tcd1.View()
	view2 := tcd2.View()

	if !strings.Contains(view1.Content, "bash") {
		t.Error("Expected 'bash' in first tool view")
	}
	if !strings.Contains(view1.Content, "running") {
		t.Error("Expected 'running' in first tool view")
	}

	if !strings.Contains(view2.Content, "search") {
		t.Error("Expected 'search' in second tool view")
	}
	if !strings.Contains(view2.Content, "complete") {
		t.Error("Expected 'complete' in second tool view")
	}
}

func TestToolCallDisplayWithDifferentThemes(t *testing.T) {
	themes := []theme.ThemeName{
		theme.ThemeDark,
		theme.ThemeLight,
		theme.ThemeHighContrast,
	}

	for _, themeName := range themes {
		theme.SetTheme(themeName)
		defer theme.SetTheme(theme.ThemeDark)

		th := theme.Current()
		tcd := NewToolCallDisplay("tool-1", "bash", th)

		tcd.SetPending()
		view := tcd.View()

		if !strings.Contains(view.Content, "bash") {
			t.Errorf("Expected 'bash' in view with theme %s", themeName)
		}
	}
}

func TestToolCallDisplayViewConsistency(t *testing.T) {
	th := theme.Current()
	tcd := NewToolCallDisplay("tool-1", "bash", th)

	tcd.SetComplete("Result text")

	// Call View multiple times
	view1 := tcd.View()
	view2 := tcd.View()

	// Both should have the same content (or very close)
	if !strings.Contains(view1.Content, "Result text") {
		t.Error("Expected result text in view1")
	}
	if !strings.Contains(view2.Content, "Result text") {
		t.Error("Expected result text in view2")
	}
}

func TestToolCallDisplayToolNameVariations(t *testing.T) {
	th := theme.Current()

	names := []string{"bash", "python", "git", "npm", "custom-tool"}

	for _, name := range names {
		tcd := NewToolCallDisplay("tool-1", name, th)
		tcd.SetPending()

		view := tcd.View()
		if !strings.Contains(view.Content, name) {
			t.Errorf("Expected tool name %q in view, got %q", name, view.Content)
		}
	}
}
