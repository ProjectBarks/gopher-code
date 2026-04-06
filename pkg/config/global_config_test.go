package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Source: src/utils/config.ts — GlobalConfig, ProjectConfig, defaults, save/load, trust, constants

func TestGlobalConfigDefaults(t *testing.T) {
	// Source: config.ts:585-623 — createDefaultGlobalConfig()
	d := DefaultGlobalConfig()

	if d.NumStartups != 0 {
		t.Errorf("NumStartups = %d, want 0", d.NumStartups)
	}
	if d.Theme != "dark" {
		t.Errorf("Theme = %q, want 'dark'", d.Theme)
	}
	if d.PreferredNotifChannel != "auto" {
		t.Errorf("PreferredNotifChannel = %q, want 'auto'", d.PreferredNotifChannel)
	}
	if d.EditorMode != "normal" {
		t.Errorf("EditorMode = %q, want 'normal'", d.EditorMode)
	}
	if !d.AutoCompactEnabled {
		t.Error("AutoCompactEnabled should default true")
	}
	if !d.ShowTurnDuration {
		t.Error("ShowTurnDuration should default true")
	}
	if d.DiffTool != "auto" {
		t.Errorf("DiffTool = %q, want 'auto'", d.DiffTool)
	}
	if !d.TodoFeatureEnabled {
		t.Error("TodoFeatureEnabled should default true")
	}
	if d.MessageIdleNotifThresholdMs != 60000 {
		t.Errorf("MessageIdleNotifThresholdMs = %d, want 60000", d.MessageIdleNotifThresholdMs)
	}
	if !d.FileCheckpointingEnabled {
		t.Error("FileCheckpointingEnabled should default true")
	}
	if !d.TerminalProgressBarEnabled {
		t.Error("TerminalProgressBarEnabled should default true")
	}
	if !d.RespectGitignore {
		t.Error("RespectGitignore should default true")
	}
	if d.CopyFullResponse {
		t.Error("CopyFullResponse should default false")
	}
	if d.MemoryUsageCount != 0 {
		t.Errorf("MemoryUsageCount = %d, want 0", d.MemoryUsageCount)
	}
	if d.Verbose {
		t.Error("Verbose should default false")
	}
	if d.AutoConnectIde {
		t.Error("AutoConnectIde should default false")
	}
	if !d.AutoInstallIdeExtension {
		t.Error("AutoInstallIdeExtension should default true")
	}
}

func TestProjectConfigDefaults(t *testing.T) {
	// Source: config.ts:138-148 — DEFAULT_PROJECT_CONFIG
	d := DefaultProjectConfig()

	if len(d.AllowedTools) != 0 {
		t.Errorf("AllowedTools should be empty, got %v", d.AllowedTools)
	}
	if d.HasTrustDialogAccepted {
		t.Error("HasTrustDialogAccepted should default false")
	}
	if d.ProjectOnboardingSeenCount != 0 {
		t.Errorf("ProjectOnboardingSeenCount = %d, want 0", d.ProjectOnboardingSeenCount)
	}
}

func TestGlobalConfigSaveAndLoad(t *testing.T) {
	// Source: config.ts:797-866 — saveGlobalConfig / getGlobalConfig
	dir := t.TempDir()
	configFile := filepath.Join(dir, ".claude.json")

	store := NewGlobalConfigStore(configFile)

	// Initial load returns defaults
	cfg := store.Get()
	if cfg.Theme != "dark" {
		t.Errorf("initial Theme = %q, want 'dark'", cfg.Theme)
	}

	// Save with updater
	store.Save(func(c *GlobalConfig) {
		c.Theme = "light"
		c.NumStartups = 5
		c.HasCompletedOnboarding = true
	})

	// Reload and verify
	cfg2 := store.Get()
	if cfg2.Theme != "light" {
		t.Errorf("Theme = %q, want 'light'", cfg2.Theme)
	}
	if cfg2.NumStartups != 5 {
		t.Errorf("NumStartups = %d, want 5", cfg2.NumStartups)
	}
	if !cfg2.HasCompletedOnboarding {
		t.Error("HasCompletedOnboarding should be true")
	}

	// Verify file permissions (0600)
	info, err := os.Stat(configFile)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("permissions = %o, want 0600", perm)
	}
}

func TestGlobalConfigLoadExistingFile(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, ".claude.json")

	// Write a config file with some non-default values
	data := `{
		"numStartups": 42,
		"theme": "light",
		"verbose": true,
		"hasCompletedOnboarding": true
	}`
	os.WriteFile(configFile, []byte(data), 0600)

	store := NewGlobalConfigStore(configFile)
	cfg := store.Get()

	if cfg.NumStartups != 42 {
		t.Errorf("NumStartups = %d, want 42", cfg.NumStartups)
	}
	if cfg.Theme != "light" {
		t.Errorf("Theme = %q, want 'light'", cfg.Theme)
	}
	if !cfg.Verbose {
		t.Error("Verbose should be true")
	}
	// Defaults should still be applied for missing fields
	if cfg.DiffTool != "auto" {
		t.Errorf("DiffTool = %q, want 'auto' (default)", cfg.DiffTool)
	}
	if !cfg.AutoCompactEnabled {
		t.Error("AutoCompactEnabled should default true for missing field")
	}
}

