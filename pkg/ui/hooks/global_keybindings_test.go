package hooks

import "testing"

func TestToggleTodos_NoTeammates_BinaryToggle(t *testing.T) {
	g := NewGlobalKeybindings()
	if g.ExpandedView != ExpandedNone {
		t.Fatalf("initial = %q, want none", g.ExpandedView)
	}

	g.HandleToggleTodos()
	if g.ExpandedView != ExpandedTasks {
		t.Fatalf("after first toggle = %q, want tasks", g.ExpandedView)
	}

	g.HandleToggleTodos()
	if g.ExpandedView != ExpandedNone {
		t.Fatalf("after second toggle = %q, want none", g.ExpandedView)
	}
}

func TestToggleTodos_WithTeammates_ThreeStateCycle(t *testing.T) {
	g := NewGlobalKeybindings()
	g.HasRunningTeammates = true

	expected := []ExpandedView{ExpandedTasks, ExpandedTeammates, ExpandedNone, ExpandedTasks}
	for i, want := range expected {
		g.HandleToggleTodos()
		if g.ExpandedView != want {
			t.Fatalf("step %d: got %q, want %q", i, g.ExpandedView, want)
		}
	}
}

func TestToggleTranscript_PromptToTranscriptAndBack(t *testing.T) {
	g := NewGlobalKeybindings()
	var entered, exited int
	g.OnEnterTranscript = func() { entered++ }
	g.OnExitTranscript = func() { exited++ }

	g.HandleToggleTranscript()
	if g.Screen != ScreenTranscript {
		t.Fatalf("screen = %q, want transcript", g.Screen)
	}
	if entered != 1 {
		t.Fatalf("entered = %d, want 1", entered)
	}

	g.HandleToggleTranscript()
	if g.Screen != ScreenPrompt {
		t.Fatalf("screen = %q, want prompt", g.Screen)
	}
	if exited != 1 {
		t.Fatalf("exited = %d, want 1", exited)
	}
}

func TestToggleTranscript_BriefStuckEscapeHatch(t *testing.T) {
	g := NewGlobalKeybindings()
	g.IsBriefOnly = true
	g.BriefFeatureEnabled = false // kill-switch fired

	// Should clear brief-only instead of toggling screen.
	g.HandleToggleTranscript()
	if g.IsBriefOnly {
		t.Fatal("isBriefOnly should be cleared")
	}
	if g.Screen != ScreenPrompt {
		t.Fatalf("screen = %q, want prompt (escape hatch should not switch)", g.Screen)
	}
}

func TestToggleShowAll(t *testing.T) {
	g := NewGlobalKeybindings()
	if g.ShowAllInTranscript {
		t.Fatal("initial should be false")
	}

	g.HandleToggleShowAll()
	if !g.ShowAllInTranscript {
		t.Fatal("after toggle should be true")
	}

	g.HandleToggleShowAll()
	if g.ShowAllInTranscript {
		t.Fatal("after second toggle should be false")
	}
}

func TestExitTranscript_NotInTranscript_ReturnsFalse(t *testing.T) {
	g := NewGlobalKeybindings()
	if g.HandleExitTranscript() {
		t.Fatal("should return false when not in transcript")
	}
}

func TestExitTranscript_InTranscript_ReturnsTrue(t *testing.T) {
	g := NewGlobalKeybindings()
	g.Screen = ScreenTranscript
	var exited int
	g.OnExitTranscript = func() { exited++ }

	if !g.HandleExitTranscript() {
		t.Fatal("should return true when in transcript")
	}
	if g.Screen != ScreenPrompt {
		t.Fatalf("screen = %q, want prompt", g.Screen)
	}
	if exited != 1 {
		t.Fatalf("exited = %d, want 1", exited)
	}
}

func TestExitTranscript_BlockedByVirtualScroll(t *testing.T) {
	g := NewGlobalKeybindings()
	g.Screen = ScreenTranscript
	g.VirtualScrollActive = true

	if g.HandleExitTranscript() {
		t.Fatal("should return false when virtual scroll is active")
	}
}

func TestExitTranscript_BlockedBySearchBar(t *testing.T) {
	g := NewGlobalKeybindings()
	g.Screen = ScreenTranscript
	g.SearchBarOpen = true

	if g.HandleExitTranscript() {
		t.Fatal("should return false when search bar is open")
	}
}

func TestToggleBrief_AsymmetricGate(t *testing.T) {
	g := NewGlobalKeybindings()

	// Feature disabled: cannot turn ON.
	g.BriefFeatureEnabled = false
	g.HandleToggleBrief()
	if g.IsBriefOnly {
		t.Fatal("should not turn on when feature disabled")
	}

	// Feature enabled: can turn ON.
	g.BriefFeatureEnabled = true
	g.HandleToggleBrief()
	if !g.IsBriefOnly {
		t.Fatal("should turn on when feature enabled")
	}

	// OFF is always allowed, even if feature now disabled.
	g.BriefFeatureEnabled = false
	g.HandleToggleBrief()
	if g.IsBriefOnly {
		t.Fatal("should always be able to turn off")
	}
}

func TestToggleTodos_LogEvent(t *testing.T) {
	g := NewGlobalKeybindings()
	var events []string
	g.LogEvent = func(name string, _ map[string]any) { events = append(events, name) }

	g.HandleToggleTodos()
	if len(events) != 1 || events[0] != "tengu_toggle_todos" {
		t.Fatalf("events = %v, want [tengu_toggle_todos]", events)
	}
}

func TestToggleTranscript_LogEvent(t *testing.T) {
	g := NewGlobalKeybindings()
	var events []string
	g.LogEvent = func(name string, _ map[string]any) { events = append(events, name) }

	g.HandleToggleTranscript()
	if len(events) != 1 || events[0] != "tengu_toggle_transcript" {
		t.Fatalf("events = %v, want [tengu_toggle_transcript]", events)
	}
}
