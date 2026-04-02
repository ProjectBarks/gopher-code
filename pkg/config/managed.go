package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// Source: utils/settings/settings.ts:62-120, utils/settings/managedPath.ts

// ManagedSettingsFileName is the base managed settings file.
const ManagedSettingsFileName = "managed-settings.json"

// ManagedSettingsDropInDir is the drop-in directory for policy fragments.
const ManagedSettingsDropInDir = "managed-settings.d"

// GetManagedSettingsDir returns the platform-specific directory for managed settings.
// Source: utils/settings/managedPath.ts:4-26
func GetManagedSettingsDir() string {
	switch runtime.GOOS {
	case "darwin":
		return "/Library/Application Support/ClaudeCode"
	case "linux":
		return "/etc/claude-code"
	default:
		// Windows or other: use user config dir
		dir, _ := os.UserConfigDir()
		return filepath.Join(dir, "claude-code")
	}
}

// GetManagedSettingsFilePath returns the path to managed-settings.json.
func GetManagedSettingsFilePath() string {
	return filepath.Join(GetManagedSettingsDir(), ManagedSettingsFileName)
}

// GetManagedSettingsDropInPath returns the path to managed-settings.d/.
// Source: utils/settings/managedPath.ts:28-33
func GetManagedSettingsDropInPath() string {
	return filepath.Join(GetManagedSettingsDir(), ManagedSettingsDropInDir)
}

// LoadManagedSettings loads managed-settings.json + managed-settings.d/*.json.
// Base file is merged first (lowest precedence), then drop-in files sorted
// alphabetically (later files win). Matches systemd/sudoers convention.
// Source: utils/settings/settings.ts:62-120
func LoadManagedSettings() (*Settings, []string) {
	return LoadManagedSettingsFrom(GetManagedSettingsFilePath(), GetManagedSettingsDropInPath())
}

// LoadManagedSettingsFrom loads from explicit paths (for testing).
// Source: utils/settings/settings.ts:74-120
func LoadManagedSettingsFrom(basePath, dropInDir string) (*Settings, []string) {
	var warnings []string
	merged := &Settings{}
	found := false

	// 1. Load base managed-settings.json
	if base, err := loadSettingsFile(basePath); err == nil && base != nil {
		mergeSettings(merged, base)
		found = true
	}

	// 2. Load managed-settings.d/*.json (sorted alphabetically)
	// Source: utils/settings/settings.ts:91-118
	entries, err := os.ReadDir(dropInDir)
	if err == nil {
		var jsonFiles []string
		for _, e := range entries {
			name := e.Name()
			if (e.Type().IsRegular() || e.Type()&os.ModeSymlink != 0) &&
				strings.HasSuffix(name, ".json") &&
				!strings.HasPrefix(name, ".") {
				jsonFiles = append(jsonFiles, name)
			}
		}
		sort.Strings(jsonFiles)

		for _, name := range jsonFiles {
			path := filepath.Join(dropInDir, name)
			s, err := loadSettingsFile(path)
			if err != nil {
				warnings = append(warnings, "failed to load "+path+": "+err.Error())
				continue
			}
			if s != nil {
				mergeSettings(merged, s)
				found = true
			}
		}
	}

	if !found {
		return nil, warnings
	}
	return merged, warnings
}

// loadSettingsFile reads and parses a single JSON settings file.
func loadSettingsFile(path string) (*Settings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// mergeSettings merges src into dst. Non-zero fields in src override dst.
// Arrays are concatenated (not replaced) for permission rules.
// Source: utils/settings/settings.ts:87 — mergeWith settingsMergeCustomizer
func mergeSettings(dst, src *Settings) {
	if src.Model != "" {
		dst.Model = src.Model
	}
	if src.MaxTurns > 0 {
		dst.MaxTurns = src.MaxTurns
	}
	if src.PermissionMode != "" {
		dst.PermissionMode = src.PermissionMode
	}
	if src.SystemPrompt != "" {
		dst.SystemPrompt = src.SystemPrompt
	}
	if src.AppendSystemPrompt != "" {
		dst.AppendSystemPrompt = src.AppendSystemPrompt
	}
	if src.Theme != "" {
		dst.Theme = src.Theme
	}
	if src.Verbose {
		dst.Verbose = true
	}
	if src.APIURL != "" {
		dst.APIURL = src.APIURL
	}
	if src.APIVersion != "" {
		dst.APIVersion = src.APIVersion
	}
	// Arrays: concatenate (permission rules accumulate across sources)
	dst.AllowedTools = append(dst.AllowedTools, src.AllowedTools...)
	dst.DisallowedTools = append(dst.DisallowedTools, src.DisallowedTools...)
	dst.Hooks = append(dst.Hooks, src.Hooks...)
}
