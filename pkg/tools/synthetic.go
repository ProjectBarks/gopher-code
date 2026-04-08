package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// Source: tools/SyntheticOutputTool/SyntheticOutputTool.ts
//
// StructuredOutput (called SyntheticOutput internally) is for non-interactive
// SDK/CLI mode. The model calls it with structured JSON matching a caller-provided
// schema. In interactive mode this tool is disabled.

// SyntheticOutputTool returns structured JSON output for SDK/headless mode.
// Name is "SyntheticOutput" internally but registered as "StructuredOutput" to match TS.
type SyntheticOutputTool struct {
	// JSONSchema is the optional schema for validating structured output.
	// Set by the SDK caller via --output-schema. Nil means accept any object.
	JSONSchema json.RawMessage
}

func (t *SyntheticOutputTool) Name() string        { return "SyntheticOutput" }
func (t *SyntheticOutputTool) Description() string { return "Return structured output in the requested format" }
func (t *SyntheticOutputTool) IsReadOnly() bool    { return true }
func (t *SyntheticOutputTool) SearchHint() string  { return "return the final response as structured JSON" }
func (t *SyntheticOutputTool) MaxResultSizeChars() int { return 100_000 }

// Prompt returns the tool's system prompt instruction.
// Source: SyntheticOutputTool.ts:51-52
func (t *SyntheticOutputTool) Prompt() string {
	return "Use this tool to return your final response in the requested structured format. You MUST call this tool exactly once at the end of your response to provide the structured output."
}

// InputSchema returns the dynamic schema (from caller) or a permissive default.
func (t *SyntheticOutputTool) InputSchema() json.RawMessage {
	if len(t.JSONSchema) > 0 {
		return t.JSONSchema
	}
	return json.RawMessage(`{"type":"object","additionalProperties":true}`)
}

// Execute validates input against the schema (if provided) and returns it.
// Source: SyntheticOutputTool.ts:59-65
func (t *SyntheticOutputTool) Execute(_ context.Context, _ *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	// Parse input as a generic object
	var obj map[string]any
	if err := json.Unmarshal(input, &obj); err != nil {
		return ErrorOutput(fmt.Sprintf("invalid JSON input: %s", err)), nil
	}

	// If a schema is provided, do basic type validation
	if len(t.JSONSchema) > 0 {
		if err := validateAgainstSchema(input, t.JSONSchema); err != nil {
			return ErrorOutput(fmt.Sprintf("Output does not match required schema: %s", err)), nil
		}
	}

	// Return success with the structured output embedded
	result := map[string]any{
		"data":              "Structured output provided successfully",
		"structured_output": obj,
	}
	out, _ := json.Marshal(result)
	return SuccessOutput(string(out)), nil
}

// IsSyntheticOutputToolEnabled returns true in non-interactive (SDK/headless) mode.
// Source: SyntheticOutputTool.ts:22-26
func IsSyntheticOutputToolEnabled(isNonInteractive bool) bool {
	return isNonInteractive
}

// validateAgainstSchema does basic structural validation of JSON against a schema.
// For full JSON Schema validation, use jsonschema/v6 — this is a lightweight check.
func validateAgainstSchema(data, schema json.RawMessage) error {
	var s struct {
		Type       string              `json:"type"`
		Required   []string            `json:"required"`
		Properties map[string]struct{} `json:"properties"`
	}
	if err := json.Unmarshal(schema, &s); err != nil {
		return nil // Can't parse schema — skip validation
	}

	if s.Type == "object" && len(s.Required) > 0 {
		var obj map[string]any
		if err := json.Unmarshal(data, &obj); err != nil {
			return fmt.Errorf("expected object, got invalid JSON")
		}
		for _, key := range s.Required {
			if _, ok := obj[key]; !ok {
				return fmt.Errorf("missing required field: %s", key)
			}
		}
	}
	return nil
}
