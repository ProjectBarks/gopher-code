package config

import (
	"os"
	"path/filepath"
	"testing"
)

// Source: utils/settings/settings.ts:62-120

func TestLoadManagedSettings(t *testing.T) {

	t.Run("base_file_only", func(t *testing.T) {
		dir := t.TempDir()
		basePath := filepath.Join(dir, "managed-settings.json")
		dropInDir := filepath.Join(dir, "managed-settings.d")

		os.WriteFile(basePath, []byte(`{"model": "claude-opus-4-20250514"}`), 0644)

		settings, warnings := LoadManagedSettingsFrom(basePath, dropInDir)
		if len(warnings) > 0 {
			t.Errorf("unexpected warnings: %v", warnings)
		}
		if settings == nil {
			t.Fatal("expected settings")
		}
		if settings.Model != "claude-opus-4-20250514" {
			t.Errorf("model = %q, want 'claude-opus-4-20250514'", settings.Model)
		}
	})

	t.Run("drop_in_files_sorted_alphabetically", func(t *testing.T) {
		// Source: utils/settings/settings.ts:67-70
		// Drop-ins sorted alphabetically, later files win
		dir := t.TempDir()
		basePath := filepath.Join(dir, "managed-settings.json")
		dropInDir := filepath.Join(dir, "managed-settings.d")
		os.MkdirAll(dropInDir, 0755)

		os.WriteFile(basePath, []byte(`{"model": "base-model"}`), 0644)
		os.WriteFile(filepath.Join(dropInDir, "10-otel.json"), []byte(`{"model": "otel-model"}`), 0644)
		os.WriteFile(filepath.Join(dropInDir, "20-security.json"), []byte(`{"model": "security-model"}`), 0644)

		settings, _ := LoadManagedSettingsFrom(basePath, dropInDir)
		if settings == nil {
			t.Fatal("expected settings")
		}
		// Last alphabetical file wins
		if settings.Model != "security-model" {
			t.Errorf("model = %q, want 'security-model' (last file wins)", settings.Model)
		}
	})

	t.Run("arrays_concatenated", func(t *testing.T) {
		// Source: utils/settings/settings.ts:87 — settingsMergeCustomizer
		dir := t.TempDir()
		basePath := filepath.Join(dir, "managed-settings.json")
		dropInDir := filepath.Join(dir, "managed-settings.d")
		os.MkdirAll(dropInDir, 0755)

		os.WriteFile(basePath, []byte(`{"allowed_tools": ["Bash"]}`), 0644)
		os.WriteFile(filepath.Join(dropInDir, "01.json"), []byte(`{"allowed_tools": ["Read"]}`), 0644)

		settings, _ := LoadManagedSettingsFrom(basePath, dropInDir)
		if settings == nil {
			t.Fatal("expected settings")
		}
		if len(settings.AllowedTools) != 2 {
			t.Errorf("expected 2 allowed tools, got %d: %v", len(settings.AllowedTools), settings.AllowedTools)
		}
	})

	t.Run("hidden_files_skipped", func(t *testing.T) {
		// Source: utils/settings/settings.ts:99
		dir := t.TempDir()
		basePath := filepath.Join(dir, "managed-settings.json")
		dropInDir := filepath.Join(dir, "managed-settings.d")
		os.MkdirAll(dropInDir, 0755)

		os.WriteFile(filepath.Join(dropInDir, ".hidden.json"), []byte(`{"model": "hidden"}`), 0644)
		os.WriteFile(filepath.Join(dropInDir, "visible.json"), []byte(`{"model": "visible"}`), 0644)

		settings, _ := LoadManagedSettingsFrom(basePath, dropInDir)
		if settings == nil {
			t.Fatal("expected settings")
		}
		if settings.Model != "visible" {
			t.Errorf("model = %q, want 'visible' (hidden files skipped)", settings.Model)
		}
	})

	t.Run("non_json_files_skipped", func(t *testing.T) {
		dir := t.TempDir()
		basePath := filepath.Join(dir, "managed-settings.json")
		dropInDir := filepath.Join(dir, "managed-settings.d")
		os.MkdirAll(dropInDir, 0755)

		os.WriteFile(filepath.Join(dropInDir, "readme.txt"), []byte(`not json`), 0644)
		os.WriteFile(filepath.Join(dropInDir, "settings.json"), []byte(`{"model": "ok"}`), 0644)

		settings, _ := LoadManagedSettingsFrom(basePath, dropInDir)
		if settings.Model != "ok" {
			t.Errorf("model = %q, want 'ok'", settings.Model)
		}
	})

	t.Run("missing_both_returns_nil", func(t *testing.T) {
		dir := t.TempDir()
		settings, _ := LoadManagedSettingsFrom(
			filepath.Join(dir, "nonexistent.json"),
			filepath.Join(dir, "nonexistent.d"),
		)
		if settings != nil {
			t.Error("expected nil when no files found")
		}
	})

	t.Run("invalid_json_warned", func(t *testing.T) {
		dir := t.TempDir()
		basePath := filepath.Join(dir, "managed-settings.json")
		dropInDir := filepath.Join(dir, "managed-settings.d")
		os.MkdirAll(dropInDir, 0755)

		os.WriteFile(filepath.Join(dropInDir, "bad.json"), []byte(`{invalid json}`), 0644)

		_, warnings := LoadManagedSettingsFrom(basePath, dropInDir)
		if len(warnings) == 0 {
			t.Error("expected warning for invalid JSON")
		}
	})
}
