package keybindings

import (
	"strings"
	"testing"
)

func TestValidateKeystroke_Valid(t *testing.T) {
	for _, ks := range []string{"ctrl+c", "alt+f", "escape", "enter", "a", "ctrl+shift+k"} {
		if w := ValidateKeystroke(ks); w != nil {
			t.Errorf("ValidateKeystroke(%q) returned warning: %s", ks, w.Message)
		}
	}
}

func TestValidateKeystroke_EmptyPart(t *testing.T) {
	w := ValidateKeystroke("ctrl++c")
	if w == nil {
		t.Fatal("expected warning for empty part")
	}
	if w.Type != WarnParseError {
		t.Errorf("type = %s, want parse_error", w.Type)
	}
}

func TestValidateContext_Valid(t *testing.T) {
	if w := ValidateContext("Global"); w != nil {
		t.Errorf("Global should be valid, got: %s", w.Message)
	}
}

func TestValidateContext_Invalid(t *testing.T) {
	w := ValidateContext("FakeContext")
	if w == nil {
		t.Fatal("expected warning for invalid context")
	}
	if w.Type != WarnInvalidContext {
		t.Errorf("type = %s, want invalid_context", w.Type)
	}
	if !strings.Contains(w.Suggestion, "Global") {
		t.Error("suggestion should list valid contexts")
	}
}

func TestCheckDuplicates(t *testing.T) {
	bindings := []ParsedBinding{
		{Chord: Chord{ParseKeystroke("ctrl+t")}, Action: "app:tasks", Context: "Global"},
		{Chord: Chord{ParseKeystroke("ctrl+t")}, Action: "app:other", Context: "Global"},
	}
	warnings := CheckDuplicates(bindings)
	if len(warnings) != 1 {
		t.Fatalf("expected 1 duplicate warning, got %d", len(warnings))
	}
	if warnings[0].Type != WarnDuplicate {
		t.Errorf("type = %s, want duplicate", warnings[0].Type)
	}
}

func TestCheckDuplicates_DifferentContextsOK(t *testing.T) {
	bindings := []ParsedBinding{
		{Chord: Chord{ParseKeystroke("enter")}, Action: "chat:submit", Context: "Chat"},
		{Chord: Chord{ParseKeystroke("enter")}, Action: "confirm:yes", Context: "Confirmation"},
	}
	warnings := CheckDuplicates(bindings)
	if len(warnings) != 0 {
		t.Errorf("same key in different contexts should not warn, got %d warnings", len(warnings))
	}
}

func TestCheckReservedShortcuts(t *testing.T) {
	bindings := []ParsedBinding{
		{Chord: Chord{ParseKeystroke("ctrl+c")}, Action: "custom:thing", Context: "Global"},
	}
	warnings := CheckReservedShortcuts(bindings)
	if len(warnings) != 1 {
		t.Fatalf("expected 1 reserved warning, got %d", len(warnings))
	}
	if warnings[0].Type != WarnReserved {
		t.Errorf("type = %s, want reserved", warnings[0].Type)
	}
}

func TestValidateBindings_Deduplicates(t *testing.T) {
	// Same duplicate twice should only produce one warning
	bindings := []ParsedBinding{
		{Chord: Chord{ParseKeystroke("ctrl+c")}, Action: "a", Context: "Global"},
		{Chord: Chord{ParseKeystroke("ctrl+c")}, Action: "b", Context: "Global"},
	}
	warnings := ValidateBindings(bindings)
	// Should have duplicate + reserved but deduplicated
	typeCount := map[WarningType]int{}
	for _, w := range warnings {
		typeCount[w.Type]++
	}
	if typeCount[WarnDuplicate] > 1 {
		t.Errorf("duplicate warnings should be deduplicated, got %d", typeCount[WarnDuplicate])
	}
}

func TestFormatWarnings(t *testing.T) {
	warnings := []Warning{
		{Type: WarnParseError, Severity: SeverityError, Message: "bad key"},
		{Type: WarnDuplicate, Severity: SeverityWarning, Message: "dupe", Suggestion: "fix it"},
	}
	output := FormatWarnings(warnings)
	if !strings.Contains(output, "✗") {
		t.Error("should contain error icon")
	}
	if !strings.Contains(output, "⚠") {
		t.Error("should contain warning icon")
	}
	if !strings.Contains(output, "fix it") {
		t.Error("should contain suggestion")
	}
}
