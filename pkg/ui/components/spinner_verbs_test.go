package components

import (
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

func TestSpinnerVerbsCount(t *testing.T) {
	if len(SpinnerVerbs) < 180 {
		t.Errorf("Expected 180+ spinner verbs, got %d", len(SpinnerVerbs))
	}
}

func TestSpinnerGlyphsCount(t *testing.T) {
	if len(SpinnerGlyphs) != 6 {
		t.Errorf("Expected 6 spinner glyphs, got %d", len(SpinnerGlyphs))
	}
}

func TestSpinnerGlyphsContainExpectedChars(t *testing.T) {
	expected := []string{"·", "✢", "✳", "✶", "✻", "✽"}
	for i, exp := range expected {
		if SpinnerGlyphs[i] != exp {
			t.Errorf("Glyph %d: expected %q, got %q", i, exp, SpinnerGlyphs[i])
		}
	}
}

func TestSpinnerTipsNotEmpty(t *testing.T) {
	if len(SpinnerTips) == 0 {
		t.Error("SpinnerTips should not be empty")
	}
}

func TestThinkingSpinnerCreation(t *testing.T) {
	ts := NewThinkingSpinner(theme.Current())
	if ts == nil {
		t.Fatal("ThinkingSpinner should not be nil")
	}
	if ts.Verb() == "" {
		t.Error("Should have a random verb")
	}
	if ts.Tip() == "" {
		t.Error("Should have a random tip")
	}
}

func TestThinkingSpinnerStart(t *testing.T) {
	ts := NewThinkingSpinner(theme.Current())
	ts.Start()
	if !ts.IsActive() {
		t.Error("Should be active after Start")
	}
}

func TestThinkingSpinnerStop(t *testing.T) {
	ts := NewThinkingSpinner(theme.Current())
	ts.Start()
	ts.Stop()
	if ts.IsActive() {
		t.Error("Should not be active after Stop")
	}
}

func TestThinkingSpinnerViewActive(t *testing.T) {
	ts := NewThinkingSpinner(theme.Current())
	ts.Start()
	view := ts.View()
	plain := stripANSI(view)
	// Should contain the verb and "thinking"
	if !strings.Contains(plain, ts.Verb()) {
		t.Errorf("Expected verb %q in view, got %q", ts.Verb(), plain)
	}
	if !strings.Contains(plain, "thinking") {
		t.Errorf("Expected 'thinking' in view, got %q", plain)
	}
}

func TestThinkingSpinnerViewComplete(t *testing.T) {
	ts := NewThinkingSpinner(theme.Current())
	ts.Start()
	ts.Stop()
	view := ts.View()
	plain := stripANSI(view)
	// Should contain one of the turn completion verbs + "for Xs"
	// Source: constants/turnCompletionVerbs.ts
	hasVerb := false
	for _, v := range TurnCompletionVerbs {
		if strings.Contains(plain, v+" for") {
			hasVerb = true
			break
		}
	}
	if !hasVerb {
		t.Errorf("Expected a turn completion verb in complete view, got %q", plain)
	}
}

func TestTurnCompletionVerbsCount(t *testing.T) {
	if len(TurnCompletionVerbs) != 8 {
		t.Errorf("Expected 8 turn completion verbs, got %d", len(TurnCompletionVerbs))
	}
}

func TestThinkingSpinnerTipView(t *testing.T) {
	ts := NewThinkingSpinner(theme.Current())
	view := ts.TipView()
	plain := stripANSI(view)
	if !strings.Contains(plain, "Tip:") {
		t.Errorf("Expected 'Tip:' in tip view, got %q", plain)
	}
	if !strings.Contains(plain, "⎿") {
		t.Error("Expected ⎿ connector in tip view")
	}
}

func TestThinkingSpinnerEffort(t *testing.T) {
	ts := NewThinkingSpinner(theme.Current())
	ts.Start()

	ts.SetEffort("low")
	view := stripANSI(ts.View())
	if !strings.Contains(view, EffortLow) {
		t.Errorf("Expected low effort icon %q in view", EffortLow)
	}

	ts.SetEffort("high")
	view = stripANSI(ts.View())
	if !strings.Contains(view, EffortHigh) {
		t.Errorf("Expected high effort icon %q in view", EffortHigh)
	}

	ts.SetEffort("max")
	view = stripANSI(ts.View())
	if !strings.Contains(view, EffortMax) {
		t.Errorf("Expected max effort icon %q in view", EffortMax)
	}
}

func TestThinkingSpinnerTickAdvancesFrame(t *testing.T) {
	ts := NewThinkingSpinner(theme.Current())
	ts.Start()
	initialFrame := ts.Frame()
	ts.Update(SpinnerTickMsg{})
	if ts.Frame() == initialFrame {
		t.Error("Tick should advance the frame")
	}
}

func TestThinkingSpinnerFrameWraps(t *testing.T) {
	ts := NewThinkingSpinner(theme.Current())
	ts.Start()
	for i := 0; i < len(SpinnerGlyphs)+1; i++ {
		ts.Update(SpinnerTickMsg{})
	}
	// Should wrap around without panic
	if ts.Frame() >= len(SpinnerGlyphs) {
		t.Errorf("Frame should wrap, got %d", ts.Frame())
	}
}

func TestThinkingSpinnerRandomVerbs(t *testing.T) {
	// Create multiple spinners and verify they don't all have the same verb
	verbs := make(map[string]bool)
	for i := 0; i < 20; i++ {
		ts := NewThinkingSpinner(theme.Current())
		verbs[ts.Verb()] = true
	}
	if len(verbs) < 2 {
		t.Error("Expected different verbs across multiple spinners")
	}
}

func TestEffortConstants(t *testing.T) {
	if EffortLow != "○" {
		t.Error("EffortLow should be ○")
	}
	if EffortMedium != "◐" {
		t.Error("EffortMedium should be ◐")
	}
	if EffortHigh != "●" {
		t.Error("EffortHigh should be ●")
	}
	if EffortMax != "◉" {
		t.Error("EffortMax should be ◉")
	}
}
