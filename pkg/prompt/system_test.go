package prompt

import (
	"os"
	"strings"
	"testing"
)

func TestDefaultSystemPrompt_ContainsCyberRiskInstruction(t *testing.T) {
	prompt := DefaultSystemPrompt()
	if !strings.Contains(prompt, "IMPORTANT: Assist with authorized security testing") {
		t.Error("DefaultSystemPrompt should contain the cyber risk instruction")
	}
	if !strings.Contains(prompt, "Refuse requests for destructive techniques") {
		t.Error("DefaultSystemPrompt should contain the cyber risk refusal text")
	}
}

func TestDefaultSystemPrompt_ContainsURLRestriction(t *testing.T) {
	prompt := DefaultSystemPrompt()
	if !strings.Contains(prompt, "NEVER generate or guess URLs") {
		t.Error("DefaultSystemPrompt should contain URL restriction")
	}
}

func TestBuildSystemPrompt_IncludesEnvironment(t *testing.T) {
	prompt := BuildSystemPrompt("", "/tmp/test", "claude-sonnet-4-6")
	if !strings.Contains(prompt, "# Environment") {
		t.Error("BuildSystemPrompt should include environment section")
	}
	if !strings.Contains(prompt, "/tmp/test") {
		t.Error("BuildSystemPrompt should include working directory")
	}
	if !strings.Contains(prompt, "claude-sonnet-4-6") {
		t.Error("BuildSystemPrompt should include model name")
	}
}

func TestBuildSystemPrompt_CustomBase(t *testing.T) {
	prompt := BuildSystemPrompt("Custom system prompt", "/tmp", "model")
	if !strings.Contains(prompt, "Custom system prompt") {
		t.Error("BuildSystemPrompt should use custom base prompt")
	}
	// Custom base should NOT include default prompt
	if strings.Contains(prompt, "IMPORTANT: Assist with authorized security testing") {
		t.Error("Custom base prompt should replace default, not append")
	}
}

// ── T381: System prompt prefix constants ─────────────────────────

func TestDefaultPrefix_VerbatimMatch(t *testing.T) {
	const want = "You are Claude Code, Anthropic's official CLI for Claude."
	if DefaultPrefix != want {
		t.Errorf("DefaultPrefix = %q, want %q", DefaultPrefix, want)
	}
}

func TestAgentSDKClaudeCodePresetPrefix_VerbatimMatch(t *testing.T) {
	const want = "You are Claude Code, Anthropic's official CLI for Claude, running within the Claude Agent SDK."
	if AgentSDKClaudeCodePresetPrefix != want {
		t.Errorf("AgentSDKClaudeCodePresetPrefix = %q, want %q", AgentSDKClaudeCodePresetPrefix, want)
	}
}

func TestAgentSDKPrefix_VerbatimMatch(t *testing.T) {
	const want = "You are a Claude agent, built on Anthropic's Claude Agent SDK."
	if AgentSDKPrefix != want {
		t.Errorf("AgentSDKPrefix = %q, want %q", AgentSDKPrefix, want)
	}
}

func TestCLISyspromptPrefixes_ContainsAll(t *testing.T) {
	for _, pfx := range []string{DefaultPrefix, AgentSDKClaudeCodePresetPrefix, AgentSDKPrefix} {
		if _, ok := CLISyspromptPrefixes[pfx]; !ok {
			t.Errorf("CLISyspromptPrefixes missing %q", pfx)
		}
	}
	if len(CLISyspromptPrefixes) != 3 {
		t.Errorf("CLISyspromptPrefixes has %d entries, want 3", len(CLISyspromptPrefixes))
	}
}

// ── T381: getCLISyspromptPrefix 3-way selector ──────────────────

func TestGetCLISyspromptPrefix_DefaultInteractive(t *testing.T) {
	// No vertex env, nil options → default
	os.Unsetenv("CLAUDE_CODE_USE_VERTEX")
	got := GetCLISyspromptPrefix(nil)
	if got != DefaultPrefix {
		t.Errorf("got %q, want DefaultPrefix", got)
	}
}

