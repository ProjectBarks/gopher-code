package permissions

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Source: utils/permissions/PermissionUpdate.ts, utils/permissions/PermissionUpdateSchema.ts

func TestPermissionUpdateDestinations(t *testing.T) {
	// Source: PermissionUpdateSchema.ts:28-39
	if DestUserSettings != "userSettings" {
		t.Error("wrong")
	}
	if DestProjectSettings != "projectSettings" {
		t.Error("wrong")
	}
	if DestLocalSettings != "localSettings" {
		t.Error("wrong")
	}
	if DestSession != "session" {
		t.Error("wrong")
	}
	if DestCLIArg != "cliArg" {
		t.Error("wrong")
	}
}

func TestSupportsPersistence(t *testing.T) {
	// Source: PermissionUpdate.ts:208-216
	if !SupportsPersistence(DestUserSettings) {
		t.Error("userSettings should support persistence")
	}
	if !SupportsPersistence(DestProjectSettings) {
		t.Error("projectSettings should support persistence")
	}
	if !SupportsPersistence(DestLocalSettings) {
		t.Error("localSettings should support persistence")
	}
	if SupportsPersistence(DestSession) {
		t.Error("session should NOT support persistence")
	}
	if SupportsPersistence(DestCLIArg) {
		t.Error("cliArg should NOT support persistence")
	}
}

func TestToolPermissionContext_ApplyAddRules(t *testing.T) {
	// Source: PermissionUpdate.ts:69-95
	ctx := NewToolPermissionContext("default")

	ctx.ApplyPermissionUpdate(PermissionUpdate{
		Type:        UpdateAddRules,
		Rules:       []string{"Bash(npm install)", "Read(src/**)"},
		Behavior:    BehaviorAllow,
		Destination: DestSession,
	})

	rules := ctx.AlwaysAllowRules[DestSession]
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(rules))
	}
	if rules[0] != "Bash(npm install)" || rules[1] != "Read(src/**)" {
		t.Errorf("rules = %v", rules)
	}
}

func TestToolPermissionContext_ApplyReplaceRules(t *testing.T) {
	// Source: PermissionUpdate.ts:97-120
	ctx := NewToolPermissionContext("default")
	ctx.AlwaysAllowRules[DestSession] = []string{"old-rule"}

	ctx.ApplyPermissionUpdate(PermissionUpdate{
		Type:        UpdateReplaceRules,
		Rules:       []string{"new-rule-1", "new-rule-2"},
		Behavior:    BehaviorAllow,
		Destination: DestSession,
	})

	rules := ctx.AlwaysAllowRules[DestSession]
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules after replace, got %d", len(rules))
	}
	if rules[0] != "new-rule-1" {
		t.Errorf("rules = %v", rules)
	}
}

func TestToolPermissionContext_ApplyRemoveRules(t *testing.T) {
	// Source: PermissionUpdate.ts:139-169
	ctx := NewToolPermissionContext("default")
	ctx.AlwaysDenyRules[DestSession] = []string{"rule-a", "rule-b", "rule-c"}

	ctx.ApplyPermissionUpdate(PermissionUpdate{
		Type:        UpdateRemoveRules,
		Rules:       []string{"rule-b"},
		Behavior:    BehaviorDeny,
		Destination: DestSession,
	})

	rules := ctx.AlwaysDenyRules[DestSession]
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules after remove, got %d", len(rules))
	}
	if rules[0] != "rule-a" || rules[1] != "rule-c" {
		t.Errorf("rules = %v", rules)
	}
}

func TestToolPermissionContext_ApplySetMode(t *testing.T) {
	// Source: PermissionUpdate.ts:60-67
	ctx := NewToolPermissionContext("default")

	ctx.ApplyPermissionUpdate(PermissionUpdate{
		Type:        UpdateSetMode,
		Mode:        "bypassPermissions",
		Destination: DestSession,
	})

	if ctx.Mode != "bypassPermissions" {
		t.Errorf("mode = %q", ctx.Mode)
	}
}

