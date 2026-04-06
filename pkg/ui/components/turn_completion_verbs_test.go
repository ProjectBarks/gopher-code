package components

import (
	"slices"
	"testing"
)

func TestTurnCompletionVerbsSlice(t *testing.T) {
	if len(TurnCompletionVerbs) != 8 {
		t.Fatalf("expected 8 turn completion verbs, got %d", len(TurnCompletionVerbs))
	}
	if !slices.Contains(TurnCompletionVerbs, "Sautéed") {
		t.Fatal("missing accented verb Sautéed")
	}
}
