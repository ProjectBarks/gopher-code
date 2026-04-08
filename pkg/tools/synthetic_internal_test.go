package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestSyntheticOutputTool_BasicExecution(t *testing.T) {
	tool := &SyntheticOutputTool{}

	input, _ := json.Marshal(map[string]any{"name": "test", "value": 42})
	out, err := tool.Execute(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !strings.Contains(out.Content, "Structured output provided successfully") {
		t.Errorf("should contain success message, got: %s", out.Content)
	}
	if !strings.Contains(out.Content, "test") {
		t.Errorf("should contain structured output, got: %s", out.Content)
	}
}

func TestSyntheticOutputTool_WithSchema(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","required":["name"],"properties":{"name":{"type":"string"}}}`)
	tool := &SyntheticOutputTool{JSONSchema: schema}

	input, _ := json.Marshal(map[string]any{"name": "test"})
	out, _ := tool.Execute(context.Background(), nil, input)
	if out.IsError {
		t.Errorf("valid input should succeed, got error: %s", out.Content)
	}

	badInput, _ := json.Marshal(map[string]any{"value": 42})
	out, _ = tool.Execute(context.Background(), nil, badInput)
	if !strings.Contains(out.Content, "missing required field: name") {
		t.Errorf("should report missing field, got: %s", out.Content)
	}
}

func TestSyntheticOutputTool_DynamicSchema(t *testing.T) {
	tool := &SyntheticOutputTool{}
	schema := tool.InputSchema()
	if !strings.Contains(string(schema), "additionalProperties") {
		t.Error("default schema should allow additional properties")
	}

	custom := json.RawMessage(`{"type":"object","properties":{"bugs":{"type":"array"}}}`)
	tool2 := &SyntheticOutputTool{JSONSchema: custom}
	if !strings.Contains(string(tool2.InputSchema()), "bugs") {
		t.Error("custom schema should be returned")
	}
}

func TestIsSyntheticOutputToolEnabled(t *testing.T) {
	if IsSyntheticOutputToolEnabled(false) {
		t.Error("should be disabled in interactive mode")
	}
	if !IsSyntheticOutputToolEnabled(true) {
		t.Error("should be enabled in non-interactive mode")
	}
}
