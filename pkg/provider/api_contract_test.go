package provider_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func testdataDir() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..", "testdata")
}

func loadJSON(t *testing.T, name string) map[string]interface{} {
	t.Helper()
	path := filepath.Join(testdataDir(), "api_contracts", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", name, err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to parse %s: %v", name, err)
	}
	return result
}

func loadJSONL(t *testing.T, name string) []map[string]interface{} {
	t.Helper()
	path := filepath.Join(testdataDir(), "api_contracts", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", name, err)
	}
	var results []map[string]interface{}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			t.Fatalf("failed to parse JSONL line: %v", err)
		}
		results = append(results, obj)
	}
	return results
}

// TestRequestFormats validates the structure of API request golden files.
func TestRequestFormats(t *testing.T) {
	requests := []struct {
		file          string
		hasSystem     bool
		hasTools      bool
		messageCount  int
		firstRole     string
	}{
		{"request_text_only.json", true, false, 1, "user"},
		{"request_with_tools.json", true, true, 1, "user"},
		{"request_with_tool_result.json", false, false, 3, "user"},
		{"request_tool_result_error.json", false, false, 3, "user"},
	}

	for _, tc := range requests {
		tc := tc
		t.Run(tc.file, func(t *testing.T) {
			req := loadJSON(t, tc.file)

			t.Run("has_model", func(t *testing.T) {
				model, ok := req["model"].(string)
				if !ok || model == "" {
					t.Error("missing or empty model field")
				}
			})
			t.Run("has_max_tokens", func(t *testing.T) {
				mt, ok := req["max_tokens"].(float64)
				if !ok || mt <= 0 {
					t.Error("missing or invalid max_tokens")
				}
			})
			t.Run("has_messages", func(t *testing.T) {
				msgs, ok := req["messages"].([]interface{})
				if !ok {
					t.Fatal("missing messages array")
				}
				if len(msgs) != tc.messageCount {
					t.Errorf("expected %d messages, got %d", tc.messageCount, len(msgs))
				}
			})
			t.Run("first_message_role", func(t *testing.T) {
				msgs := req["messages"].([]interface{})
				first := msgs[0].(map[string]interface{})
				if first["role"] != tc.firstRole {
					t.Errorf("expected first role %q, got %v", tc.firstRole, first["role"])
				}
			})
			t.Run("system_field", func(t *testing.T) {
				_, hasSystem := req["system"]
				if hasSystem != tc.hasSystem {
					t.Errorf("system present=%v, want %v", hasSystem, tc.hasSystem)
				}
			})
			t.Run("tools_field", func(t *testing.T) {
				_, hasTools := req["tools"]
				if hasTools != tc.hasTools {
					t.Errorf("tools present=%v, want %v", hasTools, tc.hasTools)
				}
			})
		})
	}
}

