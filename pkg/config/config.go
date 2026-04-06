package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// AutoModeSettings holds user-defined auto-mode classifier rules.
// Source: utils/settings/settings.ts:940-944
type AutoModeSettings struct {
	Allow       []string `json:"allow,omitempty"`
	SoftDeny    []string `json:"soft_deny,omitempty"`
	Environment []string `json:"environment,omitempty"`
}

// HookConfig for settings (mirrors hooks.HookConfig to avoid import cycles).
type HookConfig struct {
	Type    string `json:"type"`
	Matcher string `json:"matcher,omitempty"`
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"`
}

// Settings holds the merged configuration.
type Settings struct {
	// Model defaults
	Model    string `json:"model,omitempty"`
	MaxTurns int    `json:"max_turns,omitempty"`

	// Permissions
	PermissionMode string `json:"permission_mode,omitempty"` // "auto", "deny", "interactive"

	// Allowed/disallowed tools
	AllowedTools    []string `json:"allowed_tools,omitempty"`
	DisallowedTools []string `json:"disallowed_tools,omitempty"`

	// Hooks
	Hooks []HookConfig `json:"hooks,omitempty"`

	// System prompt
	SystemPrompt       string `json:"system_prompt,omitempty"`
	AppendSystemPrompt string `json:"append_system_prompt,omitempty"`

	// Auto mode classifier rules
	// Source: utils/settings/settings.ts:936-982
	AutoMode *AutoModeSettings `json:"autoMode,omitempty"`

	// UI
	Theme   string `json:"theme,omitempty"`
	Verbose bool   `json:"verbose,omitempty"`

	// Updates
	AutoUpdatesChannel string `json:"autoUpdatesChannel,omitempty"` // "latest", "beta", "stable"

	// Plugins — user-toggleable enabled/disabled overrides for builtin plugins.
	// Keys are plugin IDs (e.g., "my-plugin@builtin"), values are enabled state.
	// Source: src/plugins/builtinPlugins.ts — settings.enabledPlugins
	EnabledPlugins map[string]bool `json:"enabledPlugins,omitempty"`

	// API
	APIURL     string `json:"api_url,omitempty"`
	APIVersion string `json:"api_version,omitempty"`
}

// Load reads settings from global (~/.claude/settings.json) and project (.claude/settings.json),
// merging project settings over global settings.
func Load(cwd string) *Settings {
	s := &Settings{}

	// 1. Global settings
	if home, err := os.UserHomeDir(); err == nil {
		globalPath := filepath.Join(home, ".claude", "settings.json")
		loadInto(globalPath, s)
	}

	// 2. Project settings (override global)
	if cwd != "" {
		projectPath := filepath.Join(cwd, ".claude", "settings.json")
		loadInto(projectPath, s)
	}

	return s
}

// LoadFrom reads settings from a specific file path.
func LoadFrom(path string) *Settings {
	s := &Settings{}
	loadInto(path, s)
	return s
}

func loadInto(path string, s *Settings) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	json.Unmarshal(data, s) //nolint:errcheck // Merge: non-zero fields overwrite
}

// ── T123: CLI flag overrides for settings ────────────────────────────

// flagSettings holds CLI flag overrides for the settings file path and
// inline settings JSON. These are package-level state matching the TS
// singleton pattern in bootstrap/state.ts.
// Source: bootstrap/state.ts — flagSettingsPath / flagSettingsInline
var flagSettings struct {
	mu     sync.RWMutex
	path   string
	inline map[string]any
}

// FlagSettingsPath returns the CLI-specified settings file path override.
func FlagSettingsPath() string {
	flagSettings.mu.RLock()
	defer flagSettings.mu.RUnlock()
	return flagSettings.path
}

// SetFlagSettingsPath sets the CLI-specified settings file path override.
func SetFlagSettingsPath(path string) {
	flagSettings.mu.Lock()
	defer flagSettings.mu.Unlock()
	flagSettings.path = path
}

// FlagSettingsInline returns the inline settings JSON provided via CLI flag.
func FlagSettingsInline() map[string]any {
	flagSettings.mu.RLock()
	defer flagSettings.mu.RUnlock()
	return flagSettings.inline
}

// SetFlagSettingsInline sets the inline settings JSON from CLI flag.
func SetFlagSettingsInline(settings map[string]any) {
	flagSettings.mu.Lock()
	defer flagSettings.mu.Unlock()
	flagSettings.inline = settings
}

// ── T124: Allowed setting sources ────────────────────────────────────

// allowedSources restricts which setting sources are loaded.
// Default: user, project, local, flag, policy (all sources).
// Source: bootstrap/state.ts — allowedSettingSources
var allowedSources struct {
	mu      sync.RWMutex
	sources []SettingSource
}

func init() {
	// Default matches TS: ['userSettings','projectSettings','localSettings','flagSettings','policySettings']
	allowedSources.sources = []SettingSource{
		SourceUser,
		SourceProject,
		SourceLocal,
		SourceFlag,
		SourcePolicy,
	}
}

// AllowedSettingSources returns which setting sources are currently allowed.
func AllowedSettingSources() []SettingSource {
	allowedSources.mu.RLock()
	defer allowedSources.mu.RUnlock()
	result := make([]SettingSource, len(allowedSources.sources))
	copy(result, allowedSources.sources)
	return result
}

// SetAllowedSettingSources sets which setting sources are allowed.
func SetAllowedSettingSources(sources []SettingSource) {
	allowedSources.mu.Lock()
	defer allowedSources.mu.Unlock()
	allowedSources.sources = sources
}

// ResetFlagSettings resets flag settings to their zero values (for testing).
func ResetFlagSettings() {
	flagSettings.mu.Lock()
	defer flagSettings.mu.Unlock()
	flagSettings.path = ""
	flagSettings.inline = nil
}

// ResetAllowedSettingSources resets to default (all sources allowed).
func ResetAllowedSettingSources() {
	allowedSources.mu.Lock()
	defer allowedSources.mu.Unlock()
	allowedSources.sources = []SettingSource{
		SourceUser,
		SourceProject,
		SourceLocal,
		SourceFlag,
		SourcePolicy,
	}
}

// Save writes settings to the global config file.
func (s *Settings) Save() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	return s.SaveTo(filepath.Join(home, ".claude", "settings.json"))
}

// SaveTo writes settings to a specific file path.
func (s *Settings) SaveTo(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}
