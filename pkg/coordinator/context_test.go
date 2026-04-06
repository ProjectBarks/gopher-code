package coordinator

import (
	"os"
	"strings"
	"testing"
)

// --- T19: getCoordinatorUserContext ---

func TestGetCoordinatorUserContext_ReturnsNilWhenNotCoordinator(t *testing.T) {
	resetEnv(t)
	got := GetCoordinatorUserContext(nil, "")
	if got != nil {
		t.Fatalf("expected nil when not in coordinator mode, got %v", got)
	}
}

func TestGetCoordinatorUserContext_ReturnsWorkerToolsContext(t *testing.T) {
	resetEnv(t)
	FeatureGateChecker = func(gate string) bool { return gate == coordinatorModeGate }
	os.Setenv(coordinatorModeEnv, "1")
	defer os.Unsetenv(coordinatorModeEnv)

	got := GetCoordinatorUserContext(nil, "")
	if got == nil {
		t.Fatal("expected non-nil result in coordinator mode")
	}

	ctx, ok := got["workerToolsContext"]
	if !ok {
		t.Fatal("result should have workerToolsContext key")
	}
	if !strings.Contains(ctx, "Workers spawned via the Agent tool have access to these tools:") {
		t.Errorf("workerToolsContext missing preamble, got: %s", ctx)
	}
}

func TestGetCoordinatorUserContext_FullMode_ExcludesInternalTools(t *testing.T) {
	resetEnv(t)
	FeatureGateChecker = func(gate string) bool { return gate == coordinatorModeGate }
	os.Setenv(coordinatorModeEnv, "1")
	defer os.Unsetenv(coordinatorModeEnv)
	os.Unsetenv("CLAUDE_CODE_SIMPLE")

	got := GetCoordinatorUserContext(nil, "")
	ctx := got["workerToolsContext"]

	// Internal worker tools must NOT appear
	for name := range InternalWorkerTools {
		// Check that the tool name doesn't appear as a standalone item in the comma-separated list
		if strings.Contains(ctx, ", "+name+",") || strings.HasSuffix(ctx, ", "+name) || strings.Contains(ctx, ": "+name+",") {
			t.Errorf("workerToolsContext should not contain internal tool %q", name)
		}
	}

	// Some expected tools should be present
	for _, name := range []string{"Bash", "Edit", "Read", "Grep", "Glob"} {
		if !strings.Contains(ctx, name) {
			t.Errorf("workerToolsContext missing expected tool %q", name)
		}
	}
}

func TestGetCoordinatorUserContext_FullMode_ToolsSorted(t *testing.T) {
	resetEnv(t)
	FeatureGateChecker = func(gate string) bool { return gate == coordinatorModeGate }
	os.Setenv(coordinatorModeEnv, "1")
	defer os.Unsetenv(coordinatorModeEnv)
	os.Unsetenv("CLAUDE_CODE_SIMPLE")

	got := GetCoordinatorUserContext(nil, "")
	ctx := got["workerToolsContext"]

	// Extract tool list from "these tools: X, Y, Z"
	idx := strings.Index(ctx, "these tools: ")
	if idx < 0 {
		t.Fatal("could not find tool list in context")
	}
	toolStr := ctx[idx+len("these tools: "):]
	// Take only the first line
	if nl := strings.Index(toolStr, "\n"); nl >= 0 {
		toolStr = toolStr[:nl]
	}
	tools := strings.Split(toolStr, ", ")

	// Verify sorted
	for i := 1; i < len(tools); i++ {
		if tools[i] < tools[i-1] {
			t.Errorf("tools not sorted: %q comes after %q", tools[i], tools[i-1])
		}
	}
}

