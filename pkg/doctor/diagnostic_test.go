package doctor

import (
	"errors"
	"testing"

	uidoctor "github.com/projectbarks/gopher-code/pkg/ui/doctor"
)

func TestCollect_BasicFields(t *testing.T) {
	data := Collect(CollectOptions{})
	if data.Version == "" {
		t.Error("expected non-empty version")
	}
	if data.InstallationType != "go-binary" {
		t.Errorf("expected go-binary, got %q", data.InstallationType)
	}
	if data.InstallationPath == "" {
		t.Error("expected non-empty installation path")
	}
	if data.InvokedBinary == "" {
		t.Error("expected non-empty invoked binary")
	}
	if data.AutoUpdates != "enabled" {
		t.Errorf("expected enabled, got %q", data.AutoUpdates)
	}
}

func TestCollect_WithDistTags(t *testing.T) {
	data := Collect(CollectOptions{
		FetchDistTags: func() (*uidoctor.DistTags, error) {
			return &uidoctor.DistTags{Stable: "1.0.0", Latest: "1.1.0"}, nil
		},
	})
	if data.DistTags == nil {
		t.Fatal("expected dist tags")
	}
	if data.DistTags.Stable != "1.0.0" {
		t.Errorf("expected 1.0.0, got %q", data.DistTags.Stable)
	}
	if data.DistTagsErr != nil {
		t.Errorf("expected no error, got %v", data.DistTagsErr)
	}
}

func TestCollect_DistTagsError(t *testing.T) {
	data := Collect(CollectOptions{
		FetchDistTags: func() (*uidoctor.DistTags, error) {
			return nil, errors.New("network error")
		},
	})
	if data.DistTagsErr == nil {
		t.Error("expected dist tags error")
	}
}

func TestCollect_EnvValidation(t *testing.T) {
	// With no env vars set, should return no validation results
	data := Collect(CollectOptions{})
	// Results may or may not be empty depending on env, but should not panic
	_ = data.EnvValidation
}

func TestCollect_SandboxDetected(t *testing.T) {
	data := Collect(CollectOptions{})
	// Should always have sandbox status (may be available or not depending on OS)
	if data.Sandbox.Type == "" {
		t.Error("expected non-empty sandbox type")
	}
}

func TestCollect_PassthroughFields(t *testing.T) {
	opts := CollectOptions{
		SettingsErrors: []uidoctor.SettingsError{
			{Path: "model", Message: "bad"},
		},
		KeybindingWarnings: []uidoctor.KeybindingWarning{
			{Key: "ctrl+x", Message: "unknown"},
		},
		MCPWarnings: []uidoctor.MCPParsingWarning{
			{Server: "s1", Message: "bad config"},
		},
		ContextWarnings: &uidoctor.ContextWarnings{
			ClaudeMDWarning: &uidoctor.ContextWarning{Message: "too big"},
		},
		VersionLocks: &uidoctor.VersionLockInfo{Enabled: true},
		AgentInfo:    &uidoctor.AgentInfo{UserAgentsDir: "/agents"},
	}
	data := Collect(opts)

	if len(data.SettingsErrors) != 1 {
		t.Errorf("expected 1 settings error, got %d", len(data.SettingsErrors))
	}
	if len(data.KeybindingWarnings) != 1 {
		t.Errorf("expected 1 keybinding warning, got %d", len(data.KeybindingWarnings))
	}
	if len(data.MCPWarnings) != 1 {
		t.Errorf("expected 1 MCP warning, got %d", len(data.MCPWarnings))
	}
	if data.ContextWarnings == nil || data.ContextWarnings.ClaudeMDWarning == nil {
		t.Error("expected context warnings passthrough")
	}
	if data.VersionLocks == nil || !data.VersionLocks.Enabled {
		t.Error("expected version locks passthrough")
	}
	if data.AgentInfo == nil || data.AgentInfo.UserAgentsDir != "/agents" {
		t.Error("expected agent info passthrough")
	}
}
