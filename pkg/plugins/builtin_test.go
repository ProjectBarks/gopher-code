package plugins

import (
	"testing"
)

// ── T33: BUILTIN_PLUGINS registry ───────────────────────────────────

func TestRegisterBuiltinPlugin_AddsToRegistry(t *testing.T) {
	ClearBuiltinPlugins()
	defer ClearBuiltinPlugins()

	RegisterBuiltinPlugin(BuiltinPluginDefinition{
		Name:        "test-plugin",
		Description: "A test plugin",
		Version:     "1.0.0",
	})

	def, ok := GetBuiltinPluginDefinition("test-plugin")
	if !ok {
		t.Fatal("expected plugin to be registered")
	}
	if def.Description != "A test plugin" {
		t.Errorf("description = %q, want %q", def.Description, "A test plugin")
	}
}

func TestGetBuiltinPluginDefinition_NotFound(t *testing.T) {
	ClearBuiltinPlugins()
	defer ClearBuiltinPlugins()

	_, ok := GetBuiltinPluginDefinition("nonexistent")
	if ok {
		t.Error("expected not found for unregistered plugin")
	}
}

// ── T34: BUILTIN_MARKETPLACE_NAME ───────────────────────────────────

func TestBuiltinMarketplaceName(t *testing.T) {
	if BUILTIN_MARKETPLACE_NAME != "builtin" {
		t.Errorf("BUILTIN_MARKETPLACE_NAME = %q, want %q", BUILTIN_MARKETPLACE_NAME, "builtin")
	}
}

func TestIsBuiltinPluginID(t *testing.T) {
	tests := []struct {
		id   string
		want bool
	}{
		{"test@builtin", true},
		{"my-plugin@builtin", true},
		{"plugin@marketplace", false},
		{"builtin", false},
		{"@builtin", true},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsBuiltinPluginID(tt.id); got != tt.want {
			t.Errorf("IsBuiltinPluginID(%q) = %v, want %v", tt.id, got, tt.want)
		}
	}
}

// ── T35: registerBuiltinPlugin/getBuiltinPlugins/getBuiltinPluginSkillCommands ──

func TestGetBuiltinPlugins_EnabledByDefault(t *testing.T) {
	ClearBuiltinPlugins()
	defer ClearBuiltinPlugins()
	EnabledPluginsFunc = func() map[string]bool { return nil }

	RegisterBuiltinPlugin(BuiltinPluginDefinition{
		Name:        "default-on",
		Description: "Enabled by default",
	})

	result := GetBuiltinPlugins()
	if len(result.Enabled) != 1 {
		t.Fatalf("expected 1 enabled, got %d", len(result.Enabled))
	}
	if result.Enabled[0].Name != "default-on" {
		t.Errorf("name = %q, want %q", result.Enabled[0].Name, "default-on")
	}
	if !result.Enabled[0].IsBuiltin {
		t.Error("IsBuiltin should be true")
	}
	if result.Enabled[0].Path != "builtin" {
		t.Errorf("path = %q, want sentinel %q", result.Enabled[0].Path, "builtin")
	}
}

func TestGetBuiltinPlugins_DefaultEnabledFalse(t *testing.T) {
	ClearBuiltinPlugins()
	defer ClearBuiltinPlugins()
	EnabledPluginsFunc = func() map[string]bool { return nil }

	f := false
	RegisterBuiltinPlugin(BuiltinPluginDefinition{
		Name:           "default-off",
		Description:    "Disabled by default",
		DefaultEnabled: &f,
	})

	result := GetBuiltinPlugins()
	if len(result.Disabled) != 1 {
		t.Fatalf("expected 1 disabled, got %d", len(result.Disabled))
	}
	if len(result.Enabled) != 0 {
		t.Fatalf("expected 0 enabled, got %d", len(result.Enabled))
	}
}

func TestGetBuiltinPlugins_UserSettingOverridesDefault(t *testing.T) {
	ClearBuiltinPlugins()
	defer ClearBuiltinPlugins()

	f := false
	RegisterBuiltinPlugin(BuiltinPluginDefinition{
		Name:           "toggled",
		Description:    "User toggled",
		DefaultEnabled: &f,
	})

	// User enables it
	EnabledPluginsFunc = func() map[string]bool {
		return map[string]bool{"toggled@builtin": true}
	}

	result := GetBuiltinPlugins()
	if len(result.Enabled) != 1 {
		t.Fatalf("expected 1 enabled after user override, got %d", len(result.Enabled))
	}
}