// TestRequestMessageStructure validates content block structure in requests.
func TestRequestMessageStructure(t *testing.T) {
	t.Run("text_only_message", func(t *testing.T) {
		req := loadJSON(t, "request_text_only.json")
		msgs := req["messages"].([]interface{})
		msg := msgs[0].(map[string]interface{})
		content := msg["content"].([]interface{})

		t.Run("has_one_block", func(t *testing.T) {
			if len(content) != 1 {
				t.Errorf("expected 1 content block, got %d", len(content))
			}
		})
		t.Run("block_type_text", func(t *testing.T) {
			block := content[0].(map[string]interface{})
			if block["type"] != "text" {
				t.Errorf("expected type=text, got %v", block["type"])
			}
		})
		t.Run("block_has_text", func(t *testing.T) {
			block := content[0].(map[string]interface{})
			if _, ok := block["text"].(string); !ok {
				t.Error("text block missing text field")
			}
		})
	})

	t.Run("tool_use_message", func(t *testing.T) {
		req := loadJSON(t, "request_with_tool_result.json")
		msgs := req["messages"].([]interface{})
		assistant := msgs[1].(map[string]interface{})
		content := assistant["content"].([]interface{})

		t.Run("role_is_assistant", func(t *testing.T) {
			if assistant["role"] != "assistant" {
				t.Errorf("expected assistant, got %v", assistant["role"])
			}
		})
		t.Run("has_two_blocks", func(t *testing.T) {
			if len(content) != 2 {
				t.Errorf("expected 2 blocks, got %d", len(content))
			}
		})
		t.Run("first_block_text", func(t *testing.T) {
			if content[0].(map[string]interface{})["type"] != "text" {
				t.Error("first block should be text")
			}
		})
		t.Run("second_block_tool_use", func(t *testing.T) {
			block := content[1].(map[string]interface{})
			if block["type"] != "tool_use" {
				t.Error("second block should be tool_use")
			}
		})
		t.Run("tool_use_has_id", func(t *testing.T) {
			block := content[1].(map[string]interface{})
			if _, ok := block["id"].(string); !ok {
				t.Error("tool_use missing id")
			}
		})
		t.Run("tool_use_has_name", func(t *testing.T) {
			block := content[1].(map[string]interface{})
			if _, ok := block["name"].(string); !ok {
				t.Error("tool_use missing name")
			}
		})
		t.Run("tool_use_has_input", func(t *testing.T) {
			block := content[1].(map[string]interface{})
			if _, ok := block["input"].(map[string]interface{}); !ok {
				t.Error("tool_use missing input object")
			}
		})
	})

	t.Run("tool_result_message", func(t *testing.T) {
		req := loadJSON(t, "request_with_tool_result.json")
		msgs := req["messages"].([]interface{})
		user := msgs[2].(map[string]interface{})
		content := user["content"].([]interface{})

		t.Run("role_is_user", func(t *testing.T) {
			if user["role"] != "user" {
				t.Errorf("tool_result should be in user message, got %v", user["role"])
			}
		})
		t.Run("block_type_tool_result", func(t *testing.T) {
			block := content[0].(map[string]interface{})
			if block["type"] != "tool_result" {
				t.Error("expected tool_result type")
			}
		})
		t.Run("has_tool_use_id", func(t *testing.T) {
			block := content[0].(map[string]interface{})
			if _, ok := block["tool_use_id"].(string); !ok {
				t.Error("tool_result missing tool_use_id")
			}
		})
		t.Run("tool_use_id_matches", func(t *testing.T) {
			toolUse := msgs[1].(map[string]interface{})["content"].([]interface{})[1].(map[string]interface{})
			toolResult := content[0].(map[string]interface{})
			if toolUse["id"] != toolResult["tool_use_id"] {
				t.Errorf("tool_use id %v != tool_result tool_use_id %v", toolUse["id"], toolResult["tool_use_id"])
			}
		})
		t.Run("has_content", func(t *testing.T) {
			block := content[0].(map[string]interface{})
			if _, ok := block["content"].(string); !ok {
				t.Error("tool_result missing content string")
			}
		})
	})

	t.Run("tool_result_error", func(t *testing.T) {
		req := loadJSON(t, "request_tool_result_error.json")
		msgs := req["messages"].([]interface{})
		user := msgs[2].(map[string]interface{})
		content := user["content"].([]interface{})
		block := content[0].(map[string]interface{})

		t.Run("is_error_true", func(t *testing.T) {
			if block["is_error"] != true {
				t.Error("expected is_error=true")
			}
		})
		t.Run("content_mentions_denied", func(t *testing.T) {
			c, _ := block["content"].(string)
			if !strings.Contains(c, "permission denied") {
				t.Errorf("expected content to mention permission denied, got %q", c)
			}
		})
	})
}

// TestResponseFormats validates response golden file structure.
func TestResponseFormats(t *testing.T) {
	t.Run("text_response", func(t *testing.T) {
		resp := loadJSON(t, "response_text.json")

		fields := []string{"id", "type", "role", "content", "model", "stop_reason", "usage"}
		for _, f := range fields {
			f := f
			t.Run(fmt.Sprintf("has_%s", f), func(t *testing.T) {
				if _, ok := resp[f]; !ok {
					t.Errorf("missing field %s", f)
				}
			})
		}
		t.Run("role_is_assistant", func(t *testing.T) {
			if resp["role"] != "assistant" {
				t.Errorf("expected assistant, got %v", resp["role"])
			}
		})
		t.Run("stop_reason_end_turn", func(t *testing.T) {
			if resp["stop_reason"] != "end_turn" {
				t.Errorf("expected end_turn, got %v", resp["stop_reason"])
			}
		})
		t.Run("usage_has_input_tokens", func(t *testing.T) {
			usage := resp["usage"].(map[string]interface{})
			if _, ok := usage["input_tokens"]; !ok {
				t.Error("usage missing input_tokens")
			}
		})
		t.Run("usage_has_output_tokens", func(t *testing.T) {
			usage := resp["usage"].(map[string]interface{})
			if _, ok := usage["output_tokens"]; !ok {
				t.Error("usage missing output_tokens")
			}
		})
	})

	t.Run("tool_use_response", func(t *testing.T) {
		resp := loadJSON(t, "response_tool_use.json")

		t.Run("stop_reason_tool_use", func(t *testing.T) {
			if resp["stop_reason"] != "tool_use" {
				t.Errorf("expected tool_use, got %v", resp["stop_reason"])
			}
		})
		t.Run("content_has_text_and_tool_use", func(t *testing.T) {
			content := resp["content"].([]interface{})
			types := map[string]bool{}
			for _, block := range content {
				types[block.(map[string]interface{})["type"].(string)] = true
			}
			if !types["text"] {
				t.Error("missing text block")
			}
			if !types["tool_use"] {
				t.Error("missing tool_use block")
			}
		})
	})
}

