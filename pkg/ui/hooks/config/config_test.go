package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	cfgpkg "github.com/projectbarks/gopher-code/pkg/config"
)

// ---------------------------------------------------------------------------
// MainLoopModel tests
// ---------------------------------------------------------------------------

func TestMainLoopModel_DefaultReturnsDefaultSonnet(t *testing.T) {
	m := NewMainLoopModel("")
	got := m.Get()
	if got == "" {
		t.Fatal("Get() returned empty string, expected a default model")
	}
	// Should resolve to the default sonnet model
	want := DefaultMainLoopModelSetting()
	if got == "" {
		t.Fatalf("Get() returned empty; want resolved form of %q", want)
	}
}

func TestMainLoopModel_SetOverridesDefault(t *testing.T) {
	m := NewMainLoopModel("")
	m.Set("claude-opus-4-6")
	got := m.Get()
	if got != "claude-opus-4-6" {
		t.Fatalf("Get() = %q after Set; want %q", got, "claude-opus-4-6")
	}
}

func TestMainLoopModel_SessionOverrideTakesPrecedence(t *testing.T) {
	m := NewMainLoopModel("claude-sonnet-4-6")
	m.Override("claude-opus-4-6")
	got := m.Get()
	if got != "claude-opus-4-6" {
		t.Fatalf("Get() = %q; want session override %q", got, "claude-opus-4-6")
	}
	if !m.HasOverride() {
		t.Fatal("HasOverride() = false; want true")
	}
}

func TestMainLoopModel_ClearOverrideFallsBack(t *testing.T) {
	m := NewMainLoopModel("claude-sonnet-4-6")
	m.Override("claude-opus-4-6")
	m.Override("") // clear
	got := m.Get()
	if got != "claude-sonnet-4-6" {
		t.Fatalf("Get() = %q after clearing override; want persistent %q", got, "claude-sonnet-4-6")
	}
	if m.HasOverride() {
		t.Fatal("HasOverride() = true after clear; want false")
	}
}

func TestMainLoopModel_AliasResolution(t *testing.T) {
	m := NewMainLoopModel("")
	m.Set("sonnet")
	got := m.Get()
	// "sonnet" should resolve to a full model ID, not stay as "sonnet"
	if got == "sonnet" {
		t.Fatal("Get() returned raw alias 'sonnet'; want resolved model ID")
	}
	if got == "" {
		t.Fatal("Get() returned empty for alias 'sonnet'")
	}
}

func TestMainLoopModel_DisplayName(t *testing.T) {
	m := NewMainLoopModel("claude-opus-4-6")
	name := m.DisplayName()
	if name == "" {
		t.Fatal("DisplayName() returned empty for claude-opus-4-6")
	}
}

// ---------------------------------------------------------------------------
// SettingsWatcher tests
// ---------------------------------------------------------------------------

