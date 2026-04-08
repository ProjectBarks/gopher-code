package keybindings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNewLoader_LoadsDefaults(t *testing.T) {
	l := NewLoader()
	bindings := l.Bindings()
	if len(bindings) == 0 {
		t.Error("should have default bindings")
	}
	// Check that ctrl+t -> app:tasks exists
	found := false
	for _, b := range bindings {
		if b.Action == "app:toggleTodos" && b.Context == "Global" {
			found = true
			break
		}
	}
	if !found {
		t.Error("default bindings should include app:toggleTodos in Global")
	}
}

func TestNewLoader_NoUserFile(t *testing.T) {
	// With no ~/.claude/keybindings.json, should load without warnings
	l := NewLoader()
	// May have warnings from other sources, but should not panic
	_ = l.Warnings()
}

func TestLoader_Reload(t *testing.T) {
	l := NewLoader()
	initial := len(l.Bindings())

	l.Reload()
	if len(l.Bindings()) != initial {
		t.Error("reload should produce same number of bindings")
	}
}

func TestLoader_OnChange(t *testing.T) {
	l := NewLoader()
	var called bool
	l.OnChange(func(result KeybindingsLoadResult) {
		called = true
		if len(result.Bindings) == 0 {
			t.Error("callback should receive bindings")
		}
	})
	l.Reload()
	if !called {
		t.Error("OnChange callback should be called on Reload")
	}
}

func TestLoadUserBindings_ValidFile(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, ".claude")
	os.MkdirAll(configDir, 0755)

	blocks := []KeybindingBlock{{
		Context:  Context("Global"),
		Bindings: map[string]string{"ctrl+y": "custom:yank"},
	}}
	data, _ := json.Marshal(blocks)
	os.WriteFile(filepath.Join(configDir, "keybindings.json"), data, 0644)

	// Temporarily override HOME
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	defer os.Setenv("HOME", origHome)

	bindings, warnings := loadUserBindings()
	if len(bindings) != 1 {
		t.Fatalf("expected 1 user binding, got %d", len(bindings))
	}
	if bindings[0].Action != "custom:yank" {
		t.Errorf("action = %q, want custom:yank", bindings[0].Action)
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %d", len(warnings))
	}
}

func TestLoadUserBindings_InvalidContext(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, ".claude")
	os.MkdirAll(configDir, 0755)

	blocks := []KeybindingBlock{{
		Context:  Context("FakeContext"),
		Bindings: map[string]string{"ctrl+x": "fake:action"},
	}}
	data, _ := json.Marshal(blocks)
	os.WriteFile(filepath.Join(configDir, "keybindings.json"), data, 0644)

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	defer os.Setenv("HOME", origHome)

	_, warnings := loadUserBindings()
	if len(warnings) == 0 {
		t.Error("should warn about invalid context")
	}
}

func TestParseChord_MultiKey(t *testing.T) {
	chord := ParseChord("ctrl+k ctrl+s")
	if len(chord) != 2 {
		t.Fatalf("chord should have 2 keystrokes, got %d", len(chord))
	}
	if chord[0].Key != "k" || !chord[0].Ctrl {
		t.Error("first keystroke should be ctrl+k")
	}
	if chord[1].Key != "s" || !chord[1].Ctrl {
		t.Error("second keystroke should be ctrl+s")
	}
}

func TestMergeBindings(t *testing.T) {
	defaults := []ParsedBinding{{Action: "default:a", Context: "Global"}}
	user := []ParsedBinding{{Action: "user:b", Context: "Global"}}
	merged := mergeBindings(defaults, user)
	if len(merged) != 2 {
		t.Fatalf("merged should have 2, got %d", len(merged))
	}
	// User bindings should come after defaults (last-wins)
	if merged[1].Action != "user:b" {
		t.Error("user binding should be last")
	}
}
