package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// --- Test: defaults output ---

func TestAutoModeDefaultsOutput(t *testing.T) {
	rules := GetDefaultExternalAutoModeRules()

	// All three sections must be non-empty
	if len(rules.Allow) == 0 {
		t.Fatal("defaults: allow section is empty")
	}
	if len(rules.SoftDeny) == 0 {
		t.Fatal("defaults: soft_deny section is empty")
	}
	if len(rules.Environment) == 0 {
		t.Fatal("defaults: environment section is empty")
	}

	// JSON round-trip: output must be valid 2-space-indented JSON
	data, err := json.MarshalIndent(rules, "", "  ")
	if err != nil {
		t.Fatalf("marshal defaults: %v", err)
	}
	var decoded AutoModeRules
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal defaults: %v", err)
	}
	if len(decoded.Allow) != len(rules.Allow) {
		t.Errorf("round-trip allow: got %d, want %d", len(decoded.Allow), len(rules.Allow))
	}
	if len(decoded.SoftDeny) != len(rules.SoftDeny) {
		t.Errorf("round-trip soft_deny: got %d, want %d", len(decoded.SoftDeny), len(rules.SoftDeny))
	}
	if len(decoded.Environment) != len(rules.Environment) {
		t.Errorf("round-trip environment: got %d, want %d", len(decoded.Environment), len(rules.Environment))
	}
}

func TestWriteRulesJSON(t *testing.T) {
	rules := AutoModeRules{
		Allow:       []string{"read files"},
		SoftDeny:    []string{"delete files"},
		Environment: []string{"dev project"},
	}
	var buf bytes.Buffer
	if err := writeRulesJSON(&buf, rules); err != nil {
		t.Fatalf("writeRulesJSON: %v", err)
	}
	out := buf.String()
	// Must end with newline
	if !strings.HasSuffix(out, "\n") {
		t.Error("output must end with newline")
	}
	// Must be valid JSON
	var decoded AutoModeRules
	if err := json.Unmarshal([]byte(out), &decoded); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	if decoded.Allow[0] != "read files" {
		t.Errorf("allow[0] = %q, want %q", decoded.Allow[0], "read files")
	}
	// 2-space indent check
	if !strings.Contains(out, "  \"allow\"") {
		t.Error("expected 2-space indent in JSON output")
	}
}

// --- Test: config merge (REPLACE semantics) ---

func TestMergeAutoModeConfig_NilConfig(t *testing.T) {
	defaults := AutoModeRules{
		Allow:       []string{"default-a"},
		SoftDeny:    []string{"default-d"},
		Environment: []string{"default-e"},
	}
	merged := MergeAutoModeConfig(nil, defaults)
	if len(merged.Allow) != 1 || merged.Allow[0] != "default-a" {
		t.Errorf("nil config: allow = %v, want [default-a]", merged.Allow)
	}
	if len(merged.SoftDeny) != 1 || merged.SoftDeny[0] != "default-d" {
		t.Errorf("nil config: soft_deny = %v, want [default-d]", merged.SoftDeny)
	}
}

func TestMergeAutoModeConfig_PartialReplace(t *testing.T) {
	defaults := AutoModeRules{
		Allow:       []string{"default-allow"},
		SoftDeny:    []string{"default-deny"},
		Environment: []string{"default-env"},
	}
	cfg := &AutoModeConfig{
		Allow: []string{"user-allow-1", "user-allow-2"},
		// SoftDeny intentionally empty → defaults should remain
		// Environment intentionally empty → defaults should remain
	}
	merged := MergeAutoModeConfig(cfg, defaults)

	// allow: user section replaces defaults
	if len(merged.Allow) != 2 || merged.Allow[0] != "user-allow-1" {
		t.Errorf("partial replace: allow = %v, want [user-allow-1 user-allow-2]", merged.Allow)
	}
	// soft_deny: falls through to defaults
	if len(merged.SoftDeny) != 1 || merged.SoftDeny[0] != "default-deny" {
		t.Errorf("partial replace: soft_deny = %v, want [default-deny]", merged.SoftDeny)
	}
	// environment: falls through to defaults
	if len(merged.Environment) != 1 || merged.Environment[0] != "default-env" {
		t.Errorf("partial replace: environment = %v, want [default-env]", merged.Environment)
	}
}

func TestMergeAutoModeConfig_FullReplace(t *testing.T) {
	defaults := AutoModeRules{
		Allow:       []string{"default-allow"},
		SoftDeny:    []string{"default-deny"},
		Environment: []string{"default-env"},
	}
	cfg := &AutoModeConfig{
		Allow:       []string{"user-allow"},
		SoftDeny:    []string{"user-deny"},
		Environment: []string{"user-env"},
	}
	merged := MergeAutoModeConfig(cfg, defaults)

	if merged.Allow[0] != "user-allow" {
		t.Errorf("full replace: allow[0] = %q", merged.Allow[0])
	}
	if merged.SoftDeny[0] != "user-deny" {
		t.Errorf("full replace: soft_deny[0] = %q", merged.SoftDeny[0])
	}
	if merged.Environment[0] != "user-env" {
		t.Errorf("full replace: environment[0] = %q", merged.Environment[0])
	}
}

// --- Test: critique request format ---

func TestFormatRulesForCritique_EmptyUserRules(t *testing.T) {
	result := FormatRulesForCritique("allow", nil, []string{"default-1"})
	if result != "" {
		t.Errorf("empty user rules should produce empty string, got %q", result)
	}
}