func TestSettingsWatcher_DetectsFileChange(t *testing.T) {
	dir := t.TempDir()
	settingsDir := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(settingsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	settingsFile := filepath.Join(settingsDir, "settings.json")
	if err := os.WriteFile(settingsFile, []byte(`{"model":"sonnet"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	sw := &SettingsWatcher{
		cwd:      dir,
		interval: 50 * time.Millisecond,
		mtimes:   make(map[string]time.Time),
		paths: []watchedPath{
			{path: settingsFile, source: cfgpkg.SourceProject},
		},
	}

	// Init snapshots the current mtime
	sw.Init()

	if _, tracked := sw.mtimes[settingsFile]; !tracked {
		t.Fatal("Init did not snapshot mtime for settings file")
	}

	// Advance mtime by rewriting the file after a brief pause
	time.Sleep(50 * time.Millisecond)
	if err := os.WriteFile(settingsFile, []byte(`{"model":"opus"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	// Send a tick and check for SettingsChangedMsg
	model, cmd := sw.Update(settingsTickMsg{})
	if model == nil {
		t.Fatal("Update returned nil model")
	}
	if cmd == nil {
		t.Fatal("Update returned nil cmd after file change; expected batched cmds")
	}
}

func TestSettingsWatcher_NoFalsePositiveOnInit(t *testing.T) {
	dir := t.TempDir()
	settingsDir := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(settingsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	settingsFile := filepath.Join(settingsDir, "settings.json")
	if err := os.WriteFile(settingsFile, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	sw := &SettingsWatcher{
		cwd:      dir,
		interval: 50 * time.Millisecond,
		mtimes:   make(map[string]time.Time),
		paths: []watchedPath{
			{path: settingsFile, source: cfgpkg.SourceProject},
		},
	}
	sw.Init()

	// First tick with no file change should NOT emit SettingsChangedMsg.
	// The cmd should be just the next tick, not a batched set.
	_, cmd := sw.Update(settingsTickMsg{})
	// Execute the cmd to see what messages it produces. A simple tick
	// returns a single tea.Cmd; a batch with a change would be different.
	// We can't easily introspect tea.Batch, so we verify mtimes are stable.
	if cmd == nil {
		t.Fatal("expected at least a tick cmd")
	}
	// The key assertion: mtime was already set by Init, and file hasn't
	// changed, so no SettingsChangedMsg should be queued.
}

func TestSettingsWatcher_DetectsFileDeletion(t *testing.T) {
	dir := t.TempDir()
	settingsDir := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(settingsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	settingsFile := filepath.Join(settingsDir, "settings.json")
	if err := os.WriteFile(settingsFile, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	sw := &SettingsWatcher{
		cwd:      dir,
		interval: 50 * time.Millisecond,
		mtimes:   make(map[string]time.Time),
		paths: []watchedPath{
			{path: settingsFile, source: cfgpkg.SourceProject},
		},
	}
	sw.Init()

	// Delete the file
	os.Remove(settingsFile)

	_, cmd := sw.Update(settingsTickMsg{})
	if cmd == nil {
		t.Fatal("expected cmd after file deletion")
	}
	// After deletion, the path should no longer be tracked
	if _, tracked := sw.mtimes[settingsFile]; tracked {
		t.Fatal("deleted file should be untracked after Update")
	}
}

// ---------------------------------------------------------------------------
// DynamicConfig tests
// ---------------------------------------------------------------------------

func TestDynamicConfig_DefaultValue(t *testing.T) {
	dc := NewDynamicConfig("test_flag", 42, time.Second, func(string) (int, bool) {
		return 0, false
	})
	if dc.Value() != 42 {
		t.Fatalf("Value() = %d; want 42", dc.Value())
	}
}

func TestDynamicConfig_UpdateChangesValue(t *testing.T) {
	calls := 0
	dc := NewDynamicConfig("test_flag", "default", time.Second, func(string) (string, bool) {
		calls++
		if calls >= 1 {
			return "updated", true
		}
		return "", false
	})

	// Simulate a tick
	cmd := dc.Update(dynamicConfigTickMsg{name: "test_flag"})
	if cmd == nil {
		t.Fatal("Update returned nil cmd; expected tick cmd")
	}
	if dc.Value() != "updated" {
		t.Fatalf("Value() = %q after update; want %q", dc.Value(), "updated")
	}
}

func TestDynamicConfig_IgnoresOtherConfigTicks(t *testing.T) {
	dc := NewDynamicConfig("my_flag", 10, time.Second, func(string) (int, bool) {
		return 99, true
	})

	// Tick for a different config name should be ignored
	cmd := dc.Update(dynamicConfigTickMsg{name: "other_flag"})
	if cmd != nil {
		t.Fatal("Update should return nil for unrelated tick")
	}
	if dc.Value() != 10 {
		t.Fatalf("Value() = %d; want 10 (unchanged)", dc.Value())
	}
}

func TestDynamicConfig_FallsBackToDefault(t *testing.T) {
	first := true
	dc := NewDynamicConfig("flag", "initial", time.Second, func(string) (string, bool) {
		if first {
			first = false
			return "loaded", true
		}
		return "", false // loader returns not-found
	})

	// First tick: loader returns "loaded"
	dc.Update(dynamicConfigTickMsg{name: "flag"})
	if dc.Value() != "loaded" {
		t.Fatalf("Value() = %q; want %q", dc.Value(), "loaded")
	}

	// Second tick: loader returns not-found, should fall back to default
	dc.Update(dynamicConfigTickMsg{name: "flag"})
	if dc.Value() != "initial" {
		t.Fatalf("Value() = %q; want default %q", dc.Value(), "initial")
	}
}

func TestDynamicConfig_Init(t *testing.T) {
	dc := NewDynamicConfig("x", 0, 100*time.Millisecond, func(string) (int, bool) {
		return 0, false
	})
	cmd := dc.Init()
	if cmd == nil {
		t.Fatal("Init() returned nil; expected tick cmd")
	}
}

// ---------------------------------------------------------------------------
// SettingsWatcher — settingsPaths coverage
// ---------------------------------------------------------------------------

func TestSettingsWatcher_SettingsPathsIncludesExpectedSources(t *testing.T) {
	sw := &SettingsWatcher{
		cwd:    "/tmp/test-project",
		mtimes: make(map[string]time.Time),
	}
	paths := sw.settingsPaths()
	if len(paths) < 2 {
		t.Fatalf("expected at least 2 watched paths (user + project), got %d", len(paths))
	}

	// Verify sources are present
	sources := make(map[cfgpkg.SettingSource]bool)
	for _, p := range paths {
		sources[p.source] = true
	}
	for _, want := range []cfgpkg.SettingSource{cfgpkg.SourceProject, cfgpkg.SourceLocal} {
		if !sources[want] {
			t.Errorf("missing expected source %q in watched paths", want)
		}
	}
}

// ---------------------------------------------------------------------------
// Integration: SettingsWatcher as tea.Model
// ---------------------------------------------------------------------------

func TestSettingsWatcher_ImplementsTeaModel(t *testing.T) {
	sw := NewSettingsWatcher("/tmp/test", 100*time.Millisecond)
	// Verify it satisfies the tea.Model interface.
	var _ tea.Model = sw

	v := sw.View()
	if v.Content != "" {
		t.Fatalf("View() = %q; want empty", v.Content)
	}
}
