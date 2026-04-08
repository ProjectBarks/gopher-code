package components

import (
	"slices"
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

func TestTurnCompletionVerbsSlice(t *testing.T) {
	if len(TurnCompletionVerbs) != 8 {
		t.Fatalf("expected 8 turn completion verbs, got %d", len(TurnCompletionVerbs))
	}
	if !slices.Contains(TurnCompletionVerbs, "Sautéed") {
		t.Fatal("missing accented verb Sautéed")
	}
}

// TestTurnCompletionVerbs_IntegrationThroughSpinner verifies that TurnCompletionVerbs
// are used in the real code path: ThinkingSpinner.View() renders a random turn
// completion verb when the spinner is stopped (completed state).
func TestTurnCompletionVerbs_IntegrationThroughSpinner(t *testing.T) {
	ts := NewThinkingSpinner(theme.Current())
	ts.Start()
	ts.Stop()

	view := ts.View()
	plain := stripANSI(view)

	// The completed view must contain one of the TurnCompletionVerbs followed by "for".
	found := false
	for _, verb := range TurnCompletionVerbs {
		if strings.Contains(plain, verb+" for") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("completed spinner view should contain a turn completion verb, got %q", plain)
	}
}