func TestGetBuiltinPlugins_UnavailablePluginOmitted(t *testing.T) {
	ClearBuiltinPlugins()
	defer ClearBuiltinPlugins()
	EnabledPluginsFunc = func() map[string]bool { return nil }

	RegisterBuiltinPlugin(BuiltinPluginDefinition{
		Name:        "unavailable",
		Description: "Not available",
		IsAvailable: func() bool { return false },
	})

	result := GetBuiltinPlugins()
	if len(result.Enabled) != 0 || len(result.Disabled) != 0 {
		t.Errorf("expected empty results for unavailable plugin, got enabled=%d disabled=%d",
			len(result.Enabled), len(result.Disabled))
	}
}

func TestGetBuiltinPlugins_PluginIDFormat(t *testing.T) {
	ClearBuiltinPlugins()
	defer ClearBuiltinPlugins()
	EnabledPluginsFunc = func() map[string]bool { return nil }

	RegisterBuiltinPlugin(BuiltinPluginDefinition{
		Name:        "my-plugin",
		Description: "Test",
	})

	result := GetBuiltinPlugins()
	if len(result.Enabled) != 1 {
		t.Fatalf("expected 1 enabled, got %d", len(result.Enabled))
	}
	if result.Enabled[0].Source != "my-plugin@builtin" {
		t.Errorf("source = %q, want %q", result.Enabled[0].Source, "my-plugin@builtin")
	}
}

func TestGetBuiltinPluginSkillCommands_ReturnsOnlyEnabled(t *testing.T) {
	ClearBuiltinPlugins()
	defer ClearBuiltinPlugins()
	EnabledPluginsFunc = func() map[string]bool { return nil }

	f := false
	RegisterBuiltinPlugin(BuiltinPluginDefinition{
		Name:        "enabled-plugin",
		Description: "Has skills",
		Skills: []SkillDefinition{
			{Name: "skill-a", Description: "Skill A"},
		},
	})
	RegisterBuiltinPlugin(BuiltinPluginDefinition{
		Name:           "disabled-plugin",
		Description:    "Has skills too",
		DefaultEnabled: &f,
		Skills: []SkillDefinition{
			{Name: "skill-b", Description: "Skill B"},
		},
	})

	commands := GetBuiltinPluginSkillCommands()
	if len(commands) != 1 {
		t.Fatalf("expected 1 command (from enabled only), got %d", len(commands))
	}
	if commands[0].Name != "skill-a" {
		t.Errorf("command name = %q, want %q", commands[0].Name, "skill-a")
	}
}

func TestSkillDefinitionToCommand_Fields(t *testing.T) {
	ClearBuiltinPlugins()
	defer ClearBuiltinPlugins()
	EnabledPluginsFunc = func() map[string]bool { return nil }

	RegisterBuiltinPlugin(BuiltinPluginDefinition{
		Name:        "skill-test",
		Description: "Plugin with skills",
		Skills: []SkillDefinition{
			{
				Name:         "test-skill",
				Description:  "A test skill",
				AllowedTools: []string{"bash", "read"},
				WhenToUse:    "When testing",
			},
		},
	})

	commands := GetBuiltinPluginSkillCommands()
	if len(commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(commands))
	}

	cmd := commands[0]
	if cmd.Type != "prompt" {
		t.Errorf("type = %q, want %q", cmd.Type, "prompt")
	}
	if cmd.Source != "bundled" {
		t.Errorf("source = %q, want %q", cmd.Source, "bundled")
	}
	if cmd.LoadedFrom != "bundled" {
		t.Errorf("loadedFrom = %q, want %q", cmd.LoadedFrom, "bundled")
	}
	if !cmd.HasUserSpecifiedDesc {
		t.Error("HasUserSpecifiedDesc should be true")
	}
	if !cmd.UserInvocable {
		t.Error("UserInvocable should default to true")
	}
	if cmd.IsHidden {
		t.Error("IsHidden should be false when UserInvocable is true")
	}
	if cmd.ProgressMessage != "running" {
		t.Errorf("progressMessage = %q, want %q", cmd.ProgressMessage, "running")
	}
	if len(cmd.AllowedTools) != 2 {
		t.Errorf("allowedTools len = %d, want 2", len(cmd.AllowedTools))
	}
}