func TestGetCLISyspromptPrefix_InteractiveExplicit(t *testing.T) {
	os.Unsetenv("CLAUDE_CODE_USE_VERTEX")
	got := GetCLISyspromptPrefix(&PrefixOptions{IsNonInteractive: false})
	if got != DefaultPrefix {
		t.Errorf("got %q, want DefaultPrefix", got)
	}
}

func TestGetCLISyspromptPrefix_Vertex(t *testing.T) {
	t.Setenv("CLAUDE_CODE_USE_VERTEX", "1")
	// Even with non-interactive, vertex → default
	got := GetCLISyspromptPrefix(&PrefixOptions{IsNonInteractive: true, HasAppendSystemPrompt: true})
	if got != DefaultPrefix {
		t.Errorf("vertex should always return DefaultPrefix, got %q", got)
	}
}

func TestGetCLISyspromptPrefix_NonInteractiveWithAppend(t *testing.T) {
	os.Unsetenv("CLAUDE_CODE_USE_VERTEX")
	got := GetCLISyspromptPrefix(&PrefixOptions{IsNonInteractive: true, HasAppendSystemPrompt: true})
	if got != AgentSDKClaudeCodePresetPrefix {
		t.Errorf("got %q, want AgentSDKClaudeCodePresetPrefix", got)
	}
}

func TestGetCLISyspromptPrefix_NonInteractiveNoAppend(t *testing.T) {
	os.Unsetenv("CLAUDE_CODE_USE_VERTEX")
	got := GetCLISyspromptPrefix(&PrefixOptions{IsNonInteractive: true, HasAppendSystemPrompt: false})
	if got != AgentSDKPrefix {
		t.Errorf("got %q, want AgentSDKPrefix", got)
	}
}

// ── T381: getAttributionHeader ──────────────────────────────────