func TestGlobalConfigCorruptedJSON(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, ".claude.json")

	os.WriteFile(configFile, []byte(`{invalid json`), 0600)

	store := NewGlobalConfigStore(configFile)
	cfg := store.Get()

	// Should return defaults when JSON is corrupted
	if cfg.Theme != "dark" {
		t.Errorf("corrupted file should return default theme 'dark', got %q", cfg.Theme)
	}
}

func TestGlobalConfigMissingFile(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, "nonexistent", ".claude.json")

	store := NewGlobalConfigStore(configFile)
	cfg := store.Get()

	// Should return defaults when file doesn't exist
	if cfg.Theme != "dark" {
		t.Errorf("missing file should return default theme 'dark', got %q", cfg.Theme)
	}
}

func TestGlobalConfigProjectsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, ".claude.json")

	store := NewGlobalConfigStore(configFile)
	store.Save(func(c *GlobalConfig) {
		if c.Projects == nil {
			c.Projects = make(map[string]*ProjectConfig)
		}
		c.Projects["/home/user/myproject"] = &ProjectConfig{
			AllowedTools:           []string{"Bash", "Read"},
			HasTrustDialogAccepted: true,
		}
	})

	cfg := store.Get()
	pc, ok := cfg.Projects["/home/user/myproject"]
	if !ok {
		t.Fatal("project config missing")
	}
	if !pc.HasTrustDialogAccepted {
		t.Error("HasTrustDialogAccepted should be true")
	}
	if len(pc.AllowedTools) != 2 {
		t.Errorf("AllowedTools = %v, want [Bash Read]", pc.AllowedTools)
	}
}

func TestConfigWriteDisplayThreshold(t *testing.T) {
	// Source: config.ts:887
	if ConfigWriteDisplayThreshold != 20 {
		t.Errorf("ConfigWriteDisplayThreshold = %d, want 20", ConfigWriteDisplayThreshold)
	}
}

func TestGlobalConfigWriteCount(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, ".claude.json")

	store := NewGlobalConfigStore(configFile)
	if store.WriteCount() != 0 {
		t.Errorf("initial write count = %d, want 0", store.WriteCount())
	}

	store.Save(func(c *GlobalConfig) { c.NumStartups = 1 })
	store.Save(func(c *GlobalConfig) { c.NumStartups = 2 })

	if store.WriteCount() != 2 {
		t.Errorf("write count = %d, want 2", store.WriteCount())
	}
}

func TestAccountInfoJSON(t *testing.T) {
	// Source: config.ts:161-174
	input := `{
		"accountUuid": "acc-123",
		"emailAddress": "user@example.com",
		"organizationUuid": "org-456",
		"organizationName": "Acme Corp",
		"displayName": "Jane Doe"
	}`
	var acct AccountInfo
	if err := json.Unmarshal([]byte(input), &acct); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if acct.AccountUUID != "acc-123" {
		t.Errorf("AccountUUID = %q", acct.AccountUUID)
	}
	if acct.EmailAddress != "user@example.com" {
		t.Errorf("EmailAddress = %q", acct.EmailAddress)
	}
	if acct.OrganizationUUID != "org-456" {
		t.Errorf("OrganizationUUID = %q", acct.OrganizationUUID)
	}
	if acct.DisplayName != "Jane Doe" {
		t.Errorf("DisplayName = %q", acct.DisplayName)
	}
}

func TestIsGlobalConfigKey(t *testing.T) {
	// Source: config.ts:627-666 — GLOBAL_CONFIG_KEYS
	validKeys := []string{
		"theme", "verbose", "editorMode", "autoCompactEnabled",
		"diffTool", "preferredNotifChannel", "respectGitignore",
		"copyFullResponse", "remoteControlAtStartup",
	}
	for _, k := range validKeys {
		if !IsGlobalConfigKey(k) {
			t.Errorf("%q should be a valid global config key", k)
		}
	}

	invalidKeys := []string{"numStartups", "nonexistent", "projects", ""}
	for _, k := range invalidKeys {
		if IsGlobalConfigKey(k) {
			t.Errorf("%q should NOT be a valid global config key", k)
		}
	}
}

func TestIsProjectConfigKey(t *testing.T) {
	// Source: config.ts:674-678 — PROJECT_CONFIG_KEYS
	validKeys := []string{"allowedTools", "hasTrustDialogAccepted", "hasCompletedProjectOnboarding"}
	for _, k := range validKeys {
		if !IsProjectConfigKey(k) {
			t.Errorf("%q should be a valid project config key", k)
		}
	}

	if IsProjectConfigKey("nonexistent") {
		t.Error("'nonexistent' should not be a project config key")
	}
}

