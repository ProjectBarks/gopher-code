package hooks

import (
	"encoding/json"
	"strings"
)

// Source: utils/hooks/hookHelpers.ts

// HookResponse is the structured response schema for prompt/agent hooks.
// Source: hookHelpers.ts:16-24
type HookResponse struct {
	OK     bool   `json:"ok"`
	Reason string `json:"reason,omitempty"`
}

// SyntheticOutputToolName is the tool name used for structured output enforcement.
// Source: tools/SyntheticOutputTool/SyntheticOutputTool.ts
const SyntheticOutputToolName = "StructuredOutput"

// StructuredOutputToolPrompt is the prompt text for the structured output tool.
// Source: hookHelpers.ts:61
const StructuredOutputToolPrompt = "Use this tool to return your verification result. You MUST call this tool exactly once at the end of your response."

// StructuredOutputEnforcementTimeout is the timeout for the enforcement hook.
// Source: hookHelpers.ts:81
const StructuredOutputEnforcementTimeout = 5000

// AddArgumentsToPrompt replaces $ARGUMENTS in a prompt with the JSON input.
// Also supports indexed arguments like $ARGUMENTS[0], $ARGUMENTS[1], or shorthand $0, $1.
// Source: hookHelpers.ts:30-35
func AddArgumentsToPrompt(prompt string, jsonInput string) string {
	return SubstituteArguments(prompt, jsonInput)
}

// SubstituteArguments handles $ARGUMENTS, $ARGUMENTS[N], $N placeholder replacement.
// Source: utils/argumentSubstitution.ts
func SubstituteArguments(prompt string, jsonInput string) string {
	// Try to parse as JSON object for indexed access
	var parsed map[string]json.RawMessage
	_ = json.Unmarshal([]byte(jsonInput), &parsed)

	// Try to parse as JSON array for indexed access
	var parsedArray []json.RawMessage
	_ = json.Unmarshal([]byte(jsonInput), &parsedArray)

	// Replace $ARGUMENTS[N] first (before $ARGUMENTS), then $N shorthand
	result := prompt
	if parsedArray != nil {
		for i, val := range parsedArray {
			indexedPlaceholder := "$ARGUMENTS[" + itoa(i) + "]"
			valStr := strings.Trim(string(val), `"`)
			result = strings.ReplaceAll(result, indexedPlaceholder, valStr)
		}
		for i, val := range parsedArray {
			shortPlaceholder := "$" + itoa(i)
			valStr := strings.Trim(string(val), `"`)
			result = strings.ReplaceAll(result, shortPlaceholder, valStr)
		}
	}

	// Replace $ARGUMENTS with the full JSON input (after indexed replacements)
	result = strings.ReplaceAll(result, "$ARGUMENTS", jsonInput)

	return result
}

// itoa converts an int to string without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := make([]byte, 0, 4)
	for n > 0 {
		digits = append(digits, byte('0'+n%10))
		n /= 10
	}
	// reverse
	for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
		digits[i], digits[j] = digits[j], digits[i]
	}
	return string(digits)
}

// StructuredOutputEnforcementMessage returns the enforcement message for the structured output tool.
// Source: hookHelpers.ts:80
func StructuredOutputEnforcementMessage() string {
	return "You MUST call the " + SyntheticOutputToolName + " tool to complete this request. Call this tool now."
}

// HookResponseJSONSchema returns the JSON Schema for HookResponse.
// Source: hookHelpers.ts:42-58
func HookResponseJSONSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"ok": map[string]interface{}{
				"type":        "boolean",
				"description": "Whether the condition was met",
			},
			"reason": map[string]interface{}{
				"type":        "string",
				"description": "Reason, if the condition was not met",
			},
		},
		"required":             []string{"ok"},
		"additionalProperties": false,
	}
}