func TestGetCoordinatorUserContext_SimpleMode(t *testing.T) {
	resetEnv(t)
	FeatureGateChecker = func(gate string) bool { return gate == coordinatorModeGate }
	os.Setenv(coordinatorModeEnv, "1")
	defer os.Unsetenv(coordinatorModeEnv)
	os.Setenv("CLAUDE_CODE_SIMPLE", "1")
	defer os.Unsetenv("CLAUDE_CODE_SIMPLE")

	got := GetCoordinatorUserContext(nil, "")
	ctx := got["workerToolsContext"]

	// Simple mode should list only Bash, Edit, Read (sorted)
	if !strings.Contains(ctx, "Bash, Edit, Read") {
		t.Errorf("simple mode should list Bash, Edit, Read; got: %s", ctx)
	}
	// Should NOT contain full-mode tools
	if strings.Contains(ctx, "Grep") || strings.Contains(ctx, "Glob") {
		t.Error("simple mode should not contain Grep or Glob")
	}
}

func TestGetCoordinatorUserContext_WithMCPClients(t *testing.T) {
	resetEnv(t)
	FeatureGateChecker = func(gate string) bool { return gate == coordinatorModeGate }
	os.Setenv(coordinatorModeEnv, "1")
	defer os.Unsetenv(coordinatorModeEnv)

	clients := []MCPClient{
		{Name: "github"},
		{Name: "slack"},
	}
	got := GetCoordinatorUserContext(clients, "")
	ctx := got["workerToolsContext"]

	expected := "Workers also have access to MCP tools from connected MCP servers: github, slack"
	if !strings.Contains(ctx, expected) {
		t.Errorf("expected MCP section, got: %s", ctx)
	}
}

func TestGetCoordinatorUserContext_NoMCPClients(t *testing.T) {
	resetEnv(t)
	FeatureGateChecker = func(gate string) bool { return gate == coordinatorModeGate }
	os.Setenv(coordinatorModeEnv, "1")
	defer os.Unsetenv(coordinatorModeEnv)

	got := GetCoordinatorUserContext(nil, "")
	ctx := got["workerToolsContext"]

	if strings.Contains(ctx, "MCP tools") {
		t.Error("should not contain MCP section when no clients")
	}
}

func TestGetCoordinatorUserContext_WithScratchpad(t *testing.T) {
	resetEnv(t)
	FeatureGateChecker = func(gate string) bool { return gate == coordinatorModeGate }
	os.Setenv(coordinatorModeEnv, "1")
	defer os.Unsetenv(coordinatorModeEnv)
	ScratchpadGateChecker = func() bool { return true }
	defer func() { ScratchpadGateChecker = nil }()

	got := GetCoordinatorUserContext(nil, "/tmp/scratch")
	ctx := got["workerToolsContext"]

	if !strings.Contains(ctx, "Scratchpad directory: /tmp/scratch") {
		t.Error("expected scratchpad directory in context")
	}
	if !strings.Contains(ctx, "Workers can read and write here without permission prompts") {
		t.Error("expected scratchpad description")
	}
}

func TestGetCoordinatorUserContext_ScratchpadGateDisabled(t *testing.T) {
	resetEnv(t)
	FeatureGateChecker = func(gate string) bool { return gate == coordinatorModeGate }
	os.Setenv(coordinatorModeEnv, "1")
	defer os.Unsetenv(coordinatorModeEnv)
	// ScratchpadGateChecker is nil (default disabled)

	got := GetCoordinatorUserContext(nil, "/tmp/scratch")
	ctx := got["workerToolsContext"]

	if strings.Contains(ctx, "Scratchpad") {
		t.Error("scratchpad should not appear when gate is disabled")
	}
}

func TestGetCoordinatorUserContext_ScratchpadEmptyDir(t *testing.T) {
	resetEnv(t)
	FeatureGateChecker = func(gate string) bool { return gate == coordinatorModeGate }
	os.Setenv(coordinatorModeEnv, "1")
	defer os.Unsetenv(coordinatorModeEnv)
	ScratchpadGateChecker = func() bool { return true }
	defer func() { ScratchpadGateChecker = nil }()

	got := GetCoordinatorUserContext(nil, "")
	ctx := got["workerToolsContext"]

	if strings.Contains(ctx, "Scratchpad") {
		t.Error("scratchpad should not appear when dir is empty")
	}
}