func TestIsPathTrusted(t *testing.T) {
	// Source: config.ts:752-761 — isPathTrusted walks parent dirs
	dir := t.TempDir()
	configFile := filepath.Join(dir, ".claude.json")

	store := NewGlobalConfigStore(configFile)

	// Mark parent as trusted
	parentDir := filepath.Join(dir, "workspace")
	childDir := filepath.Join(parentDir, "subdir", "deep")
	os.MkdirAll(childDir, 0755)

	store.Save(func(c *GlobalConfig) {
		if c.Projects == nil {
			c.Projects = make(map[string]*ProjectConfig)
		}
		c.Projects[NormalizePathForConfigKey(parentDir)] = &ProjectConfig{
			HasTrustDialogAccepted: true,
		}
	})

	// Child should be trusted (parent trust propagates)
	if !store.IsPathTrusted(childDir) {
		t.Error("child dir should be trusted via parent")
	}

	// Unrelated path should NOT be trusted
	otherDir := filepath.Join(dir, "other")
	os.MkdirAll(otherDir, 0755)
	if store.IsPathTrusted(otherDir) {
		t.Error("unrelated dir should not be trusted")
	}
}

func TestNormalizePathForConfigKey(t *testing.T) {
	// Source: src/utils/path.ts:149 — normalize + forward slashes
	// On Unix, just normalizes . and ..
	result := NormalizePathForConfigKey("/home/user/../user/project")
	if result != "/home/user/project" {
		t.Errorf("got %q, want '/home/user/project'", result)
	}

	result2 := NormalizePathForConfigKey("/home/user/project/")
	// Should not have trailing slash
	if result2 != "/home/user/project" {
		t.Errorf("got %q, want '/home/user/project'", result2)
	}
}

func TestConfigConstants(t *testing.T) {
	// Source: src/utils/configConstants.ts

	t.Run("notification_channels", func(t *testing.T) {
		expected := []string{
			"auto", "iterm2", "iterm2_with_bell", "terminal_bell",
			"kitty", "ghostty", "notifications_disabled",
		}
		if len(NotificationChannels) != len(expected) {
			t.Fatalf("len = %d, want %d", len(NotificationChannels), len(expected))
		}
		for i, v := range expected {
			if NotificationChannels[i] != v {
				t.Errorf("[%d] = %q, want %q", i, NotificationChannels[i], v)
			}
		}
	})

	t.Run("editor_modes", func(t *testing.T) {
		expected := []string{"normal", "vim"}
		if len(EditorModes) != len(expected) {
			t.Fatalf("len = %d, want %d", len(EditorModes), len(expected))
		}
		for i, v := range expected {
			if EditorModes[i] != v {
				t.Errorf("[%d] = %q, want %q", i, EditorModes[i], v)
			}
		}
	})

	t.Run("teammate_modes", func(t *testing.T) {
		expected := []string{"auto", "tmux", "in-process"}
		if len(TeammateModes) != len(expected) {
			t.Fatalf("len = %d, want %d", len(TeammateModes), len(expected))
		}
		for i, v := range expected {
			if TeammateModes[i] != v {
				t.Errorf("[%d] = %q, want %q", i, TeammateModes[i], v)
			}
		}
	})
}

func TestEnableConfigs(t *testing.T) {
	// Source: config.ts:1334-1356 — enableConfigs lifecycle gate
	dir := t.TempDir()
	configFile := filepath.Join(dir, ".claude.json")
	os.WriteFile(configFile, []byte(`{"numStartups": 10}`), 0600)

	store := NewGlobalConfigStore(configFile)

	// Before enabling, store should work (Go doesn't need the gate but
	// we expose Enable() for parity and early-validation)
	store.Enable()

	cfg := store.Get()
	if cfg.NumStartups != 10 {
		t.Errorf("NumStartups = %d, want 10", cfg.NumStartups)
	}

	// Idempotent
	store.Enable()
}

func TestGetCustomApiKeyStatus(t *testing.T) {
	// Source: config.ts:1103-1114
	dir := t.TempDir()
	configFile := filepath.Join(dir, ".claude.json")

	store := NewGlobalConfigStore(configFile)
	store.Save(func(c *GlobalConfig) {
		c.CustomApiKeyResponses = &CustomApiKeyResponses{
			Approved: []string{"sk-abc...xyz"},
			Rejected: []string{"sk-bad...key"},
		}
	})

	if status := store.GetCustomApiKeyStatus("sk-abc...xyz"); status != "approved" {
		t.Errorf("got %q, want 'approved'", status)
	}
	if status := store.GetCustomApiKeyStatus("sk-bad...key"); status != "rejected" {
		t.Errorf("got %q, want 'rejected'", status)
	}
	if status := store.GetCustomApiKeyStatus("sk-new...key"); status != "new" {
		t.Errorf("got %q, want 'new'", status)
	}
}
