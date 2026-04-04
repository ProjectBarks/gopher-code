package prompt

import (
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

func TestCyberRiskInstruction_Constant(t *testing.T) {
	// Verify the constant matches the TS source exactly
	if !strings.HasPrefix(CyberRiskInstruction, "IMPORTANT: Assist with authorized security testing") {
		t.Error("CyberRiskInstruction should start with the expected text")
	}
	if !strings.Contains(CyberRiskInstruction, "pentesting engagements, CTF competitions, security research, or defensive use cases") {
		t.Error("CyberRiskInstruction should contain the authorization context list")
	}
}
