package remote

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/message"
)

// Source: src/remote/remotePermissionBridge.ts

func TestCreateSyntheticAssistantMessage(t *testing.T) {
	// Source: remotePermissionBridge.ts:15-42

	t.Run("wraps_tool_use_in_assistant_shape", func(t *testing.T) {
		req := SDKControlPermissionRequest{
			ToolName:  "Write",
			ToolUseID: "tu-123",
			Input:     map[string]any{"path": "/tmp/foo.txt", "content": "hello"},
		}
		msg := CreateSyntheticAssistantMessage(req, "req-abc")

		// ID format: remote-{requestId}
		if msg.ID != "remote-req-abc" {
			t.Errorf("ID = %q, want %q", msg.ID, "remote-req-abc")
		}
		// Model empty
		if msg.Model != "" {
			t.Errorf("Model = %q, want empty", msg.Model)
		}
		// StopReason nil
		if msg.StopReason != nil {
			t.Errorf("StopReason = %v, want nil", msg.StopReason)
		}
		// Usage all-zero
		if msg.Usage.InputTokens != 0 || msg.Usage.OutputTokens != 0 {
			t.Errorf("Usage = %+v, want all zeros", msg.Usage)
		}
		// Timestamp non-empty
		if msg.Timestamp == "" {
			t.Error("Timestamp should not be empty")
		}
		// Content has exactly one tool_use block
		if len(msg.Content) != 1 {
			t.Fatalf("Content len = %d, want 1", len(msg.Content))
		}
		block := msg.Content[0]
		if block.Type != message.ContentToolUse {
			t.Errorf("block.Type = %q, want %q", block.Type, message.ContentToolUse)
		}
		if block.Name != "Write" {
			t.Errorf("block.Name = %q, want %q", block.Name, "Write")
		}
		if block.ID != "tu-123" {
			t.Errorf("block.ID = %q, want %q", block.ID, "tu-123")
		}
		// Input should be valid JSON containing the original input
		var parsed map[string]any
		if err := json.Unmarshal(block.Input, &parsed); err != nil {
			t.Fatalf("Input unmarshal: %v", err)
		}
		if parsed["path"] != "/tmp/foo.txt" {
			t.Errorf("Input[path] = %v", parsed["path"])
		}
	})

	t.Run("generates_tool_use_id_when_empty", func(t *testing.T) {
		req := SDKControlPermissionRequest{
			ToolName: "Read",
			Input:    map[string]any{"path": "/etc/hosts"},
		}
		msg := CreateSyntheticAssistantMessage(req, "req-xyz")
		if len(msg.Content) != 1 {
			t.Fatalf("Content len = %d", len(msg.Content))
		}
		if msg.Content[0].ID == "" {
			t.Error("should generate a tool_use ID when empty")
		}
	})
}

func TestCreateToolStub(t *testing.T) {
	// Source: remotePermissionBridge.ts:44-78

	t.Run("conservative_defaults", func(t *testing.T) {
		stub := CreateToolStub("CustomMCPTool")

		if stub.UserFacingName() != "CustomMCPTool" {
			t.Errorf("UserFacingName = %q", stub.UserFacingName())
		}
		if !stub.IsEnabled() {
			t.Error("IsEnabled should be true")
		}
		if !stub.NeedsPermissions() {
			t.Error("NeedsPermissions should be true (conservative)")
		}
		if stub.IsReadOnly() {
			t.Error("IsReadOnly should be false (conservative)")
		}
		if stub.IsMCP() {
			t.Error("IsMCP should be false for stubs")
		}
	})
}

func TestFormatToolInput(t *testing.T) {
	// Source: remotePermissionBridge.ts:62-73

	t.Run("formats_first_3_entries", func(t *testing.T) {
		// Keys sorted: alpha, beta, delta, zeta — first 3 = alpha, beta, delta
		input := map[string]any{
			"alpha": "one",
			"beta":  "two",
			"delta": "three",
			"zeta":  "four", // should be truncated (4th alphabetically)
		}
		result := FormatToolInput(input, 3)
		if !strings.Contains(result, "alpha: one") {
			t.Errorf("missing alpha: %s", result)
		}
		if !strings.Contains(result, "beta: two") {
			t.Errorf("missing beta: %s", result)
		}
		if !strings.Contains(result, "delta: three") {
			t.Errorf("missing delta: %s", result)
		}
		if strings.Contains(result, "zeta") {
			t.Errorf("should not contain zeta: %s", result)
		}
	})

	t.Run("empty_input", func(t *testing.T) {
		result := FormatToolInput(nil, 3)
		if result != "" {
			t.Errorf("expected empty, got %q", result)
		}
	})

	t.Run("json_serializes_non_strings", func(t *testing.T) {
		input := map[string]any{
			"count": 42,
			"flag":  true,
		}
		result := FormatToolInput(input, 3)
		if !strings.Contains(result, "count: 42") {
			t.Errorf("expected numeric value: %s", result)
		}
		if !strings.Contains(result, "flag: true") {
			t.Errorf("expected boolean value: %s", result)
		}
	})
}

func TestPermissionBridge_WrapRequest(t *testing.T) {
	// Integration: PermissionBridge.WrapRequest ties T74 + T75 together

	pb := NewPermissionBridge()
	req := SDKControlPermissionRequest{
		ToolName:  "Bash",
		ToolUseID: "tu-999",
		Input:     map[string]any{"command": "ls -la"},
	}
	msg, stub := pb.WrapRequest(req, "req-wrap-test")

	if msg.ID != "remote-req-wrap-test" {
		t.Errorf("msg.ID = %q", msg.ID)
	}
	if stub.UserFacingName() != "Bash" {
		t.Errorf("stub name = %q", stub.UserFacingName())
	}
	if !stub.NeedsPermissions() {
		t.Error("stub should need permissions")
	}
}

func TestToolStub_RenderInput(t *testing.T) {
	stub := CreateToolStub("Write")
	result := stub.RenderInput(map[string]any{
		"path":    "/tmp/test.txt",
		"content": "hello world",
	})
	if !strings.Contains(result, "content: hello world") {
		t.Errorf("unexpected render: %s", result)
	}
	if !strings.Contains(result, "path: /tmp/test.txt") {
		t.Errorf("unexpected render: %s", result)
	}
}