func TestGetAttributionHeader_Basic(t *testing.T) {
	t.Setenv("CLAUDE_CODE_ENTRYPOINT", "cli")
	os.Unsetenv("CLAUDE_CODE_ATTRIBUTION_HEADER")

	got := GetAttributionHeader("abc123", AttributionConfig{Version: "1.0.0"})
	want := "x-anthropic-billing-header: cc_version=1.0.0.abc123; cc_entrypoint=cli;"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestGetAttributionHeader_DefaultEntrypoint(t *testing.T) {
	os.Unsetenv("CLAUDE_CODE_ENTRYPOINT")
	os.Unsetenv("CLAUDE_CODE_ATTRIBUTION_HEADER")

	got := GetAttributionHeader("fp", AttributionConfig{Version: "2.0"})
	if !strings.Contains(got, "cc_entrypoint=unknown;") {
		t.Errorf("expected default entrypoint 'unknown', got %q", got)
	}
}

func TestGetAttributionHeader_EnvDisabled(t *testing.T) {
	t.Setenv("CLAUDE_CODE_ATTRIBUTION_HEADER", "false")
	got := GetAttributionHeader("fp", AttributionConfig{Version: "1.0"})
	if got != "" {
		t.Errorf("expected empty when env disabled, got %q", got)
	}
}

func TestGetAttributionHeader_EnvDisabledZero(t *testing.T) {
	t.Setenv("CLAUDE_CODE_ATTRIBUTION_HEADER", "0")
	got := GetAttributionHeader("fp", AttributionConfig{Version: "1.0"})
	if got != "" {
		t.Errorf("expected empty when env=0, got %q", got)
	}
}

func TestGetAttributionHeader_GrowthBookKillswitch(t *testing.T) {
	os.Unsetenv("CLAUDE_CODE_ATTRIBUTION_HEADER")
	got := GetAttributionHeader("fp", AttributionConfig{
		Version:           "1.0",
		GrowthBookEnabled: func() bool { return false },
	})
	if got != "" {
		t.Errorf("expected empty when growthbook disabled, got %q", got)
	}
}

func TestGetAttributionHeader_GrowthBookNilDefaultsEnabled(t *testing.T) {
	os.Unsetenv("CLAUDE_CODE_ATTRIBUTION_HEADER")
	os.Unsetenv("CLAUDE_CODE_ENTRYPOINT")
	got := GetAttributionHeader("fp", AttributionConfig{Version: "1.0"})
	if got == "" {
		t.Error("nil GrowthBookEnabled should default to enabled")
	}
}

func TestGetAttributionHeader_WithNativeAttestation(t *testing.T) {
	os.Unsetenv("CLAUDE_CODE_ATTRIBUTION_HEADER")
	t.Setenv("CLAUDE_CODE_ENTRYPOINT", "cli")

	got := GetAttributionHeader("fp", AttributionConfig{
		Version:                 "1.0",
		NativeClientAttestation: true,
	})
	if !strings.Contains(got, " cch=00000;") {
		t.Errorf("expected cch placeholder, got %q", got)
	}
}

func TestGetAttributionHeader_WithoutNativeAttestation(t *testing.T) {
	os.Unsetenv("CLAUDE_CODE_ATTRIBUTION_HEADER")
	t.Setenv("CLAUDE_CODE_ENTRYPOINT", "cli")

	got := GetAttributionHeader("fp", AttributionConfig{
		Version:                 "1.0",
		NativeClientAttestation: false,
	})
	if strings.Contains(got, "cch=") {
		t.Errorf("should not contain cch, got %q", got)
	}
}

func TestGetAttributionHeader_WithWorkload(t *testing.T) {
	os.Unsetenv("CLAUDE_CODE_ATTRIBUTION_HEADER")
	t.Setenv("CLAUDE_CODE_ENTRYPOINT", "cli")

	got := GetAttributionHeader("fp", AttributionConfig{
		Version:     "1.0",
		GetWorkload: func() string { return "cron" },
	})
	if !strings.Contains(got, " cc_workload=cron;") {
		t.Errorf("expected cc_workload=cron, got %q", got)
	}
}

func TestGetAttributionHeader_EmptyWorkload(t *testing.T) {
	os.Unsetenv("CLAUDE_CODE_ATTRIBUTION_HEADER")
	t.Setenv("CLAUDE_CODE_ENTRYPOINT", "cli")

	got := GetAttributionHeader("fp", AttributionConfig{
		Version:     "1.0",
		GetWorkload: func() string { return "" },
	})
	if strings.Contains(got, "cc_workload") {
		t.Errorf("empty workload should be omitted, got %q", got)
	}
}

func TestGetAttributionHeader_FullTemplate(t *testing.T) {
	os.Unsetenv("CLAUDE_CODE_ATTRIBUTION_HEADER")
	t.Setenv("CLAUDE_CODE_ENTRYPOINT", "sdk")

	got := GetAttributionHeader("abcdef", AttributionConfig{
		Version:                 "3.2.1",
		NativeClientAttestation: true,
		GetWorkload:             func() string { return "batch" },
	})
	want := "x-anthropic-billing-header: cc_version=3.2.1.abcdef; cc_entrypoint=sdk; cch=00000; cc_workload=batch;"
	if got != want {
		t.Errorf("full template:\ngot  %q\nwant %q", got, want)
	}
}

// ── T381: isEnvDefinedFalsy helper ──────────────────────────────

func TestIsEnvDefinedFalsy(t *testing.T) {
	tests := []struct {
		name   string
		set    bool
		val    string
		expect bool
	}{
		{"unset", false, "", false},
		{"empty", true, "", true},
		{"zero", true, "0", true},
		{"false", true, "false", true},
		{"FALSE", true, "FALSE", true},
		{"no", true, "no", true},
		{"true", true, "true", false},
		{"1", true, "1", false},
		{"yes", true, "yes", false},
		{"arbitrary", true, "anything", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.set {
				t.Setenv("TEST_ENV_FALSY", tt.val)
			} else {
				os.Unsetenv("TEST_ENV_FALSY")
			}
			got := isEnvDefinedFalsy("TEST_ENV_FALSY")
			if got != tt.expect {
				t.Errorf("isEnvDefinedFalsy(%q) = %v, want %v", tt.val, got, tt.expect)
			}
		})
	}
}

func TestCyberRiskInstruction_Constant(t *testing.T) {
	// Verify the constant matches the TS source exactly
	if !strings.HasPrefix(CyberRiskInstruction, "IMPORTANT: Assist with authorized security testing") {
		t.Error("CyberRiskInstruction should start with the expected text")
	}
	if !strings.Contains(CyberRiskInstruction, "pentesting engagements, CTF competitions, security research, or defensive use cases") {
		t.Error("CyberRiskInstruction should contain the authorization context list")
	}
}