func TestFormatRulesForCritique_WithUserRules(t *testing.T) {
	result := FormatRulesForCritique("allow", []string{"custom-1", "custom-2"}, []string{"default-1"})

	// Must contain section header
	if !strings.Contains(result, "## allow (custom rules replacing defaults)") {
		t.Error("missing section header")
	}
	// Must contain Custom: label
	if !strings.Contains(result, "Custom:\n") {
		t.Error("missing Custom: label")
	}
	// Must contain user rules as bullets
	if !strings.Contains(result, "- custom-1\n") {
		t.Error("missing user rule bullet")
	}
	// Must contain Defaults being replaced: label
	if !strings.Contains(result, "Defaults being replaced:\n") {
		t.Error("missing defaults label")
	}
	// Must contain default rules as bullets
	if !strings.Contains(result, "- default-1\n") {
		t.Error("missing default rule bullet")
	}
}

func TestAutoModeCritiqueHandler_NoCustomRules(t *testing.T) {
	// With nil config, handler should print "no custom rules" message and return nil
	// We can't capture stdout easily here, so just verify no error
	err := AutoModeCritiqueHandler(context.Background(), nil, "test-model", nil)
	if err != nil {
		t.Errorf("expected nil error for no custom rules, got %v", err)
	}
}

func TestAutoModeCritiqueHandler_RequestFormat(t *testing.T) {
	var captured SideQueryOptions
	mockSideQuery := func(_ context.Context, opts SideQueryOptions) (*SideQueryResponse, error) {
		captured = opts
		return &SideQueryResponse{
			Content: []ContentBlock{{Type: "text", Text: "Looks good!"}},
		}, nil
	}

	cfg := &AutoModeConfig{
		Allow: []string{"read files"},
	}

	err := AutoModeCritiqueHandler(context.Background(), cfg, "claude-sonnet-4-6", mockSideQuery)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify request fields
	if captured.QuerySource != "auto_mode_critique" {
		t.Errorf("querySource = %q, want %q", captured.QuerySource, "auto_mode_critique")
	}
	if captured.Model != "claude-sonnet-4-6" {
		t.Errorf("model = %q, want %q", captured.Model, "claude-sonnet-4-6")
	}
	if captured.MaxTokens != 4096 {
		t.Errorf("maxTokens = %d, want 4096", captured.MaxTokens)
	}
	if !captured.SkipSystemPromptPrefix {
		t.Error("skipSystemPromptPrefix should be true")
	}
	if len(captured.Messages) != 1 {
		t.Fatalf("messages count = %d, want 1", len(captured.Messages))
	}
	if captured.Messages[0].Role != "user" {
		t.Errorf("message role = %q, want %q", captured.Messages[0].Role, "user")
	}
	// Message content must include classifier prompt and user rules summary
	content := captured.Messages[0].Content
	if !strings.Contains(content, "<classifier_system_prompt>") {
		t.Error("missing <classifier_system_prompt> tag in message")
	}
	if !strings.Contains(content, "Please critique these custom rules.") {
		t.Error("missing critique instruction in message")
	}
	if !strings.Contains(content, "## allow (custom rules replacing defaults)") {
		t.Error("missing user rules section in message")
	}
	// System prompt must be the critique prompt
	if !strings.Contains(captured.System, "expert reviewer of auto mode classifier rules") {
		t.Error("system prompt doesn't contain expected critique instructions")
	}
}

func TestAutoModeCritiqueHandler_SideQueryError(t *testing.T) {
	mockSideQuery := func(_ context.Context, _ SideQueryOptions) (*SideQueryResponse, error) {
		return nil, fmt.Errorf("network error")
	}

	cfg := &AutoModeConfig{
		Allow: []string{"read files"},
	}

	err := AutoModeCritiqueHandler(context.Background(), cfg, "test-model", mockSideQuery)
	if err == nil {
		t.Fatal("expected error from side query failure")
	}
	if !strings.Contains(err.Error(), "network error") {
		t.Errorf("error = %v, should contain 'network error'", err)
	}
}

func TestAutoModeCritiqueHandler_NoTextBlock(t *testing.T) {
	mockSideQuery := func(_ context.Context, _ SideQueryOptions) (*SideQueryResponse, error) {
		return &SideQueryResponse{
			Content: []ContentBlock{{Type: "tool_use"}},
		}, nil
	}

	cfg := &AutoModeConfig{
		SoftDeny: []string{"dangerous stuff"},
	}

	// Should not error, just print "no critique" message
	err := AutoModeCritiqueHandler(context.Background(), cfg, "test-model", mockSideQuery)
	if err != nil {
		t.Errorf("expected nil error for no text block, got %v", err)
	}
}

// --- Test: BuildDefaultExternalSystemPrompt ---

func TestBuildDefaultExternalSystemPrompt(t *testing.T) {
	prompt := BuildDefaultExternalSystemPrompt()

	// Should not contain any XML tags from the template
	if strings.Contains(prompt, "<user_allow_rules_to_replace>") {
		t.Error("prompt still contains <user_allow_rules_to_replace> tag")
	}
	if strings.Contains(prompt, "<user_deny_rules_to_replace>") {
		t.Error("prompt still contains <user_deny_rules_to_replace> tag")
	}
	if strings.Contains(prompt, "<user_environment_to_replace>") {
		t.Error("prompt still contains <user_environment_to_replace> tag")
	}

	// Should contain the actual rule content
	if !strings.Contains(prompt, "Read files") {
		t.Error("prompt missing default allow rules content")
	}
}

// --- Test: extractTaggedBullets ---

func TestExtractTaggedBullets(t *testing.T) {
	allow := extractTaggedBullets("user_allow_rules_to_replace")
	if len(allow) == 0 {
		t.Fatal("extractTaggedBullets returned empty for allow")
	}
	// Each bullet must not have the "- " prefix
	for _, b := range allow {
		if strings.HasPrefix(b, "- ") {
			t.Errorf("bullet still has '- ' prefix: %q", b)
		}
	}
}
