package coordinator

import (
	"os"
	"strings"
	"testing"
)

// --- T21: INTERNAL_WORKER_TOOLS exclusion set ---

func TestInternalWorkerTools_ExactSet(t *testing.T) {
	// Source: coordinatorMode.ts:29-34
	expected := map[string]bool{
		"TeamCreate":      true,
		"TeamDelete":      true,
		"SendMessage":     true,
		"SyntheticOutput": true,
	}
	if len(InternalWorkerTools) != len(expected) {
		t.Fatalf("InternalWorkerTools has %d entries, want %d", len(InternalWorkerTools), len(expected))
	}
	for name := range expected {
		if !InternalWorkerTools[name] {
			t.Errorf("InternalWorkerTools missing %q", name)
		}
	}
}

func TestInternalWorkerTools_ExcludesAgentAndTaskStop(t *testing.T) {
	// Agent and TaskStop are coordinator tools, NOT internal worker tools.
	for _, name := range []string{"Agent", "TaskStop"} {
		if InternalWorkerTools[name] {
			t.Errorf("InternalWorkerTools should NOT contain %q", name)
		}
	}
}

// --- T20: getCoordinatorSystemPrompt ---

func TestGetCoordinatorSystemPrompt_ReturnsEmptyWhenNotCoordinator(t *testing.T) {
	resetEnv(t)
	// No gate checker → not coordinator mode
	got := GetCoordinatorSystemPrompt()
	if got != "" {
		t.Fatalf("expected empty string when not in coordinator mode, got %d chars", len(got))
	}
}

func TestGetCoordinatorSystemPrompt_ReturnsPromptWhenActive(t *testing.T) {
	resetEnv(t)
	FeatureGateChecker = func(gate string) bool { return gate == coordinatorModeGate }
	os.Setenv(coordinatorModeEnv, "1")
	defer os.Unsetenv(coordinatorModeEnv)

	got := GetCoordinatorSystemPrompt()
	if got == "" {
		t.Fatal("expected non-empty coordinator prompt")
	}

	// Verify key sections are present (verbatim headings from TS source)
	requiredSections := []string{
		"## 1. Your Role",
		"## 2. Your Tools",
		"## 3. Workers",
		"## 4. Task Workflow",
		"## 5. Writing Worker Prompts",
		"## 6. Example Session",
	}
	for _, section := range requiredSections {
		if !strings.Contains(got, section) {
			t.Errorf("coordinator prompt missing section %q", section)
		}
	}
}

func TestGetCoordinatorSystemPrompt_ContainsToolNames(t *testing.T) {
	resetEnv(t)
	FeatureGateChecker = func(gate string) bool { return gate == coordinatorModeGate }
	os.Setenv(coordinatorModeEnv, "1")
	defer os.Unsetenv(coordinatorModeEnv)

	got := GetCoordinatorSystemPrompt()

	// All coordinator tool names must appear in the prompt
	for _, name := range []string{"Agent", "SendMessage", "TaskStop"} {
		if !strings.Contains(got, name) {
			t.Errorf("coordinator prompt missing tool name %q", name)
		}
	}
}

func TestGetCoordinatorSystemPrompt_ContainsTaskNotificationXML(t *testing.T) {
	resetEnv(t)
	FeatureGateChecker = func(gate string) bool { return gate == coordinatorModeGate }
	os.Setenv(coordinatorModeEnv, "1")
	defer os.Unsetenv(coordinatorModeEnv)

	got := GetCoordinatorSystemPrompt()

	// T25: task-notification XML contract
	xmlElements := []string{
		"<task-notification>",
		"<task-id>",
		"<status>",
		"<summary>",
		"<result>",
		"<usage>",
		"<total_tokens>",
		"<tool_uses>",
		"<duration_ms>",
	}
	for _, elem := range xmlElements {
		if !strings.Contains(got, elem) {
			t.Errorf("coordinator prompt missing XML element %q", elem)
		}
	}
}

func TestGetCoordinatorSystemPrompt_FullModeWorkerCapabilities(t *testing.T) {
	resetEnv(t)
	FeatureGateChecker = func(gate string) bool { return gate == coordinatorModeGate }
	os.Setenv(coordinatorModeEnv, "1")
	defer os.Unsetenv(coordinatorModeEnv)
	os.Unsetenv("CLAUDE_CODE_SIMPLE")

	got := GetCoordinatorSystemPrompt()
	expected := "Workers have access to standard tools, MCP tools from configured MCP servers, and project skills via the Skill tool. Delegate skill invocations (e.g. /commit, /verify) to workers."
	if !strings.Contains(got, expected) {
		t.Error("full mode prompt should contain standard worker capabilities string")
	}
}

func TestGetCoordinatorSystemPrompt_SimpleModeWorkerCapabilities(t *testing.T) {
	resetEnv(t)
	FeatureGateChecker = func(gate string) bool { return gate == coordinatorModeGate }
	os.Setenv(coordinatorModeEnv, "1")
	defer os.Unsetenv(coordinatorModeEnv)
	os.Setenv("CLAUDE_CODE_SIMPLE", "1")
	defer os.Unsetenv("CLAUDE_CODE_SIMPLE")

	got := GetCoordinatorSystemPrompt()
	expected := "Workers have access to Bash, Read, and Edit tools, plus MCP tools from configured MCP servers."
	if !strings.Contains(got, expected) {
		t.Error("simple mode prompt should contain simple worker capabilities string")
	}
}

func TestGetCoordinatorSystemPrompt_RoleDescription(t *testing.T) {
	resetEnv(t)
	FeatureGateChecker = func(gate string) bool { return gate == coordinatorModeGate }
	os.Setenv(coordinatorModeEnv, "1")
	defer os.Unsetenv(coordinatorModeEnv)

	got := GetCoordinatorSystemPrompt()
	if !strings.Contains(got, "You are Claude Code, an AI assistant that orchestrates software engineering tasks across multiple workers.") {
		t.Error("prompt should start with the coordinator role description")
	}
}

func TestGetCoordinatorSystemPrompt_ContainsExampleSession(t *testing.T) {
	resetEnv(t)
	FeatureGateChecker = func(gate string) bool { return gate == coordinatorModeGate }
	os.Setenv(coordinatorModeEnv, "1")
	defer os.Unsetenv(coordinatorModeEnv)

	got := GetCoordinatorSystemPrompt()
	// Key verbatim strings from the example session
	if !strings.Contains(got, "Investigate auth bug") {
		t.Error("prompt should contain example agent description")
	}
	if !strings.Contains(got, "null pointer in src/auth/validate.ts:42") {
		t.Error("prompt should contain example finding")
	}
}