func TestToolPermissionContext_ApplyAddDirectories(t *testing.T) {
	// Source: PermissionUpdate.ts:122-137
	ctx := NewToolPermissionContext("default")

	ctx.ApplyPermissionUpdate(PermissionUpdate{
		Type:        UpdateAddDirectories,
		Directories: []string{"/usr/local/lib", "/opt/shared"},
		Destination: DestProjectSettings,
	})

	if len(ctx.AdditionalWorkingDirectories) != 2 {
		t.Fatalf("expected 2 dirs, got %d", len(ctx.AdditionalWorkingDirectories))
	}
	if ctx.AdditionalWorkingDirectories["/usr/local/lib"] != "projectSettings" {
		t.Error("directory source should be projectSettings")
	}
}

func TestToolPermissionContext_ApplyRemoveDirectories(t *testing.T) {
	// Source: PermissionUpdate.ts:171-183
	ctx := NewToolPermissionContext("default")
	ctx.AdditionalWorkingDirectories["/a"] = "session"
	ctx.AdditionalWorkingDirectories["/b"] = "session"

	ctx.ApplyPermissionUpdate(PermissionUpdate{
		Type:        UpdateRemoveDirectories,
		Directories: []string{"/a"},
		Destination: DestSession,
	})

	if len(ctx.AdditionalWorkingDirectories) != 1 {
		t.Fatalf("expected 1 dir, got %d", len(ctx.AdditionalWorkingDirectories))
	}
	if _, ok := ctx.AdditionalWorkingDirectories["/b"]; !ok {
		t.Error("/b should remain")
	}
}

func TestToolPermissionContext_AllAllowRules(t *testing.T) {
	ctx := NewToolPermissionContext("default")
	ctx.AlwaysAllowRules[DestUserSettings] = []string{"Bash"}
	ctx.AlwaysAllowRules[DestSession] = []string{"Read", "Write"}

	rules := ctx.AllAllowRules()
	if len(rules) != 3 {
		t.Errorf("expected 3, got %d", len(rules))
	}
}

func TestToolPermissionContext_DenyBehavior(t *testing.T) {
	// Verify deny rules go to the deny map
	ctx := NewToolPermissionContext("default")
	ctx.ApplyPermissionUpdate(PermissionUpdate{
		Type:        UpdateAddRules,
		Rules:       []string{"PowerShell(*)"},
		Behavior:    BehaviorDeny,
		Destination: DestUserSettings,
	})

	rules := ctx.AllDenyRules()
	if len(rules) != 1 || rules[0] != "PowerShell(*)" {
		t.Errorf("deny rules = %v", rules)
	}
}

func TestPersistPermissionUpdate_AddRules(t *testing.T) {
	// Source: PermissionUpdate.ts:230-241
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	os.MkdirAll(filepath.Join(homeDir, ".claude"), 0755)

	p := NewPermissionRulePersister(homeDir, "")

	err := p.PersistPermissionUpdate(PermissionUpdate{
		Type:        UpdateAddRules,
		Rules:       []string{"Bash(npm install)", "Read"},
		Behavior:    BehaviorAllow,
		Destination: DestUserSettings,
	})
	if err != nil {
		t.Fatalf("persist error: %v", err)
	}

	// Verify file
	data, err := os.ReadFile(filepath.Join(homeDir, ".claude", "settings.json"))
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	var raw map[string]json.RawMessage
	json.Unmarshal(data, &raw)
	var perms PermissionsConfig
	json.Unmarshal(raw["permissions"], &perms)

	if len(perms.Allow) != 2 {
		t.Fatalf("expected 2 allow rules, got %d", len(perms.Allow))
	}
	if perms.Allow[0] != "Bash(npm install)" {
		t.Errorf("allow[0] = %q", perms.Allow[0])
	}
}

func TestPersistPermissionUpdate_Deduplication(t *testing.T) {
	// Source: PermissionUpdate.ts — dedup on add
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	os.MkdirAll(filepath.Join(homeDir, ".claude"), 0755)

	p := NewPermissionRulePersister(homeDir, "")

	// Add twice
	p.PersistPermissionUpdate(PermissionUpdate{
		Type: UpdateAddRules, Rules: []string{"Bash"}, Behavior: BehaviorAllow, Destination: DestUserSettings,
	})
	p.PersistPermissionUpdate(PermissionUpdate{
		Type: UpdateAddRules, Rules: []string{"Bash"}, Behavior: BehaviorAllow, Destination: DestUserSettings,
	})

	perms := p.loadPermissions(filepath.Join(homeDir, ".claude", "settings.json"))
	if len(perms.Allow) != 1 {
		t.Errorf("expected 1 (deduped), got %d: %v", len(perms.Allow), perms.Allow)
	}
}

