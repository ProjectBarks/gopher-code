package config

import (
	"encoding/json"
	"os"
	"path/filepath"
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