func TestSkillDefinitionToCommand_UserInvocableFalse(t *testing.T) {
	f := false
	cmd := skillDefinitionToCommand(SkillDefinition{
		Name:          "hidden-skill",
		Description:   "Not user invocable",
		UserInvocable: &f,
	})
	if cmd.UserInvocable {
		t.Error("UserInvocable should be false")
	}
	if !cmd.IsHidden {
		t.Error("IsHidden should be true when UserInvocable is false")
	}
}

// ── T37: user-toggleable with config integration ──────────────────────

func TestGetBuiltinPlugins_UserDisablesDefaultEnabled(t *testing.T) {
	ClearBuiltinPlugins()
	defer ClearBuiltinPlugins()

	RegisterBuiltinPlugin(BuiltinPluginDefinition{
		Name:        "user-disabled",
		Description: "Enabled by default, disabled by user",
	})

	// User explicitly disables it
	EnabledPluginsFunc = func() map[string]bool {
		return map[string]bool{"user-disabled@builtin": false}
	}
	defer func() { EnabledPluginsFunc = func() map[string]bool { return nil } }()

	result := GetBuiltinPlugins()
	if len(result.Enabled) != 0 {
		t.Errorf("expected 0 enabled, got %d", len(result.Enabled))
	}
	if len(result.Disabled) != 1 {
		t.Errorf("expected 1 disabled, got %d", len(result.Disabled))
	}
}

// ── T36: isBuiltinPluginId additional edge cases ──────────────────────

func TestIsBuiltinPluginID_ExactSuffix(t *testing.T) {
	// Verify it checks @builtin suffix, not just "builtin" substring
	if IsBuiltinPluginID("builtin@marketplace") {
		t.Error("should not match 'builtin@marketplace'")
	}
	if !IsBuiltinPluginID("x@builtin") {
		t.Error("should match 'x@builtin'")
	}
}

// ── T39: skillDefinitionToCommand adapter completeness ────────────────

func TestSkillDefinitionToCommand_AllFields(t *testing.T) {
	cmd := skillDefinitionToCommand(SkillDefinition{
		Name:                   "full-skill",
		Description:            "Full skill",
		AllowedTools:           []string{"Bash"},
		ArgumentHint:           "<file>",
		WhenToUse:              "When X",
		Model:                  "claude-sonnet-4-20250514",
		DisableModelInvocation: true,
		Context:                "fork",
		Agent:                  "my-agent",
	})
	if cmd.ArgumentHint != "<file>" {
		t.Errorf("ArgumentHint = %q, want %q", cmd.ArgumentHint, "<file>")
	}
	if cmd.WhenToUse != "When X" {
		t.Errorf("WhenToUse = %q, want %q", cmd.WhenToUse, "When X")
	}
	if cmd.Model != "claude-sonnet-4-20250514" {
		t.Errorf("Model = %q, want %q", cmd.Model, "claude-sonnet-4-20250514")
	}
	if !cmd.DisableModelInvocation {
		t.Error("DisableModelInvocation should be true")
	}
	if cmd.Context != "fork" {
		t.Errorf("Context = %q, want %q", cmd.Context, "fork")
	}
	if cmd.Agent != "my-agent" {
		t.Errorf("Agent = %q, want %q", cmd.Agent, "my-agent")
	}
	if cmd.ContentLength != 0 {
		t.Errorf("ContentLength = %d, want 0", cmd.ContentLength)
	}
}

func TestClearBuiltinPlugins(t *testing.T) {
	ClearBuiltinPlugins()

	RegisterBuiltinPlugin(BuiltinPluginDefinition{
		Name: "temp", Description: "temp",
	})

	ClearBuiltinPlugins()

	_, ok := GetBuiltinPluginDefinition("temp")
	if ok {
		t.Error("expected clear to remove all plugins")
	}
}