func TestPersistPermissionUpdate_RemoveRules(t *testing.T) {
	// Source: PermissionUpdate.ts:268-294
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	settingsDir := filepath.Join(homeDir, ".claude")
	os.MkdirAll(settingsDir, 0755)
	os.WriteFile(filepath.Join(settingsDir, "settings.json"),
		[]byte(`{"permissions":{"allow":["Bash","Read","Write"]}}`), 0644)

	p := NewPermissionRulePersister(homeDir, "")

	err := p.PersistPermissionUpdate(PermissionUpdate{
		Type: UpdateRemoveRules, Rules: []string{"Read"}, Behavior: BehaviorAllow, Destination: DestUserSettings,
	})
	if err != nil {
		t.Fatalf("persist error: %v", err)
	}

	perms := p.loadPermissions(filepath.Join(settingsDir, "settings.json"))
	if len(perms.Allow) != 2 {
		t.Fatalf("expected 2 after remove, got %d: %v", len(perms.Allow), perms.Allow)
	}
	for _, r := range perms.Allow {
		if r == "Read" {
			t.Error("Read should have been removed")
		}
	}
}

func TestPersistPermissionUpdate_SessionNotPersisted(t *testing.T) {
	// Source: PermissionUpdate.ts:223
	tmpDir := t.TempDir()
	p := NewPermissionRulePersister(tmpDir, "")

	err := p.PersistPermissionUpdate(PermissionUpdate{
		Type: UpdateAddRules, Rules: []string{"Bash"}, Behavior: BehaviorAllow, Destination: DestSession,
	})
	if err != nil {
		t.Fatalf("should succeed (no-op): %v", err)
	}
	// No file should be created
	_, err = os.Stat(filepath.Join(tmpDir, ".claude", "settings.json"))
	if err == nil {
		t.Error("session rules should not create settings file")
	}
}

func TestPersistPermissionUpdate_PreservesOtherFields(t *testing.T) {
	// Existing settings should be preserved
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	settingsDir := filepath.Join(homeDir, ".claude")
	os.MkdirAll(settingsDir, 0755)
	os.WriteFile(filepath.Join(settingsDir, "settings.json"),
		[]byte(`{"model":"opus","permissions":{"allow":["Read"]}}`), 0644)

	p := NewPermissionRulePersister(homeDir, "")
	p.PersistPermissionUpdate(PermissionUpdate{
		Type: UpdateAddRules, Rules: []string{"Bash"}, Behavior: BehaviorAllow, Destination: DestUserSettings,
	})

	data, _ := os.ReadFile(filepath.Join(settingsDir, "settings.json"))
	var raw map[string]json.RawMessage
	json.Unmarshal(data, &raw)

	// Model should still be there
	if _, ok := raw["model"]; !ok {
		t.Error("model field should be preserved")
	}
}

func TestPersistPermissionUpdate_ProjectSettings(t *testing.T) {
	tmpDir := t.TempDir()
	cwd := filepath.Join(tmpDir, "project")
	os.MkdirAll(filepath.Join(cwd, ".claude"), 0755)

	p := NewPermissionRulePersister(tmpDir, cwd)

	err := p.PersistPermissionUpdate(PermissionUpdate{
		Type: UpdateAddRules, Rules: []string{"Bash(npm test)"}, Behavior: BehaviorAllow, Destination: DestProjectSettings,
	})
	if err != nil {
		t.Fatalf("persist error: %v", err)
	}

	perms := p.loadPermissions(filepath.Join(cwd, ".claude", "settings.json"))
	if len(perms.Allow) != 1 || perms.Allow[0] != "Bash(npm test)" {
		t.Errorf("allow = %v", perms.Allow)
	}
}

func TestPersistPermissionUpdate_SetMode(t *testing.T) {
	// Source: PermissionUpdate.ts:317-325
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	os.MkdirAll(filepath.Join(homeDir, ".claude"), 0755)

	p := NewPermissionRulePersister(homeDir, "")
	p.PersistPermissionUpdate(PermissionUpdate{
		Type: UpdateSetMode, Mode: "auto", Destination: DestUserSettings,
	})

	perms := p.loadPermissions(filepath.Join(homeDir, ".claude", "settings.json"))
	if perms.DefaultMode != "auto" {
		t.Errorf("defaultMode = %q, want auto", perms.DefaultMode)
	}
}

