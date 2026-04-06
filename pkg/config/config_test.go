package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadGlobalSettings(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".claude")
	os.MkdirAll(configDir, 0700)

	settingsJSON := `{
  "model": "claude-sonnet-4-20250514",
  "max_turns": 50,
  "permission_mode": "auto",
  "verbose": true,
  "hooks": [
    {"type": "PreToolUse", "matcher": "Bash", "command": "echo pre"}
  ]
}`
	os.WriteFile(filepath.Join(configDir, "settings.json"), []byte(settingsJSON), 0600)

	s := LoadFrom(filepath.Join(configDir, "settings.json"))

	if s.Model != "claude-sonnet-4-20250514" {
		t.Errorf("expected model claude-sonnet-4-20250514, got %s", s.Model)
	}
	if s.MaxTurns != 50 {
		t.Errorf("expected max_turns 50, got %d", s.MaxTurns)
	}
	if s.PermissionMode != "auto" {
		t.Errorf("expected permission_mode auto, got %s", s.PermissionMode)
	}
	if !s.Verbose {
		t.Error("expected verbose true")
	}
	if len(s.Hooks) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(s.Hooks))
	}
	if s.Hooks[0].Type != "PreToolUse" {
		t.Errorf("expected hook type PreToolUse, got %s", s.Hooks[0].Type)
	}
	if s.Hooks[0].Matcher != "Bash" {
		t.Errorf("expected hook matcher Bash, got %s", s.Hooks[0].Matcher)
	}
}

func TestLoadProjectOverride(t *testing.T) {
	tmpDir := t.TempDir()

	// Global settings
	globalDir := filepath.Join(tmpDir, "global", ".claude")
	os.MkdirAll(globalDir, 0700)
	globalJSON := `{"model": "global-model", "max_turns": 100, "verbose": true}`
	os.WriteFile(filepath.Join(globalDir, "settings.json"), []byte(globalJSON), 0600)

	// Project settings (overrides model)
	projectDir := filepath.Join(tmpDir, "project")
	projectConfigDir := filepath.Join(projectDir, ".claude")
	os.MkdirAll(projectConfigDir, 0700)
	projectJSON := `{"model": "project-model", "max_turns": 25}`
	os.WriteFile(filepath.Join(projectConfigDir, "settings.json"), []byte(projectJSON), 0600)

	// Load global first, then project on top
	s := &Settings{}
	loadInto(filepath.Join(globalDir, "settings.json"), s)
	loadInto(filepath.Join(projectConfigDir, "settings.json"), s)

	if s.Model != "project-model" {
		t.Errorf("expected project-model to override, got %s", s.Model)
	}
	if s.MaxTurns != 25 {
		t.Errorf("expected project max_turns 25 to override, got %d", s.MaxTurns)
	}
	// verbose should still be true from global since project didn't set it to false explicitly
	// (json.Unmarshal won't overwrite with zero value unless explicitly present)
	// Actually, since verbose is bool and not present in project JSON, it stays true.
	if !s.Verbose {
		t.Error("expected verbose to remain true from global")
	}
}

func TestLoadMissing(t *testing.T) {
	s := LoadFrom("/nonexistent/path/settings.json")

	// Should return defaults (zero values)
	if s.Model != "" {
		t.Errorf("expected empty model, got %s", s.Model)
	}
	if s.MaxTurns != 0 {
		t.Errorf("expected 0 max_turns, got %d", s.MaxTurns)
	}
	if len(s.Hooks) != 0 {
		t.Errorf("expected 0 hooks, got %d", len(s.Hooks))
	}
}

func TestSaveAndReload(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, ".claude", "settings.json")

	original := &Settings{
		Model:          "test-model",
		MaxTurns:       42,
		PermissionMode: "deny",
		AllowedTools:   []string{"Read", "Glob"},
		Hooks: []HookConfig{
			{Type: "PreToolUse", Matcher: "Bash", Command: "exit 0", Timeout: 10},
		},
		Verbose: true,
	}

	if err := original.SaveTo(path); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	reloaded := LoadFrom(path)

	if reloaded.Model != original.Model {
		t.Errorf("model: got %s, want %s", reloaded.Model, original.Model)
	}
	if reloaded.MaxTurns != original.MaxTurns {
		t.Errorf("max_turns: got %d, want %d", reloaded.MaxTurns, original.MaxTurns)
	}
	if reloaded.PermissionMode != original.PermissionMode {
		t.Errorf("permission_mode: got %s, want %s", reloaded.PermissionMode, original.PermissionMode)
	}
	if len(reloaded.AllowedTools) != len(original.AllowedTools) {
		t.Errorf("allowed_tools: got %d, want %d", len(reloaded.AllowedTools), len(original.AllowedTools))
	}
	if len(reloaded.Hooks) != 1 {
		t.Fatalf("hooks: got %d, want 1", len(reloaded.Hooks))
	}
	if reloaded.Hooks[0].Matcher != "Bash" {
		t.Errorf("hook matcher: got %s, want Bash", reloaded.Hooks[0].Matcher)
	}
	if reloaded.Hooks[0].Timeout != 10 {
		t.Errorf("hook timeout: got %d, want 10", reloaded.Hooks[0].Timeout)
	}
	if !reloaded.Verbose {
		t.Error("verbose: got false, want true")
	}
}

// ── T38: settings.enabledPlugins map field ────────────────────────────

func TestEnabledPlugins_LoadFromJSON(t *testing.T) {
	tmpDir := t.TempDir()
	settingsJSON := `{
  "enabledPlugins": {
    "my-plugin@builtin": true,
    "other-plugin@builtin": false
  }
}`
	path := filepath.Join(tmpDir, "settings.json")
	os.WriteFile(path, []byte(settingsJSON), 0600)

	s := LoadFrom(path)
	if s.EnabledPlugins == nil {
		t.Fatal("expected EnabledPlugins to be non-nil")
	}
	if !s.EnabledPlugins["my-plugin@builtin"] {
		t.Error("expected my-plugin@builtin to be true")
	}
	if s.EnabledPlugins["other-plugin@builtin"] {
		t.Error("expected other-plugin@builtin to be false")
	}
}

func TestEnabledPlugins_SaveAndReload(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "settings.json")

	original := &Settings{
		EnabledPlugins: map[string]bool{
			"test-plugin@builtin": true,
			"disabled@builtin":    false,
		},
	}
	if err := original.SaveTo(path); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	reloaded := LoadFrom(path)
	if len(reloaded.EnabledPlugins) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(reloaded.EnabledPlugins))
	}
	if !reloaded.EnabledPlugins["test-plugin@builtin"] {
		t.Error("expected test-plugin@builtin to be true")
	}
	if reloaded.EnabledPlugins["disabled@builtin"] {
		t.Error("expected disabled@builtin to be false")
	}
}

func TestEnabledPlugins_NilWhenAbsent(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "settings.json")
	os.WriteFile(path, []byte(`{"model": "test"}`), 0600)

	s := LoadFrom(path)
	if s.EnabledPlugins != nil {
		t.Errorf("expected nil EnabledPlugins when not in JSON, got %v", s.EnabledPlugins)
	}
}

func TestSaveCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "nested", "deep", "settings.json")

	s := &Settings{Model: "test"}
	if err := s.SaveTo(path); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("expected file to be created")
	}
}
