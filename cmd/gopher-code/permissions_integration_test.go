package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/permissions"
)

// TestBuildPermissionPolicy_DefaultBypass verifies that buildPermissionPolicy
// with ModeBypassPermissions creates a WaterfallPolicy that allows all tools.
func TestBuildPermissionPolicy_DefaultBypass(t *testing.T) {
	dir := t.TempDir()
	permCtx, policy := buildPermissionPolicy(permissions.ModeBypassPermissions, dir)

	if permCtx == nil {
		t.Fatal("expected non-nil PermissionContext")
	}
	if policy == nil {
		t.Fatal("expected non-nil PermissionPolicy")
	}

	// Bypass mode should allow any tool
	decision := policy.Check(context.Background(), "Bash", "rm -rf /")
	if _, ok := decision.(permissions.AllowDecision); !ok {
		t.Errorf("expected AllowDecision in bypass mode, got %T", decision)
	}
}

// TestBuildPermissionPolicy_DenyRulesFromSettings verifies that deny rules
// loaded from project settings.json cause DenyDecision for matching tools.
func TestBuildPermissionPolicy_DenyRulesFromSettings(t *testing.T) {
	dir := t.TempDir()

	// Create project settings with a deny rule for Bash
	claudeDir := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}
	settings := map[string]interface{}{
		"permissions": map[string]interface{}{
			"deny": []string{"Bash"},
		},
	}
	data, _ := json.Marshal(settings)
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	// Use default mode so deny rules are evaluated before bypass
	_, policy := buildPermissionPolicy(permissions.ModeDefault, dir)

	// Bash should be denied by the project deny rule
	decision := policy.Check(context.Background(), "Bash", "ls")
	if d, ok := decision.(permissions.DenyDecision); !ok {
		t.Errorf("expected DenyDecision for denied Bash tool, got %T: %v", decision, decision)
	} else if d.Reason == "" {
		t.Error("expected non-empty deny reason")
	}

	// Read tool should not be denied (it's not in deny rules)
	decision2 := policy.Check(context.Background(), "Read", "/tmp/file.txt")
	if _, ok := decision2.(permissions.DenyDecision); ok {
		t.Errorf("Read tool should not be denied, got DenyDecision")
	}
}

// TestBuildPermissionPolicy_AllowRulesFromSettings verifies that allow rules
// loaded from settings cause AllowDecision for matching tools in default mode.
func TestBuildPermissionPolicy_AllowRulesFromSettings(t *testing.T) {
	dir := t.TempDir()

	// Create project settings with an allow rule for Edit
	claudeDir := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}
	settings := map[string]interface{}{
		"permissions": map[string]interface{}{
			"allow": []string{"Edit"},
		},
	}
	data, _ := json.Marshal(settings)
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	_, policy := buildPermissionPolicy(permissions.ModeDefault, dir)

	// Edit should be allowed by the allow rule
	decision := policy.Check(context.Background(), "Edit", "some-file.go")
	if _, ok := decision.(permissions.AllowDecision); !ok {
		t.Errorf("expected AllowDecision for allowed Edit tool, got %T: %v", decision, decision)
	}
}

// TestBuildPermissionPolicy_PermissionContextTracksState verifies that the
// PermissionContext returned by buildPermissionPolicy supports state tracking.
func TestBuildPermissionPolicy_PermissionContextTracksState(t *testing.T) {
	dir := t.TempDir()
	permCtx, _ := buildPermissionPolicy(permissions.ModeDefault, dir)

	// Initially no tools are session-allowed
	if permCtx.IsSessionAllowed("Bash") {
		t.Error("Bash should not be session-allowed initially")
	}

	// Mark allowed and verify
	permCtx.MarkAllowed("Bash")
	if !permCtx.IsSessionAllowed("Bash") {
		t.Error("Bash should be session-allowed after MarkAllowed")
	}

	// Record a decision and verify audit trail
	permCtx.RecordDecision("Bash", "tu-1", "accept", "user_temporary")
	decisions := permCtx.GetDecisions()
	if len(decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(decisions))
	}
	if decisions[0].ToolName != "Bash" || decisions[0].Decision != "accept" {
		t.Errorf("unexpected decision: %+v", decisions[0])
	}
}

// TestBuildPermissionPolicy_ModeAcceptEdits verifies that acceptEdits mode
// auto-approves file tools but prompts for Bash.
func TestBuildPermissionPolicy_ModeAcceptEdits(t *testing.T) {
	dir := t.TempDir()
	_, policy := buildPermissionPolicy(permissions.ModeAcceptEdits, dir)

	// File tools should be allowed
	for _, tool := range []string{"Edit", "Write", "Read", "Glob", "Grep"} {
		decision := policy.Check(context.Background(), tool, "file.txt")
		if _, ok := decision.(permissions.AllowDecision); !ok {
			t.Errorf("expected AllowDecision for %s in acceptEdits mode, got %T", tool, decision)
		}
	}

	// Bash should require asking
	decision := policy.Check(context.Background(), "Bash", "ls")
	if _, ok := decision.(permissions.AskDecision); !ok {
		t.Errorf("expected AskDecision for Bash in acceptEdits mode, got %T", decision)
	}
}

// TestBuildPermissionPolicy_DangerousStripped verifies that dangerous bash
// allow rules are stripped in auto/bypass modes.
func TestBuildPermissionPolicy_DangerousStripped(t *testing.T) {
	dir := t.TempDir()

	// Create project settings with a dangerous bash allow rule
	claudeDir := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}
	settings := map[string]interface{}{
		"permissions": map[string]interface{}{
			"allow": []string{"Bash"},
		},
	}
	data, _ := json.Marshal(settings)
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	// In auto mode, the dangerous "Bash" allow rule should be stripped.
	_, policy := buildPermissionPolicy(permissions.ModeAuto, dir)
	wp, ok := policy.(*permissions.WaterfallPolicy)
	if !ok {
		t.Fatalf("expected *WaterfallPolicy, got %T", policy)
	}

	// Verify dangerous Bash rule was stripped from allow rules
	for _, rule := range wp.AllowRules {
		if rule.ToolName == "Bash" && rule.RuleContent == "" {
			t.Error("dangerous Bash allow rule should have been stripped")
		}
	}
}
