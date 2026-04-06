package tools_test

import (
	"fmt"
	"testing"

	"github.com/projectbarks/gopher-code/internal/testharness"
)

// helpers
func getProps(schema map[string]interface{}) map[string]interface{} {
	p, _ := schema["properties"].(map[string]interface{})
	if p == nil {
		return map[string]interface{}{}
	}
	return p
}

func getRequired(schema map[string]interface{}) []string {
	raw, _ := schema["required"].([]interface{})
	out := make([]string, 0, len(raw))
	for _, r := range raw {
		if s, ok := r.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func isInRequired(schema map[string]interface{}, field string) bool {
	for _, r := range getRequired(schema) {
		if r == field {
			return true
		}
	}
	return false
}

func getPropField(prop map[string]interface{}, key string) (interface{}, bool) {
	v, ok := prop[key]
	return v, ok
}

// TestToolSchemaDetails validates every aspect of every tool schema in the golden file.
// Each property of each tool gets its own subtest for maximum granularity.
func TestToolSchemaDetails(t *testing.T) {
	schemas, err := testharness.LoadToolSchemas()
	if err != nil {
		t.Fatalf("failed to load schemas: %v", err)
	}

	for _, schema := range schemas {
		schema := schema
		t.Run(schema.Name, func(t *testing.T) {
			t.Run("name_non_empty", func(t *testing.T) {
				if schema.Name == "" {
					t.Error("tool name is empty")
				}
			})
			t.Run("description_non_empty", func(t *testing.T) {
				if schema.Description == "" {
					t.Error("tool description is empty")
				}
			})
			t.Run("schema_type_is_object", func(t *testing.T) {
				if schema.InputSchema["type"] != "object" {
					t.Errorf("expected type=object, got %v", schema.InputSchema["type"])
				}
			})
			t.Run("additional_properties_false", func(t *testing.T) {
				ap, ok := schema.InputSchema["additionalProperties"]
				if !ok {
					t.Error("missing additionalProperties")
				} else if ap != false {
					t.Errorf("expected additionalProperties=false, got %v", ap)
				}
			})
			t.Run("has_properties_object", func(t *testing.T) {
				_, ok := schema.InputSchema["properties"].(map[string]interface{})
				if !ok {
					t.Error("missing or invalid properties object")
				}
			})
			t.Run("has_required_array", func(t *testing.T) {
				_, ok := schema.InputSchema["required"].([]interface{})
				if !ok {
					t.Error("missing or invalid required array")
				}
			})
			t.Run("max_result_size_set", func(t *testing.T) {
				if schema.MaxResultSizeChars == 0 {
					t.Error("max_result_size_chars is 0 (should be set or -1 for infinity)")
				}
			})

			// Validate each required field exists in properties
			props := getProps(schema.InputSchema)
			for _, req := range getRequired(schema.InputSchema) {
				req := req
				t.Run(fmt.Sprintf("required_%s_in_properties", req), func(t *testing.T) {
					if _, ok := props[req]; !ok {
						t.Errorf("required field %q not found in properties", req)
					}
				})
			}

			// Validate each property
			for propName, propVal := range props {
				propName := propName
				prop, ok := propVal.(map[string]interface{})
				if !ok {
					continue
				}
				t.Run(fmt.Sprintf("prop_%s_has_type", propName), func(t *testing.T) {
					if _, ok := getPropField(prop, "type"); !ok {
						// arrays/objects with items/properties are also valid
						if _, hasItems := prop["items"]; !hasItems {
							if _, hasProps := prop["properties"]; !hasProps {
								t.Errorf("property %q has no type field", propName)
							}
						}
					}
				})
				t.Run(fmt.Sprintf("prop_%s_valid_type", propName), func(t *testing.T) {
					typ, ok := prop["type"].(string)
					if !ok {
						return // complex type, skip
					}
					validTypes := map[string]bool{
						"string": true, "number": true, "integer": true,
						"boolean": true, "array": true, "object": true,
					}
					if !validTypes[typ] {
						t.Errorf("property %q has invalid type %q", propName, typ)
					}
				})
				// Check enum values if present
				if enumVal, ok := prop["enum"]; ok {
					t.Run(fmt.Sprintf("prop_%s_enum_non_empty", propName), func(t *testing.T) {
						arr, ok := enumVal.([]interface{})
						if !ok || len(arr) == 0 {
							t.Errorf("property %q has empty or invalid enum", propName)
						}
					})
				}
				// Check array items
				if prop["type"] == "array" {
					t.Run(fmt.Sprintf("prop_%s_array_has_items", propName), func(t *testing.T) {
						if _, ok := prop["items"]; !ok {
							t.Errorf("array property %q missing items", propName)
						}
					})
				}
			}
		})
	}
}

// TestCrossToolValidation checks invariants across all tools.
func TestCrossToolValidation(t *testing.T) {
	schemas, err := testharness.LoadToolSchemas()
	if err != nil {
		t.Fatalf("failed to load schemas: %v", err)
	}

	t.Run("tool_count_is_27", func(t *testing.T) {
		if len(schemas) != 27 {
			t.Errorf("expected 27 tools, got %d", len(schemas))
		}
	})

	t.Run("all_names_unique", func(t *testing.T) {
		seen := map[string]bool{}
		for _, s := range schemas {
			if seen[s.Name] {
				t.Errorf("duplicate tool name: %s", s.Name)
			}
			seen[s.Name] = true
		}
	})

	t.Run("read_only_implies_concurrency_safe", func(t *testing.T) {
		for _, s := range schemas {
			if s.IsReadOnly && !s.IsConcurrencySafe {
				t.Errorf("tool %s is read-only but not concurrency-safe", s.Name)
			}
		}
	})

	coreTools := []string{"Bash", "Read", "Write", "Edit", "Glob", "Grep"}
	for _, name := range coreTools {
		name := name
		t.Run(fmt.Sprintf("core_tool_%s_present", name), func(t *testing.T) {
			found := false
			for _, s := range schemas {
				if s.Name == name {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("core tool %q not found", name)
			}
		})
	}

	extTools := []string{"WebFetch", "WebSearch", "Agent", "Skill", "ToolSearch", "LSP", "NotebookEdit", "TodoWrite", "TaskCreate", "TaskUpdate", "TaskGet", "TaskList", "TaskStop", "TaskOutput", "AskUserQuestion", "PowerShell", "TeamCreate", "TeamDelete", "Config", "RemoteTrigger", "SyntheticOutput"}
	for _, name := range extTools {
		name := name
		t.Run(fmt.Sprintf("extended_tool_%s_present", name), func(t *testing.T) {
			found := false
			for _, s := range schemas {
				if s.Name == name {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("extended tool %q not found", name)
			}
		})
	}

	t.Run("all_schemas_are_objects", func(t *testing.T) {
		for _, s := range schemas {
			if s.InputSchema["type"] != "object" {
				t.Errorf("tool %s schema type is %v, want object", s.Name, s.InputSchema["type"])
			}
		}
	})

	t.Run("no_tool_allows_additional_properties", func(t *testing.T) {
		for _, s := range schemas {
			if ap, ok := s.InputSchema["additionalProperties"]; ok && ap != false {
				t.Errorf("tool %s allows additional properties", s.Name)
			}
		}
	})
}

// TestPerToolExpectedValues validates exact expected values for each tool.
func TestPerToolExpectedValues(t *testing.T) {
	schemas, err := testharness.LoadToolSchemas()
	if err != nil {
		t.Fatalf("failed to load schemas: %v", err)
	}
	byName := map[string]testharness.ToolSchemaEntry{}
	for _, s := range schemas {
		byName[s.Name] = s
	}

	type toolExpectation struct {
		name           string
		readOnly       bool
		concSafe       bool
		maxResult      int
		propCount      int
		requiredCount  int
		requiredFields []string
	}

	expectations := []toolExpectation{
		{"Bash", false, false, 30000, 5, 1, []string{"command"}},
		{"Read", true, true, -1, 4, 1, []string{"file_path"}},
		{"Write", false, false, 100000, 2, 2, []string{"file_path", "content"}},
		{"Edit", false, false, 100000, 4, 3, []string{"file_path", "old_string", "new_string"}},
		{"Glob", true, true, 100000, 2, 1, []string{"pattern"}},
		{"Grep", true, true, 20000, 14, 1, []string{"pattern"}},
		{"WebFetch", true, true, 100000, 2, 2, []string{"url", "prompt"}},
		{"WebSearch", true, true, 100000, 3, 1, []string{"query"}},
		{"Agent", true, true, 100000, 6, 2, []string{"description", "prompt"}},
		{"Skill", false, false, 100000, 2, 1, []string{"skill"}},
		{"ToolSearch", true, true, 100000, 2, 1, []string{"query"}},
		{"LSP", true, true, 100000, 4, 4, []string{"operation", "filePath", "line", "character"}},
		{"NotebookEdit", false, false, 100000, 5, 2, []string{"notebook_path", "new_source"}},
		{"TodoWrite", false, false, 100000, 1, 1, []string{"todos"}},
		{"TaskCreate", false, true, 100000, 4, 2, []string{"subject", "description"}},
		{"TaskUpdate", false, true, 100000, 9, 1, []string{"taskId"}},
		{"TaskGet", true, true, 100000, 1, 1, []string{"taskId"}},
		{"TaskList", true, true, 100000, 0, 0, []string{}},
		{"TaskStop", false, true, 100000, 2, 0, []string{}},
		{"TaskOutput", false, true, 100000, 4, 1, []string{"task_id"}},
		{"AskUserQuestion", true, true, 100000, 4, 1, []string{"questions"}},
		{"PowerShell", false, false, 30000, 2, 1, []string{"command"}},
		{"TeamCreate", false, true, 100000, 3, 1, []string{"team_name"}},
		{"TeamDelete", false, true, 100000, 1, 1, []string{"team_name"}},
		{"Config", true, true, 100000, 3, 1, []string{"action"}},
		{"RemoteTrigger", false, true, 100000, 2, 2, []string{"agent", "prompt"}},
		{"SyntheticOutput", true, true, 100000, 1, 1, []string{"text"}},
	}

	for _, exp := range expectations {
		exp := exp
		t.Run(exp.name, func(t *testing.T) {
			s, ok := byName[exp.name]
			if !ok {
				t.Fatalf("tool %q not in golden file", exp.name)
				return
			}

			t.Run("is_read_only", func(t *testing.T) {
				if s.IsReadOnly != exp.readOnly {
					t.Errorf("got %v, want %v", s.IsReadOnly, exp.readOnly)
				}
			})
			t.Run("is_concurrency_safe", func(t *testing.T) {
				if s.IsConcurrencySafe != exp.concSafe {
					t.Errorf("got %v, want %v", s.IsConcurrencySafe, exp.concSafe)
				}
			})
			t.Run("max_result_size", func(t *testing.T) {
				if s.MaxResultSizeChars != exp.maxResult {
					t.Errorf("got %d, want %d", s.MaxResultSizeChars, exp.maxResult)
				}
			})
			t.Run("property_count", func(t *testing.T) {
				props := getProps(s.InputSchema)
				if len(props) != exp.propCount {
					t.Errorf("got %d properties, want %d", len(props), exp.propCount)
				}
			})
			t.Run("required_count", func(t *testing.T) {
				req := getRequired(s.InputSchema)
				if len(req) != exp.requiredCount {
					t.Errorf("got %d required, want %d", len(req), exp.requiredCount)
				}
			})
			for _, field := range exp.requiredFields {
				field := field
				t.Run(fmt.Sprintf("required_%s", field), func(t *testing.T) {
					if !isInRequired(s.InputSchema, field) {
						t.Errorf("field %q should be required", field)
					}
				})
			}
			// Verify each required field is in properties
			for _, field := range exp.requiredFields {
				field := field
				t.Run(fmt.Sprintf("required_%s_exists_in_props", field), func(t *testing.T) {
					props := getProps(s.InputSchema)
					if _, ok := props[field]; !ok {
						t.Errorf("required field %q not in properties", field)
					}
				})
			}
		})
	}
}

// TestPropertyTypes validates the exact type of every property in every tool.
func TestPropertyTypes(t *testing.T) {
	schemas, err := testharness.LoadToolSchemas()
	if err != nil {
		t.Fatalf("failed to load schemas: %v", err)
	}
	byName := map[string]testharness.ToolSchemaEntry{}
	for _, s := range schemas {
		byName[s.Name] = s
	}

	type propCheck struct {
		tool     string
		prop     string
		typ      string
		hasEnum  bool
		enumVals []string
	}

	checks := []propCheck{
		// Bash
		{"Bash", "command", "string", false, nil},
		{"Bash", "timeout", "number", false, nil},
		{"Bash", "description", "string", false, nil},
		{"Bash", "run_in_background", "boolean", false, nil},
		{"Bash", "dangerouslyDisableSandbox", "boolean", false, nil},
		// Read
		{"Read", "file_path", "string", false, nil},
		{"Read", "offset", "integer", false, nil},
		{"Read", "limit", "integer", false, nil},
		{"Read", "pages", "string", false, nil},
		// Write
		{"Write", "file_path", "string", false, nil},
		{"Write", "content", "string", false, nil},
		// Edit
		{"Edit", "file_path", "string", false, nil},
		{"Edit", "old_string", "string", false, nil},
		{"Edit", "new_string", "string", false, nil},
		{"Edit", "replace_all", "boolean", false, nil},
		// Glob
		{"Glob", "pattern", "string", false, nil},
		{"Glob", "path", "string", false, nil},
		// Grep
		{"Grep", "pattern", "string", false, nil},
		{"Grep", "path", "string", false, nil},
		{"Grep", "glob", "string", false, nil},
		{"Grep", "output_mode", "string", true, []string{"content", "files_with_matches", "count"}},
		{"Grep", "-B", "number", false, nil},
		{"Grep", "-A", "number", false, nil},
		{"Grep", "-C", "number", false, nil},
		{"Grep", "context", "number", false, nil},
		{"Grep", "-n", "boolean", false, nil},
		{"Grep", "-i", "boolean", false, nil},
		{"Grep", "type", "string", false, nil},
		{"Grep", "head_limit", "number", false, nil},
		{"Grep", "offset", "number", false, nil},
		{"Grep", "multiline", "boolean", false, nil},
		// WebFetch
		{"WebFetch", "url", "string", false, nil},
		{"WebFetch", "prompt", "string", false, nil},
		// WebSearch
		{"WebSearch", "query", "string", false, nil},
		{"WebSearch", "allowed_domains", "array", false, nil},
		{"WebSearch", "blocked_domains", "array", false, nil},
		// Agent
		{"Agent", "description", "string", false, nil},
		{"Agent", "prompt", "string", false, nil},
		{"Agent", "subagent_type", "string", false, nil},
		{"Agent", "model", "string", true, []string{"sonnet", "opus", "haiku"}},
		{"Agent", "run_in_background", "boolean", false, nil},
		{"Agent", "isolation", "string", true, []string{"worktree"}},
		// Skill
		{"Skill", "skill", "string", false, nil},
		{"Skill", "args", "string", false, nil},
		// ToolSearch
		{"ToolSearch", "query", "string", false, nil},
		{"ToolSearch", "max_results", "number", false, nil},
		// LSP
		{"LSP", "operation", "string", true, []string{"goToDefinition", "findReferences", "hover", "documentSymbol", "workspaceSymbol", "goToImplementation", "prepareCallHierarchy", "incomingCalls", "outgoingCalls"}},
		{"LSP", "filePath", "string", false, nil},
		{"LSP", "line", "number", false, nil},
		{"LSP", "character", "number", false, nil},
		// NotebookEdit
		{"NotebookEdit", "notebook_path", "string", false, nil},
		{"NotebookEdit", "cell_id", "string", false, nil},
		{"NotebookEdit", "new_source", "string", false, nil},
		{"NotebookEdit", "cell_type", "string", true, []string{"code", "markdown"}},
		{"NotebookEdit", "edit_mode", "string", true, []string{"replace", "insert", "delete"}},
		// TaskCreate
		{"TaskCreate", "subject", "string", false, nil},
		{"TaskCreate", "description", "string", false, nil},
		{"TaskCreate", "activeForm", "string", false, nil},
		{"TaskCreate", "metadata", "object", false, nil},
		// TaskUpdate
		{"TaskUpdate", "taskId", "string", false, nil},
		{"TaskUpdate", "subject", "string", false, nil},
		{"TaskUpdate", "description", "string", false, nil},
		{"TaskUpdate", "activeForm", "string", false, nil},
		{"TaskUpdate", "status", "string", true, []string{"pending", "in_progress", "completed", "deleted"}},
		{"TaskUpdate", "addBlocks", "array", false, nil},
		{"TaskUpdate", "addBlockedBy", "array", false, nil},
		{"TaskUpdate", "owner", "string", false, nil},
		{"TaskUpdate", "metadata", "object", false, nil},
		// TaskGet
		{"TaskGet", "taskId", "string", false, nil},
		// TaskStop
		{"TaskStop", "task_id", "string", false, nil},
		{"TaskStop", "shell_id", "string", false, nil},
		// TaskOutput
		{"TaskOutput", "task_id", "string", false, nil},
		{"TaskOutput", "output", "string", false, nil},
		{"TaskOutput", "block", "boolean", false, nil},
		{"TaskOutput", "timeout", "number", false, nil},
		// AskUserQuestion
		{"AskUserQuestion", "questions", "array", false, nil},
		{"AskUserQuestion", "answers", "object", false, nil},
		{"AskUserQuestion", "annotations", "object", false, nil},
		{"AskUserQuestion", "metadata", "object", false, nil},
		// PowerShell
		{"PowerShell", "command", "string", false, nil},
		{"PowerShell", "timeout", "integer", false, nil},
		// TeamCreate
		{"TeamCreate", "team_name", "string", false, nil},
		{"TeamCreate", "description", "string", false, nil},
		{"TeamCreate", "agent_type", "string", false, nil},
		// TeamDelete
		{"TeamDelete", "team_name", "string", false, nil},
		// Config
		{"Config", "action", "string", true, []string{"get", "set", "list"}},
		{"Config", "key", "string", false, nil},
		{"Config", "value", "string", false, nil},
		// RemoteTrigger
		{"RemoteTrigger", "agent", "string", false, nil},
		{"RemoteTrigger", "prompt", "string", false, nil},
		// SyntheticOutput
		{"SyntheticOutput", "text", "string", false, nil},
	}

	for _, c := range checks {
		c := c
		t.Run(fmt.Sprintf("%s/%s/type_%s", c.tool, c.prop, c.typ), func(t *testing.T) {
			s, ok := byName[c.tool]
			if !ok {
				t.Fatalf("tool %q not found", c.tool)
			}
			props := getProps(s.InputSchema)
			prop, ok := props[c.prop].(map[string]interface{})
			if !ok {
				t.Fatalf("property %q not found in %s", c.prop, c.tool)
			}
			typ, _ := prop["type"].(string)
			if typ != c.typ {
				t.Errorf("expected type %q, got %q", c.typ, typ)
			}
		})

		if c.hasEnum {
			c := c
			t.Run(fmt.Sprintf("%s/%s/enum_values", c.tool, c.prop), func(t *testing.T) {
				s := byName[c.tool]
				props := getProps(s.InputSchema)
				prop := props[c.prop].(map[string]interface{})
				enumRaw, ok := prop["enum"].([]interface{})
				if !ok {
					t.Fatal("missing enum")
				}
				got := make([]string, len(enumRaw))
				for i, v := range enumRaw {
					got[i] = v.(string)
				}
				if len(got) != len(c.enumVals) {
					t.Fatalf("enum length %d, want %d", len(got), len(c.enumVals))
				}
				for i, v := range c.enumVals {
					if got[i] != v {
						t.Errorf("enum[%d] = %q, want %q", i, got[i], v)
					}
				}
			})
		}
	}
}

// TestQueryConstants validates every constant in the golden constants file.
func TestQueryConstants(t *testing.T) {
	c, err := testharness.LoadQueryLoopConstants()
	if err != nil {
		t.Fatalf("failed to load constants: %v", err)
	}

	tests := []struct {
		name     string
		got      int
		expected int
	}{
		{"max_output_tokens_recovery_limit", c.MaxOutputTokensRecoveryLimit, 3},
		{"max_consecutive_autocompact_failures", c.MaxConsecutiveAutocompactFailures, 3},
		{"autocompact_buffer_tokens", c.AutocompactBufferTokens, 13000},
		{"warning_threshold_buffer_tokens", c.WarningThresholdBufferTokens, 20000},
		{"error_threshold_buffer_tokens", c.ErrorThresholdBufferTokens, 20000},
		{"manual_compact_buffer_tokens", c.ManualCompactBufferTokens, 3000},
		{"max_output_tokens_for_summary", c.MaxOutputTokensForSummary, 20000},
		{"escalated_max_tokens", c.EscalatedMaxTokens, 64000},
		{"max_tool_use_concurrency", c.MaxToolUseConcurrency, 10},
		{"default_max_output_tokens", c.DefaultMaxOutputTokens, 16000},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("got %d, want %d", tt.got, tt.expected)
			}
		})
	}

	t.Run("synthetic_placeholder", func(t *testing.T) {
		expected := "[Tool result missing due to internal error]"
		if c.SyntheticToolResultPlaceholder != expected {
			t.Errorf("got %q, want %q", c.SyntheticToolResultPlaceholder, expected)
		}
	})
}