func TestPersistPermissionUpdate_AddDirectories(t *testing.T) {
	// Source: PermissionUpdate.ts:244-266
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	os.MkdirAll(filepath.Join(homeDir, ".claude"), 0755)

	p := NewPermissionRulePersister(homeDir, "")
	p.PersistPermissionUpdate(PermissionUpdate{
		Type: UpdateAddDirectories, Directories: []string{"/opt/lib"}, Destination: DestUserSettings,
	})

	perms := p.loadPermissions(filepath.Join(homeDir, ".claude", "settings.json"))
	if len(perms.AdditionalDirectories) != 1 || perms.AdditionalDirectories[0] != "/opt/lib" {
		t.Errorf("dirs = %v", perms.AdditionalDirectories)
	}

	// Add again — should deduplicate
	p.PersistPermissionUpdate(PermissionUpdate{
		Type: UpdateAddDirectories, Directories: []string{"/opt/lib"}, Destination: DestUserSettings,
	})
	perms = p.loadPermissions(filepath.Join(homeDir, ".claude", "settings.json"))
	if len(perms.AdditionalDirectories) != 1 {
		t.Errorf("expected 1 (deduped), got %d", len(perms.AdditionalDirectories))
	}
}

func TestLoadPermissionContext(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	cwd := filepath.Join(tmpDir, "project")
	os.MkdirAll(filepath.Join(homeDir, ".claude"), 0755)
	os.MkdirAll(filepath.Join(cwd, ".claude"), 0755)

	// Write user settings
	os.WriteFile(filepath.Join(homeDir, ".claude", "settings.json"),
		[]byte(`{"permissions":{"allow":["Bash"],"deny":["PowerShell"]}}`), 0644)

	// Write project settings
	os.WriteFile(filepath.Join(cwd, ".claude", "settings.json"),
		[]byte(`{"permissions":{"allow":["Read(src/**)"],"ask":["Bash(rm *)"]}}`), 0644)

	p := NewPermissionRulePersister(homeDir, cwd)
	ctx := p.LoadPermissionContext("default")

	// User allow rules
	userAllow := ctx.AlwaysAllowRules[DestUserSettings]
	if len(userAllow) != 1 || userAllow[0] != "Bash" {
		t.Errorf("user allow = %v", userAllow)
	}

	// User deny rules
	userDeny := ctx.AlwaysDenyRules[DestUserSettings]
	if len(userDeny) != 1 || userDeny[0] != "PowerShell" {
		t.Errorf("user deny = %v", userDeny)
	}

	// Project allow rules
	projectAllow := ctx.AlwaysAllowRules[DestProjectSettings]
	if len(projectAllow) != 1 || projectAllow[0] != "Read(src/**)" {
		t.Errorf("project allow = %v", projectAllow)
	}

	// Project ask rules
	projectAsk := ctx.AlwaysAskRules[DestProjectSettings]
	if len(projectAsk) != 1 || projectAsk[0] != "Bash(rm *)" {
		t.Errorf("project ask = %v", projectAsk)
	}

	// All allow (flattened)
	allAllow := ctx.AllAllowRules()
	if len(allAllow) != 2 {
		t.Errorf("expected 2 total allow rules, got %d", len(allAllow))
	}
}

func TestPermissionBehaviors(t *testing.T) {
	if BehaviorAllow != "allow" {
		t.Error("wrong")
	}
	if BehaviorDeny != "deny" {
		t.Error("wrong")
	}
	if BehaviorAsk != "ask" {
		t.Error("wrong")
	}
}

func TestPermissionUpdateTypes(t *testing.T) {
	// Source: PermissionUpdateSchema.ts:43-77
	types := []PermissionUpdateType{
		UpdateAddRules, UpdateReplaceRules, UpdateRemoveRules,
		UpdateSetMode, UpdateAddDirectories, UpdateRemoveDirectories,
	}
	for _, typ := range types {
		if typ == "" {
			t.Error("update type should not be empty")
		}
	}
}
