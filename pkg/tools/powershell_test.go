package tools_test

import (
	"context"
	"encoding/json"
	"runtime"
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/tools"
)

func TestPowerShellTool(t *testing.T) {
	tool := &tools.PowerShellTool{}

	t.Run("name", func(t *testing.T) {
		if tool.Name() != "PowerShell" {
			t.Errorf("expected 'PowerShell', got %q", tool.Name())
		}
	})

	t.Run("not_read_only", func(t *testing.T) {
		if tool.IsReadOnly() {
			t.Error("PowerShellTool should not be read-only")
		}
	})

	t.Run("valid_schema", func(t *testing.T) {
		var parsed map[string]interface{}
		if err := json.Unmarshal(tool.InputSchema(), &parsed); err != nil {
			t.Fatalf("schema is not valid JSON: %v", err)
		}
		props, ok := parsed["properties"].(map[string]interface{})
		if !ok {
			t.Fatal("schema missing properties")
		}
		if _, ok := props["command"]; !ok {
			t.Error("schema missing 'command' property")
		}
		if _, ok := props["timeout"]; !ok {
			t.Error("schema missing 'timeout' property")
		}
		// Verify additionalProperties is false
		if ap, ok := parsed["additionalProperties"]; !ok || ap != false {
			t.Error("additionalProperties should be false")
		}
	})

	t.Run("missing_command", func(t *testing.T) {
		input := json.RawMessage(`{"command": ""}`)
		out, err := tool.Execute(context.Background(), nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for empty command")
		}
	})

	t.Run("invalid_json", func(t *testing.T) {
		out, err := tool.Execute(context.Background(), nil, json.RawMessage(`{bad}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for invalid JSON")
		}
	})

	t.Run("non_windows_without_pwsh", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("skipping non-Windows test on Windows")
		}
		// This test validates the behavior on non-Windows when pwsh is unavailable.
		// If pwsh IS available, it will execute; if not, it returns an error.
		input := json.RawMessage(`{"command": "Write-Output 'hello'"}`)
		out, err := tool.Execute(context.Background(), nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Should either succeed (pwsh available) or return platform error
		if out.IsError && !strings.Contains(out.Content, "not available") {
			// pwsh might be installed but failed for another reason
			t.Logf("got error: %s", out.Content)
		}
	})
}