// TestStreamingEvents validates the streaming JSONL golden file.
func TestStreamingEvents(t *testing.T) {
	events := loadJSONL(t, "streaming_events.jsonl")

	t.Run("event_count", func(t *testing.T) {
		if len(events) != 11 {
			t.Errorf("expected 11 events, got %d", len(events))
		}
	})

	expectedTypes := []string{
		"message_start", "content_block_start", "content_block_delta",
		"content_block_delta", "content_block_stop", "content_block_start",
		"content_block_delta", "content_block_delta", "content_block_stop",
		"message_delta", "message_stop",
	}

	for i, expected := range expectedTypes {
		i, expected := i, expected
		if i >= len(events) {
			break
		}
		t.Run(fmt.Sprintf("event_%d_type_%s", i, expected), func(t *testing.T) {
			if events[i]["type"] != expected {
				t.Errorf("event[%d] type=%v, want %s", i, events[i]["type"], expected)
			}
		})
	}

	t.Run("first_text_delta", func(t *testing.T) {
		delta := events[2]["delta"].(map[string]interface{})
		if delta["type"] != "text_delta" {
			t.Error("expected text_delta")
		}
		if delta["text"] != "Hello" {
			t.Errorf("expected Hello, got %v", delta["text"])
		}
	})

	t.Run("second_text_delta", func(t *testing.T) {
		delta := events[3]["delta"].(map[string]interface{})
		if delta["text"] != " world" {
			t.Errorf("expected ' world', got %v", delta["text"])
		}
	})

	t.Run("tool_use_block_start", func(t *testing.T) {
		block := events[5]["content_block"].(map[string]interface{})
		if block["type"] != "tool_use" {
			t.Error("expected tool_use")
		}
		if block["name"] != "Bash" {
			t.Errorf("expected Bash, got %v", block["name"])
		}
	})

	t.Run("json_delta_1", func(t *testing.T) {
		delta := events[6]["delta"].(map[string]interface{})
		if delta["type"] != "input_json_delta" {
			t.Error("expected input_json_delta")
		}
	})

	t.Run("json_delta_2", func(t *testing.T) {
		delta := events[7]["delta"].(map[string]interface{})
		pj, _ := delta["partial_json"].(string)
		if !strings.Contains(pj, "ls -la") {
			t.Errorf("expected partial JSON with ls -la, got %q", pj)
		}
	})

	t.Run("json_assembly", func(t *testing.T) {
		d1 := events[6]["delta"].(map[string]interface{})["partial_json"].(string)
		d2 := events[7]["delta"].(map[string]interface{})["partial_json"].(string)
		full := d1 + d2
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(full), &parsed); err != nil {
			t.Fatalf("assembled JSON invalid: %v (got %q)", err, full)
		}
		if parsed["command"] != "ls -la" {
			t.Errorf("expected command=ls -la, got %v", parsed["command"])
		}
	})

	t.Run("message_delta_stop_reason", func(t *testing.T) {
		delta := events[9]["delta"].(map[string]interface{})
		if delta["stop_reason"] != "tool_use" {
			t.Errorf("expected stop_reason=tool_use, got %v", delta["stop_reason"])
		}
	})

	t.Run("message_stop_is_last", func(t *testing.T) {
		last := events[len(events)-1]
		if last["type"] != "message_stop" {
			t.Errorf("last event should be message_stop, got %v", last["type"])
		}
	})
}

