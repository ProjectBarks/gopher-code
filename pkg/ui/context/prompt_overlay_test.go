package context

import "testing"

func TestPromptOverlay_Empty(t *testing.T) {
	var o PromptOverlay
	if o.IsActive() {
		t.Error("empty overlay should not be active")
	}
	if o.HasSuggestions() {
		t.Error("should have no suggestions")
	}
	if o.HasDialog() {
		t.Error("should have no dialog")
	}
	if o.SelectedSuggestion() != nil {
		t.Error("selected suggestion should be nil")
	}
}

func TestPromptOverlay_Suggestions(t *testing.T) {
	var o PromptOverlay
	items := []SuggestionItem{
		{Label: "/help", Description: "Show help", Value: "/help", Type: "command"},
		{Label: "/clear", Description: "Clear screen", Value: "/clear", Type: "command"},
		{Label: "/model", Description: "Change model", Value: "/model", Type: "command"},
	}

	o.SetSuggestions(items, 0)
	if !o.HasSuggestions() {
		t.Error("should have suggestions")
	}
	if !o.IsActive() {
		t.Error("should be active with suggestions")
	}
	if len(o.Suggestions) != 3 {
		t.Errorf("suggestion count = %d, want 3", len(o.Suggestions))
	}

	sel := o.SelectedSuggestion()
	if sel == nil || sel.Label != "/help" {
		t.Errorf("selected = %v, want /help", sel)
	}
}

func TestPromptOverlay_MoveSelection(t *testing.T) {
	var o PromptOverlay
	o.SetSuggestions([]SuggestionItem{
		{Label: "a"}, {Label: "b"}, {Label: "c"},
	}, 0)

	// Move down
	o.MoveSelection(1)
	if o.SelectedIndex != 1 {
		t.Errorf("after +1: index = %d, want 1", o.SelectedIndex)
	}

	// Move down again
	o.MoveSelection(1)
	if o.SelectedIndex != 2 {
		t.Errorf("after +1+1: index = %d, want 2", o.SelectedIndex)
	}

	// Wrap around forward
	o.MoveSelection(1)
	if o.SelectedIndex != 0 {
		t.Errorf("after wrap: index = %d, want 0", o.SelectedIndex)
	}

	// Wrap around backward
	o.MoveSelection(-1)
	if o.SelectedIndex != 2 {
		t.Errorf("after -1 wrap: index = %d, want 2", o.SelectedIndex)
	}
}

func TestPromptOverlay_MoveSelectionEmpty(t *testing.T) {
	var o PromptOverlay
	o.MoveSelection(1) // should not panic
	if o.SelectedIndex != 0 {
		t.Error("move on empty should be no-op")
	}
}

func TestPromptOverlay_ClearSuggestions(t *testing.T) {
	var o PromptOverlay
	o.SetSuggestions([]SuggestionItem{{Label: "x"}}, 0)
	o.ClearSuggestions()

	if o.HasSuggestions() {
		t.Error("should have no suggestions after clear")
	}
	if o.SelectedIndex != -1 {
		t.Errorf("index should be -1 after clear, got %d", o.SelectedIndex)
	}
}

func TestPromptOverlay_Dialog(t *testing.T) {
	var o PromptOverlay
	o.SetDialog("Enable auto-mode?")

	if !o.HasDialog() {
		t.Error("should have dialog")
	}
	if !o.IsActive() {
		t.Error("should be active with dialog")
	}
	if o.Dialog != "Enable auto-mode?" {
		t.Errorf("dialog = %q", o.Dialog)
	}

	o.ClearDialog()
	if o.HasDialog() {
		t.Error("should have no dialog after clear")
	}
}

func TestPromptOverlay_Clear(t *testing.T) {
	var o PromptOverlay
	o.SetSuggestions([]SuggestionItem{{Label: "x"}}, 0)
	o.SetDialog("test")

	o.Clear()
	if o.IsActive() {
		t.Error("should not be active after Clear()")
	}
}

func TestPromptOverlay_SelectedOutOfBounds(t *testing.T) {
	var o PromptOverlay
	o.SetSuggestions([]SuggestionItem{{Label: "a"}}, 5) // out of bounds
	if o.SelectedSuggestion() != nil {
		t.Error("out-of-bounds selection should return nil")
	}

	o.SelectedIndex = -1
	if o.SelectedSuggestion() != nil {
		t.Error("negative selection should return nil")
	}
}

func TestSuggestionItem_Fields(t *testing.T) {
	item := SuggestionItem{
		Label:       "/help",
		Description: "Show help",
		Value:       "/help",
		Type:        "command",
	}
	if item.Label != "/help" {
		t.Error("label wrong")
	}
	if item.Type != "command" {
		t.Error("type wrong")
	}
}
