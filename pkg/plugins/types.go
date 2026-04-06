// Package plugins provides the built-in plugin registry for user-toggleable
// features that ship with the CLI.
//
// Source: src/plugins/builtinPlugins.ts, src/types/plugin.ts
package plugins

// BuiltinPluginDefinition describes a built-in plugin that ships with the CLI.
// Built-in plugins appear in the /plugin UI and can be enabled/disabled by users.
// Source: types/plugin.ts — BuiltinPluginDefinition
type BuiltinPluginDefinition struct {
	// Name is used in the "{name}@builtin" plugin identifier.
	Name string
	// Description shown in the /plugin UI.
	Description string
	// Version string (optional).
	Version string
	// Skills provided by this plugin.
	Skills []SkillDefinition
	// Hooks provided by this plugin (opaque config).
	Hooks map[string]any
	// McpServers provided by this plugin.
	McpServers map[string]any
	// IsAvailable gates whether the plugin is shown at all. Nil means always available.
	IsAvailable func() bool
	// DefaultEnabled is the default enabled state before user sets a preference (defaults to true).
	DefaultEnabled *bool
}

// SkillDefinition mirrors BundledSkillDefinition for plugin-provided skills.
// Source: skills/bundledSkills.ts — BundledSkillDefinition (subset)
type SkillDefinition struct {
	Name                   string
	Description            string
	AllowedTools           []string
	ArgumentHint           string
	WhenToUse              string
	Model                  string
	DisableModelInvocation bool
	UserInvocable          *bool // nil defaults to true
	Hooks                  map[string]any
	Context                string // "inline" or "fork"
	Agent                  string
	IsEnabled              func() bool
	GetPromptForCommand    func(args string) (string, error)
}

// LoadedPlugin represents a plugin that has been resolved from a definition
// with enable/disable state applied.
// Source: types/plugin.ts — LoadedPlugin
type LoadedPlugin struct {
	Name       string
	Manifest   PluginManifest
	Path       string // sentinel "builtin" for built-in plugins
	Source     string // "{name}@builtin"
	Repository string
	Enabled    bool
	IsBuiltin  bool // true for built-in plugins that ship with the CLI
	HooksConfig map[string]any
	McpServers  map[string]any
}

// PluginManifest holds plugin metadata.
// Source: utils/plugins/schemas.ts — PluginManifest (subset)
type PluginManifest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version,omitempty"`
}

// PluginLoadResult holds enabled and disabled plugin lists.
// Source: types/plugin.ts — PluginLoadResult
type PluginLoadResult struct {
	Enabled  []LoadedPlugin
	Disabled []LoadedPlugin
}

// Command represents a skill command surfaced to the model.
// Source: types/command.ts — Command (subset for plugin skills)
type Command struct {
	Type                    string   `json:"type"`
	Name                    string   `json:"name"`
	Description             string   `json:"description"`
	HasUserSpecifiedDesc    bool     `json:"hasUserSpecifiedDescription"`
	AllowedTools            []string `json:"allowedTools"`
	ArgumentHint            string   `json:"argumentHint,omitempty"`
	WhenToUse               string   `json:"whenToUse,omitempty"`
	Model                   string   `json:"model,omitempty"`
	DisableModelInvocation  bool     `json:"disableModelInvocation"`
	UserInvocable           bool     `json:"userInvocable"`
	ContentLength           int      `json:"contentLength"`
	Source                  string   `json:"source"`   // "bundled" (not "builtin")
	LoadedFrom              string   `json:"loadedFrom"` // "bundled"
	Context                 string   `json:"context,omitempty"`
	Agent                   string   `json:"agent,omitempty"`
	IsHidden                bool     `json:"isHidden"`
	ProgressMessage         string   `json:"progressMessage"`
}
