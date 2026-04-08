package main

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/projectbarks/gopher-code/pkg/permissions"
	"github.com/projectbarks/gopher-code/pkg/ui"
	"github.com/projectbarks/gopher-code/pkg/ui/hooks"
)

// TestStandaloneHooks_WiredThroughAppModel verifies that the 6 standalone hook
// types from pkg/ui/hooks are wired into the AppModel and reachable from the
// binary. This exercises the real code path: main.go -> ui.NewAppModel ->
// initStandaloneHooks.
func TestStandaloneHooks_WiredThroughAppModel(t *testing.T) {
	app := ui.NewAppModel(nil, nil)

	if app.TerminalSizeTracker() == nil {
		t.Fatal("expected non-nil TerminalSizeTracker from AppModel")
	}
	if app.GlobalKeybindings() == nil {
		t.Fatal("expected non-nil GlobalKeybindings from AppModel")
	}
	if app.MergedTools() == nil {
		t.Fatal("expected non-nil MergedTools from AppModel")
	}
	if app.ApiKeyVerification() == nil {
		t.Fatal("expected non-nil ApiKeyVerification from AppModel")
	}
	if app.InteractionTracker() == nil {
		t.Fatal("expected non-nil InteractionTracker from AppModel")
	}
	if app.UpdateNotificationTracker() == nil {
		t.Fatal("expected non-nil UpdateNotification from AppModel")
	}
}

// TestStandaloneHooks_TerminalSizeUpdatesOnResize verifies that the terminal
// size tracker is updated when AppModel processes a WindowSizeMsg.
func TestStandaloneHooks_TerminalSizeUpdatesOnResize(t *testing.T) {
	app := ui.NewAppModel(nil, nil)

	// Initial size is 0x0 (no WindowSizeMsg yet).
	tracker := app.TerminalSizeTracker()
	if tracker.Width() != 0 || tracker.Height() != 0 {
		t.Errorf("initial size = %dx%d, want 0x0", tracker.Width(), tracker.Height())
	}

	// Send a resize through the real Update path.
	model, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	updated := model.(*ui.AppModel)

	tracker = updated.TerminalSizeTracker()
	if tracker.Width() != 120 || tracker.Height() != 40 {
		t.Errorf("after resize size = %dx%d, want 120x40",
			tracker.Width(), tracker.Height())
	}
}

// TestStandaloneHooks_GlobalKeybindingsToggleTodos verifies the global
// keybindings state machine works through the AppModel accessor.
func TestStandaloneHooks_GlobalKeybindingsToggleTodos(t *testing.T) {
	app := ui.NewAppModel(nil, nil)
	gk := app.GlobalKeybindings()

	if gk.ExpandedView != hooks.ExpandedNone {
		t.Fatalf("initial expanded view = %q, want %q", gk.ExpandedView, hooks.ExpandedNone)
	}

	gk.HandleToggleTodos()
	if gk.ExpandedView != hooks.ExpandedTasks {
		t.Errorf("after toggle, expanded view = %q, want %q",
			gk.ExpandedView, hooks.ExpandedTasks)
	}

	gk.HandleToggleTodos()
	if gk.ExpandedView != hooks.ExpandedNone {
		t.Errorf("after second toggle, expanded view = %q, want %q",
			gk.ExpandedView, hooks.ExpandedNone)
	}
}

// TestStandaloneHooks_MergedToolsDefaultMode verifies the merged tools pool
// starts with the correct default permission mode.
func TestStandaloneHooks_MergedToolsDefaultMode(t *testing.T) {
	app := ui.NewAppModel(nil, nil)
	mt := app.MergedTools()

	// Set some initial tools and verify they come back.
	mt.SetInitialTools([]hooks.ToolEntry{
		{Name: "read_file", Source: hooks.ToolSourceBuiltIn, ReadOnly: true},
		{Name: "write_file", Source: hooks.ToolSourceBuiltIn, ReadOnly: false},
	})

	tools := mt.Tools()
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools in default mode, got %d", len(tools))
	}

	// Switch to plan mode — only read-only tools should remain.
	mt.SetMode(permissions.ModePlan)
	tools = mt.Tools()
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool in plan mode, got %d", len(tools))
	}
	if tools[0].Name != "read_file" {
		t.Errorf("plan mode tool = %q, want read_file", tools[0].Name)
	}
}

// TestStandaloneHooks_ApiKeyVerificationInitialStatus verifies the API key
// verification state machine initializes correctly through the AppModel.
func TestStandaloneHooks_ApiKeyVerificationInitialStatus(t *testing.T) {
	app := ui.NewAppModel(nil, nil)
	akv := app.ApiKeyVerification()

	// With auth disabled (default stub), initial status should be valid.
	if akv.Status() != hooks.VerificationValid {
		t.Errorf("initial verification status = %q, want %q",
			akv.Status(), hooks.VerificationValid)
	}
}

// TestStandaloneHooks_InteractionTrackerTouch verifies the interaction tracker
// is wired and functional through the AppModel.
func TestStandaloneHooks_InteractionTrackerTouch(t *testing.T) {
	app := ui.NewAppModel(nil, nil)
	tracker := app.InteractionTracker()

	tracker.Touch()
	elapsed := tracker.SinceLastInteraction()
	if elapsed < 0 {
		t.Errorf("SinceLastInteraction = %v, expected >= 0", elapsed)
	}
}

// TestStandaloneHooks_UpdateNotificationCheck verifies the update notification
// deduplicator works through the AppModel accessor.
func TestStandaloneHooks_UpdateNotificationCheck(t *testing.T) {
	app := ui.NewAppModel(nil, nil)
	un := app.UpdateNotificationTracker()

	// First check with a new version should return the version.
	result := un.Check("1.2.3")
	if result != "1.2.3" {
		t.Errorf("first Check = %q, want %q", result, "1.2.3")
	}

	// Same version again should return empty (deduplicated).
	result = un.Check("1.2.3")
	if result != "" {
		t.Errorf("duplicate Check = %q, want empty", result)
	}
}
