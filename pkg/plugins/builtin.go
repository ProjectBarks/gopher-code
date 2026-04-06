package plugins

import (
	"fmt"
	"strings"
	"sync"
)

// BUILTIN_MARKETPLACE_NAME is the marketplace identifier for built-in plugins.
// Plugin IDs use the format "{name}@builtin".
// Source: src/plugins/builtinPlugins.ts — BUILTIN_MARKETPLACE_NAME
const BUILTIN_MARKETPLACE_NAME = "builtin"

// builtinPlugins is the global registry of built-in plugin definitions.
// Source: src/plugins/builtinPlugins.ts — BUILTIN_PLUGINS Map
var (
	builtinMu      sync.RWMutex
	builtinPlugins = map[string]BuiltinPluginDefinition{}
)

// EnabledPluginsFunc is the function used to read user plugin preferences.
// Default returns nil (no user overrides). Set during startup to wire in
// config.Settings.EnabledPlugins.
var EnabledPluginsFunc = func() map[string]bool { return nil }

// RegisterBuiltinPlugin registers a built-in plugin. Call from initBuiltinPlugins() at startup.
// Source: src/plugins/builtinPlugins.ts — registerBuiltinPlugin
func RegisterBuiltinPlugin(definition BuiltinPluginDefinition) {
	builtinMu.Lock()
	defer builtinMu.Unlock()
	builtinPlugins[definition.Name] = definition
}

// IsBuiltinPluginID checks if a plugin ID represents a built-in plugin (ends with @builtin).
// Source: src/plugins/builtinPlugins.ts — isBuiltinPluginId
func IsBuiltinPluginID(pluginID string) bool {
	return strings.HasSuffix(pluginID, "@"+BUILTIN_MARKETPLACE_NAME)
}

// GetBuiltinPluginDefinition returns a specific built-in plugin definition by name.
// Source: src/plugins/builtinPlugins.ts — getBuiltinPluginDefinition
func GetBuiltinPluginDefinition(name string) (BuiltinPluginDefinition, bool) {
	builtinMu.RLock()
	defer builtinMu.RUnlock()
	def, ok := builtinPlugins[name]
	return def, ok
}

// GetBuiltinPlugins returns all registered built-in plugins as LoadedPlugin objects,
// split into enabled/disabled based on user settings (with defaultEnabled as fallback).
// Plugins whose IsAvailable() returns false are omitted entirely.
// Source: src/plugins/builtinPlugins.ts — getBuiltinPlugins
func GetBuiltinPlugins() PluginLoadResult {
	builtinMu.RLock()
	defer builtinMu.RUnlock()

	enabledPlugins := EnabledPluginsFunc()
	var result PluginLoadResult

	for name, definition := range builtinPlugins {
		// Skip unavailable plugins entirely
		if definition.IsAvailable != nil && !definition.IsAvailable() {
			continue
		}

		pluginID := fmt.Sprintf("%s@%s", name, BUILTIN_MARKETPLACE_NAME)

		// Enabled state: user preference > plugin default > true
		isEnabled := true
		if definition.DefaultEnabled != nil {
			isEnabled = *definition.DefaultEnabled
		}
		if userSetting, ok := enabledPlugins[pluginID]; ok {
			isEnabled = userSetting
		}

		plugin := LoadedPlugin{
			Name: name,
			Manifest: PluginManifest{
				Name:        name,
				Description: definition.Description,
				Version:     definition.Version,
			},
			Path:        BUILTIN_MARKETPLACE_NAME, // sentinel — no filesystem path
			Source:      pluginID,
			Repository:  pluginID,
			Enabled:     isEnabled,
			IsBuiltin:   true,
			HooksConfig: definition.Hooks,
			McpServers:  definition.McpServers,
		}

		if isEnabled {
			result.Enabled = append(result.Enabled, plugin)
		} else {
			result.Disabled = append(result.Disabled, plugin)
		}
	}

	return result
}

// GetBuiltinPluginSkillCommands returns skills from enabled built-in plugins as Command objects.
// Skills from disabled plugins are not returned.
// Source: src/plugins/builtinPlugins.ts — getBuiltinPluginSkillCommands
func GetBuiltinPluginSkillCommands() []Command {
	result := GetBuiltinPlugins()
	var commands []Command

	builtinMu.RLock()
	defer builtinMu.RUnlock()

	for _, plugin := range result.Enabled {
		definition, ok := builtinPlugins[plugin.Name]
		if !ok || len(definition.Skills) == 0 {
			continue
		}
		for _, skill := range definition.Skills {
			commands = append(commands, skillDefinitionToCommand(skill))
		}
	}

	return commands
}

// ClearBuiltinPlugins clears the registry (for testing).
// Source: src/plugins/builtinPlugins.ts — clearBuiltinPlugins
func ClearBuiltinPlugins() {
	builtinMu.Lock()
	defer builtinMu.Unlock()
	builtinPlugins = map[string]BuiltinPluginDefinition{}
}

// skillDefinitionToCommand converts a SkillDefinition to a Command.
// Source: src/plugins/builtinPlugins.ts — skillDefinitionToCommand
func skillDefinitionToCommand(def SkillDefinition) Command {
	userInvocable := true
	if def.UserInvocable != nil {
		userInvocable = *def.UserInvocable
	}

	return Command{
		Type:                   "prompt",
		Name:                   def.Name,
		Description:            def.Description,
		HasUserSpecifiedDesc:   true,
		AllowedTools:           def.AllowedTools,
		ArgumentHint:           def.ArgumentHint,
		WhenToUse:              def.WhenToUse,
		Model:                  def.Model,
		DisableModelInvocation: def.DisableModelInvocation,
		UserInvocable:          userInvocable,
		ContentLength:          0,
		// 'bundled' not 'builtin' — 'builtin' in Command.source means hardcoded
		// slash commands (/help, /clear). Using 'bundled' keeps these skills in
		// the Skill tool's listing, analytics, and prompt-truncation exemption.
		Source:          "bundled",
		LoadedFrom:      "bundled",
		Context:         def.Context,
		Agent:           def.Agent,
		IsHidden:        !userInvocable,
		ProgressMessage: "running",
	}
}