// TestErrorResponses validates error response golden file.
func TestErrorResponses(t *testing.T) {
	data := loadJSON(t, "error_responses.json")

	errors := data["errors"].([]interface{})
	t.Run("error_count", func(t *testing.T) {
		if len(errors) != 5 {
			t.Errorf("expected 5 errors, got %d", len(errors))
		}
	})

	expectedErrors := []struct {
		name   string
		status float64
		errTyp string
	}{
		{"context_too_long", 400, "invalid_request_error"},
		{"rate_limit", 429, "rate_limit_error"},
		{"overloaded", 529, "overloaded_error"},
		{"auth_error", 401, "authentication_error"},
		{"server_error", 500, "api_error"},
	}

	for _, exp := range expectedErrors {
		exp := exp
		t.Run(exp.name, func(t *testing.T) {
			var found map[string]interface{}
			for _, e := range errors {
				em := e.(map[string]interface{})
				if em["name"] == exp.name {
					found = em
					break
				}
			}
			if found == nil {
				t.Fatalf("error %q not found", exp.name)
			}
			t.Run("status_code", func(t *testing.T) {
				if found["status"].(float64) != exp.status {
					t.Errorf("expected status %v, got %v", exp.status, found["status"])
				}
			})
			t.Run("has_body", func(t *testing.T) {
				body := found["body"].(map[string]interface{})
				if body["type"] != "error" {
					t.Error("body type should be 'error'")
				}
			})
			t.Run("error_type", func(t *testing.T) {
				body := found["body"].(map[string]interface{})
				errObj := body["error"].(map[string]interface{})
				if errObj["type"] != exp.errTyp {
					t.Errorf("expected error type %q, got %v", exp.errTyp, errObj["type"])
				}
			})
			t.Run("has_message", func(t *testing.T) {
				body := found["body"].(map[string]interface{})
				errObj := body["error"].(map[string]interface{})
				if _, ok := errObj["message"].(string); !ok {
					t.Error("error missing message")
				}
			})
		})
	}

	// Validate stop reasons
	t.Run("stop_reasons", func(t *testing.T) {
		reasons := data["stop_reasons"].([]interface{})
		expected := []string{"end_turn", "tool_use", "max_tokens", "stop_sequence"}
		if len(reasons) != len(expected) {
			t.Fatalf("expected %d stop reasons, got %d", len(expected), len(reasons))
		}
		for i, r := range expected {
			t.Run(r, func(t *testing.T) {
				if reasons[i].(string) != r {
					t.Errorf("expected %q, got %v", r, reasons[i])
				}
			})
		}
	})

	// Validate content block types
	t.Run("content_block_types", func(t *testing.T) {
		types := data["content_block_types"].([]interface{})
		expected := []string{"text", "tool_use", "tool_result"}
		if len(types) != len(expected) {
			t.Fatalf("expected %d types, got %d", len(expected), len(types))
		}
		for i, typ := range expected {
			t.Run(typ, func(t *testing.T) {
				if types[i].(string) != typ {
					t.Errorf("expected %q, got %v", typ, types[i])
				}
			})
		}
	})

	// Validate message roles
	t.Run("message_roles", func(t *testing.T) {
		roles := data["message_roles"].([]interface{})
		expected := []string{"user", "assistant"}
		if len(roles) != len(expected) {
			t.Fatalf("expected %d roles, got %d", len(expected), len(roles))
		}
		for i, role := range expected {
			t.Run(role, func(t *testing.T) {
				if roles[i].(string) != role {
					t.Errorf("expected %q, got %v", role, roles[i])
				}
			})
		}
	})
}

// TestToolDefinitionFormat validates tool definitions match Anthropic API format.
func TestToolDefinitionFormat(t *testing.T) {
	req := loadJSON(t, "request_with_tools.json")
	tools := req["tools"].([]interface{})

	for i, tool := range tools {
		tool := tool.(map[string]interface{})
		name, _ := tool["name"].(string)
		t.Run(fmt.Sprintf("tool_%d_%s", i, name), func(t *testing.T) {
			t.Run("has_name", func(t *testing.T) {
				if name == "" {
					t.Error("missing name")
				}
			})
			t.Run("has_description", func(t *testing.T) {
				if _, ok := tool["description"].(string); !ok {
					t.Error("missing description")
				}
			})
			t.Run("has_input_schema", func(t *testing.T) {
				if _, ok := tool["input_schema"].(map[string]interface{}); !ok {
					t.Error("missing input_schema")
				}
			})
			t.Run("schema_type_object", func(t *testing.T) {
				schema := tool["input_schema"].(map[string]interface{})
				if schema["type"] != "object" {
					t.Error("input_schema type should be object")
				}
			})
			t.Run("schema_has_properties", func(t *testing.T) {
				schema := tool["input_schema"].(map[string]interface{})
				if _, ok := schema["properties"].(map[string]interface{}); !ok {
					t.Error("schema missing properties")
				}
			})
			t.Run("schema_no_additional_properties", func(t *testing.T) {
				schema := tool["input_schema"].(map[string]interface{})
				if schema["additionalProperties"] != false {
					t.Error("schema should have additionalProperties=false")
				}
			})
		})
	}
}
