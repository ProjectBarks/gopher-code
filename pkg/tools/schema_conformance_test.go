package tools_test

import (
	"testing"

	"github.com/projectbarks/gopher-code/internal/testharness"
)

func TestSchemaConformance(t *testing.T) {
	schemas, err := testharness.LoadToolSchemas()
	if err != nil {
		t.Fatalf("failed to load golden tool schemas: %v", err)
	}

	t.Run("golden_file_has_expected_tools", func(t *testing.T) {
		if len(schemas) < 27 {
			t.Errorf("expected at least 27 tools in golden file, got %d", len(schemas))
		}
	})

	// Verify each core tool exists in golden file with expected properties
	coreTools := map[string]struct {
		readOnly        bool
		concurrencySafe bool
		requiredFields  []string
	}{
		"Bash":  {false, false, []string{"command"}},
		"Read":  {true, true, []string{"file_path"}},
		"Write": {false, false, []string{"file_path", "content"}},
		"Edit":  {false, false, []string{"file_path", "old_string", "new_string"}},
		"Glob":  {true, true, []string{"pattern"}},
		"Grep":  {true, true, []string{"pattern"}},
	}

	for toolName, expected := range coreTools {
		toolName, expected := toolName, expected // capture
		t.Run("golden_"+toolName+"_schema", func(t *testing.T) {
			var found *testharness.ToolSchemaEntry
			for i := range schemas {
				if schemas[i].Name == toolName {
					found = &schemas[i]
					break
				}
			}
			if found == nil {
				t.Fatalf("tool %q not found in golden file", toolName)
			}

			if found.IsReadOnly != expected.readOnly {
				t.Errorf("%s: is_read_only = %v, want %v", toolName, found.IsReadOnly, expected.readOnly)
			}
			if found.IsConcurrencySafe != expected.concurrencySafe {
				t.Errorf("%s: is_concurrency_safe = %v, want %v", toolName, found.IsConcurrencySafe, expected.concurrencySafe)
			}

			// Validate required fields exist in schema
			schema := found.InputSchema
			required, ok := schema["required"].([]interface{})
			if !ok {
				t.Fatalf("%s: input_schema missing 'required' field", toolName)
			}
			requiredSet := make(map[string]bool)
			for _, r := range required {
				if s, ok := r.(string); ok {
					requiredSet[s] = true
				}
			}
			for _, field := range expected.requiredFields {
				if !requiredSet[field] {
					t.Errorf("%s: expected required field %q not found in schema", toolName, field)
				}
			}

			// Validate additionalProperties is false
			if ap, ok := schema["additionalProperties"]; ok {
				if ap != false {
					t.Errorf("%s: additionalProperties should be false, got %v", toolName, ap)
				}
			}
		})
	}
}

func TestQueryLoopConstants(t *testing.T) {
	constants, err := testharness.LoadQueryLoopConstants()
	if err != nil {
		t.Fatalf("failed to load golden query loop constants: %v", err)
	}

	t.Run("max_output_tokens_recovery_limit", func(t *testing.T) {
		if constants.MaxOutputTokensRecoveryLimit != 3 {
			t.Errorf("expected 3, got %d", constants.MaxOutputTokensRecoveryLimit)
		}
	})

	t.Run("max_consecutive_autocompact_failures", func(t *testing.T) {
		if constants.MaxConsecutiveAutocompactFailures != 3 {
			t.Errorf("expected 3, got %d", constants.MaxConsecutiveAutocompactFailures)
		}
	})

	t.Run("autocompact_buffer_tokens", func(t *testing.T) {
		if constants.AutocompactBufferTokens != 13000 {
			t.Errorf("expected 13000, got %d", constants.AutocompactBufferTokens)
		}
	})

	t.Run("escalated_max_tokens", func(t *testing.T) {
		if constants.EscalatedMaxTokens != 64000 {
			t.Errorf("expected 64000, got %d", constants.EscalatedMaxTokens)
		}
	})

	t.Run("max_tool_use_concurrency", func(t *testing.T) {
		if constants.MaxToolUseConcurrency != 10 {
			t.Errorf("expected 10, got %d", constants.MaxToolUseConcurrency)
		}
	})

	t.Run("synthetic_tool_result_placeholder", func(t *testing.T) {
		expected := "[Tool result missing due to internal error]"
		if constants.SyntheticToolResultPlaceholder != expected {
			t.Errorf("expected %q, got %q", expected, constants.SyntheticToolResultPlaceholder)
		}
	})
}
