package handlers

import (
	"bytes"
	"strings"
	"testing"
)

func TestSetupToken_AuthConflictWarning(t *testing.T) {
	t.Run("shows warning when auth already configured", func(t *testing.T) {
		var buf bytes.Buffer
		code := SetupToken(SetupTokenOpts{
			Output:      &buf,
			AuthChecker: func() bool { return false }, // auth NOT enabled → conflict
		})
		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
		out := buf.String()
		if !strings.Contains(out, SetupTokenAuthWarning1) {
			t.Errorf("missing auth warning line 1 in output:\n%s", out)
		}
		if !strings.Contains(out, SetupTokenAuthWarning2) {
			t.Errorf("missing auth warning line 2 in output:\n%s", out)
		}
	})

	t.Run("no warning when no external auth", func(t *testing.T) {
		var buf bytes.Buffer
		code := SetupToken(SetupTokenOpts{
			Output:      &buf,
			AuthChecker: func() bool { return true }, // auth enabled → no conflict
		})
		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
		out := buf.String()
		if strings.Contains(out, "Warning") {
			t.Errorf("unexpected warning in output:\n%s", out)
		}
	})
}

func TestSetupToken_StartingMessage(t *testing.T) {
	var buf bytes.Buffer
	SetupToken(SetupTokenOpts{
		Output:      &buf,
		AuthChecker: func() bool { return true },
	})
	out := buf.String()
	if !strings.Contains(out, SetupTokenStarting) {
		t.Errorf("missing starting message in output:\n%s", out)
	}
}

func TestSetupToken_AuthChecker_EnvVar(t *testing.T) {
	t.Run("ANTHROPIC_API_KEY disables auth", func(t *testing.T) {
		t.Setenv("ANTHROPIC_API_KEY", "sk-test-key")
		// Clear other env vars that might interfere
		t.Setenv("CLAUDE_CODE_USE_BEDROCK", "")
		t.Setenv("CLAUDE_CODE_USE_VERTEX", "")
		t.Setenv("CLAUDE_CODE_USE_FOUNDRY", "")
		t.Setenv("ANTHROPIC_AUTH_TOKEN", "")

		if isAnthropicAuthEnabled() {
			t.Error("expected isAnthropicAuthEnabled()=false when ANTHROPIC_API_KEY set")
		}
	})

	t.Run("3P provider disables auth", func(t *testing.T) {
		t.Setenv("ANTHROPIC_API_KEY", "")
		t.Setenv("CLAUDE_CODE_USE_BEDROCK", "1")
		t.Setenv("CLAUDE_CODE_USE_VERTEX", "")
		t.Setenv("CLAUDE_CODE_USE_FOUNDRY", "")
		t.Setenv("ANTHROPIC_AUTH_TOKEN", "")

		if isAnthropicAuthEnabled() {
			t.Error("expected isAnthropicAuthEnabled()=false when CLAUDE_CODE_USE_BEDROCK=1")
		}
	})

	t.Run("clean env enables auth", func(t *testing.T) {
		t.Setenv("ANTHROPIC_API_KEY", "")
		t.Setenv("CLAUDE_CODE_USE_BEDROCK", "")
		t.Setenv("CLAUDE_CODE_USE_VERTEX", "")
		t.Setenv("CLAUDE_CODE_USE_FOUNDRY", "")
		t.Setenv("ANTHROPIC_AUTH_TOKEN", "")

		if !isAnthropicAuthEnabled() {
			t.Error("expected isAnthropicAuthEnabled()=true with clean env")
		}
	})
}
