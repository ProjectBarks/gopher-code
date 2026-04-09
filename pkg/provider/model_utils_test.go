package provider

import "testing"

func TestResolveModelAlias(t *testing.T) {
	// Family aliases
	if m := ResolveModelAlias("sonnet"); m == "sonnet" {
		t.Error("sonnet alias should resolve to a model ID")
	}
	if m := ResolveModelAlias("opus"); m == "opus" {
		t.Error("opus alias should resolve to a model ID")
	}
	if m := ResolveModelAlias("haiku"); m == "haiku" {
		t.Error("haiku alias should resolve to a model ID")
	}

	// Non-alias should pass through
	if m := ResolveModelAlias("claude-sonnet-4-6-20250514"); m != "claude-sonnet-4-6-20250514" {
		t.Errorf("non-alias should pass through, got %q", m)
	}
}

func TestIsModelAllowed_NoAllowlist(t *testing.T) {
	SetModelAllowlist(nil) // clear
	if !IsModelAllowed("any-model") {
		t.Error("should allow all when no allowlist")
	}
}

func TestIsModelAllowed_WithAllowlist(t *testing.T) {
	SetModelAllowlist([]string{"claude-sonnet-4-6-20250514", "claude-opus-4-6-20250514"})
	defer SetModelAllowlist(nil)

	if !IsModelAllowed("claude-sonnet-4-6-20250514") {
		t.Error("should allow listed model")
	}
	if IsModelAllowed("gpt-4") {
		t.Error("should reject non-listed model")
	}
}

func TestModelSupports1M(t *testing.T) {
	if !ModelSupports1M("claude-opus-4-6-20250514") {
		t.Error("opus-4-6 should support 1M")
	}
	if !ModelSupports1M("claude-sonnet-4-6-20250514") {
		t.Error("sonnet-4-6 should support 1M")
	}
	if ModelSupports1M("claude-haiku-4-5") {
		t.Error("haiku should not support 1M")
	}
}

func TestGetModelCapabilities(t *testing.T) {
	caps := GetModelCapabilities("claude-opus-4-6-20250514")
	if !caps.Effort {
		t.Error("opus-4-6 should support effort")
	}
	if !caps.MaxEffort {
		t.Error("opus-4-6 should support max effort")
	}
	if !caps.Context1M {
		t.Error("opus-4-6 should support 1M context")
	}
	if !caps.AutoMode {
		t.Error("opus-4-6 should support auto mode")
	}
}

func TestGetDefaultMainLoopModel(t *testing.T) {
	m := GetDefaultMainLoopModel()
	if m == "" {
		t.Error("default model should not be empty")
	}
}

func TestGetUserSpecifiedModel_Default(t *testing.T) {
	t.Setenv("ANTHROPIC_MODEL", "")
	if m := GetUserSpecifiedModel(); m != "" {
		t.Errorf("should be empty without env, got %q", m)
	}
}

func TestGetUserSpecifiedModel_FromEnv(t *testing.T) {
	t.Setenv("ANTHROPIC_MODEL", "sonnet")
	SetModelAllowlist(nil) // allow all
	m := GetUserSpecifiedModel()
	if m == "" || m == "sonnet" {
		t.Errorf("should resolve alias, got %q", m)
	}
}
