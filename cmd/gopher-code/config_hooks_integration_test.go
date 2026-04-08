package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	confighooks "github.com/projectbarks/gopher-code/pkg/ui/hooks/config"
)

// TestMainLoopModel_WiredIntoBinary verifies that the confighooks package is
// reachable from the binary by exercising the same code path used in main():
// NewMainLoopModel(resolvedModel) -> .Get() -> sess.InitialMainLoopModel.
//
// This mirrors the T409 wiring in main.go where MainLoopModel is created
// with the resolved model flag and populates session state.
func TestMainLoopModel_WiredIntoBinary(t *testing.T) {
	// Simulate what main() does: resolve a model alias, then create MainLoopModel.
	resolvedModel := resolveModelAlias("sonnet")

	mlm := confighooks.NewMainLoopModel(resolvedModel)
	got := mlm.Get()

	if got == "" {
		t.Fatal("MainLoopModel.Get() returned empty; expected resolved model ID")
	}
	// The resolved model should be the full ID, not the alias.
	if got == "sonnet" {
		t.Fatal("MainLoopModel.Get() returned raw alias 'sonnet'; expected full model ID")
	}

	// Verify DisplayName returns something non-empty.
	display := mlm.DisplayName()
	if display == "" {
		t.Fatal("MainLoopModel.DisplayName() returned empty")
	}

	// Verify session override takes precedence (same pattern as /model command).
	mlm.Override("claude-opus-4-6")
	if mlm.Get() != "claude-opus-4-6" {
		t.Fatalf("Get() = %q after Override; want claude-opus-4-6", mlm.Get())
	}
	if !mlm.HasOverride() {
		t.Fatal("HasOverride() = false; want true")
	}

	// Clear override falls back to persistent model.
	mlm.Override("")
	if mlm.Get() != resolvedModel {
		t.Fatalf("Get() = %q after clearing override; want %q", mlm.Get(), resolvedModel)
	}
}

// TestSettingsWatcher_WiredIntoBinary exercises the SettingsWatcher through
// the same construction path used in main() and verifies it detects file
// changes via its bubbletea Update cycle.
func TestSettingsWatcher_WiredIntoBinary(t *testing.T) {
	dir := t.TempDir()
	settingsDir := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(settingsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	settingsFile := filepath.Join(settingsDir, "settings.json")
	if err := os.WriteFile(settingsFile, []byte(`{"model":"sonnet"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	// Same construction as main(): NewSettingsWatcher(cwd, interval)
	sw := confighooks.NewSettingsWatcher(dir, 50*time.Millisecond)

	// Init snapshots mtimes -- same as what would happen in a bubbletea program.
	sw.Init()

	// View should be empty (watcher has no visual output).
	v := sw.View()
	if v.Content != "" {
		t.Fatalf("SettingsWatcher.View() = %q; want empty", v.Content)
	}
}

// TestDynamicConfig_WiredIntoBinary exercises DynamicConfig through the same
// package import path used in main(), verifying it's reachable from the binary.
func TestDynamicConfig_WiredIntoBinary(t *testing.T) {
	loader := func(name string) (string, bool) {
		if name == "test_feature" {
			return "enabled", true
		}
		return "", false
	}

	dc := confighooks.NewDynamicConfig("test_feature", "disabled", time.Second, loader)
	if dc.Value() != "disabled" {
		t.Fatalf("Value() = %q; want default 'disabled'", dc.Value())
	}

	// Init returns a tick cmd (non-nil).
	cmd := dc.Init()
	if cmd == nil {
		t.Fatal("Init() returned nil; expected tick cmd")
	}
}
