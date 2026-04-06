package hooks

import (
	"encoding/json"
	"testing"
)

// Source: utils/hooks/hookHelpers.ts

func TestHookResponseSchema(t *testing.T) {
	// Source: hookHelpers.ts:16-24
	t.Run("ok_true", func(t *testing.T) {
		resp := HookResponse{OK: true}
		data, err := json.Marshal(resp)
		if err != nil {
			t.Fatal(err)
		}
		var parsed HookResponse
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Fatal(err)
		}
		if !parsed.OK {
			t.Error("expected ok=true")
		}
		if parsed.Reason != "" {
			t.Errorf("expected empty reason, got %q", parsed.Reason)
		}
	})

	t.Run("ok_false_with_reason", func(t *testing.T) {
		resp := HookResponse{OK: false, Reason: "command not safe"}
		data, err := json.Marshal(resp)
		if err != nil {
			t.Fatal(err)
		}
		var parsed HookResponse
		json.Unmarshal(data, &parsed)
		if parsed.OK {
			t.Error("expected ok=false")
		}
		if parsed.Reason != "command not safe" {
			t.Errorf("reason = %q", parsed.Reason)
		}
	})

	t.Run("parse_from_json", func(t *testing.T) {
		data := `{"ok":false,"reason":"denied"}`
		var resp HookResponse
		if err := json.Unmarshal([]byte(data), &resp); err != nil {
			t.Fatal(err)
		}
		if resp.OK {
			t.Error("expected false")
		}
		if resp.Reason != "denied" {
			t.Errorf("reason = %q", resp.Reason)
		}
	})

	t.Run("omit_empty_reason", func(t *testing.T) {
		resp := HookResponse{OK: true}
		data, _ := json.Marshal(resp)
		var parsed map[string]interface{}
		json.Unmarshal(data, &parsed)
		if _, ok := parsed["reason"]; ok {
			t.Error("expected reason to be omitted when empty")
		}
	})
}

func TestHookResponseJSONSchema(t *testing.T) {
	// Source: hookHelpers.ts:42-58
	schema := HookResponseJSONSchema()
	if schema["type"] != "object" {
		t.Errorf("type = %v", schema["type"])
	}
	props, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("properties not a map")
	}
	if _, ok := props["ok"]; !ok {
		t.Error("missing ok property")
	}
	if _, ok := props["reason"]; !ok {
		t.Error("missing reason property")
	}
	required, ok := schema["required"].([]string)
	if !ok {
		t.Fatal("required not a string slice")
	}
	if len(required) != 1 || required[0] != "ok" {
		t.Errorf("required = %v", required)
	}
	if schema["additionalProperties"] != false {
		t.Error("additionalProperties should be false")
	}
}

func TestAddArgumentsToPrompt(t *testing.T) {
	// Source: hookHelpers.ts:30-35

	t.Run("replace_arguments_placeholder", func(t *testing.T) {
		result := AddArgumentsToPrompt("Check if $ARGUMENTS is safe", `{"command":"rm -rf /"}`)
		if result != `Check if {"command":"rm -rf /"} is safe` {
			t.Errorf("got %q", result)
		}
	})

	t.Run("no_placeholder_appends_nothing", func(t *testing.T) {
		result := AddArgumentsToPrompt("Just a prompt with no placeholders", `{"key":"val"}`)
		if result != "Just a prompt with no placeholders" {
			t.Errorf("got %q", result)
		}
	})

	t.Run("multiple_arguments_placeholders", func(t *testing.T) {
		result := AddArgumentsToPrompt("First: $ARGUMENTS, Second: $ARGUMENTS", `"hello"`)
		if result != `First: "hello", Second: "hello"` {
			t.Errorf("got %q", result)
		}
	})
}

func TestSubstituteArguments_IndexedAccess(t *testing.T) {
	// Source: utils/argumentSubstitution.ts — indexed arguments

	t.Run("array_indexed", func(t *testing.T) {
		result := SubstituteArguments("arg0=$ARGUMENTS[0] arg1=$ARGUMENTS[1]", `["foo","bar"]`)
		if result != "arg0=foo arg1=bar" {
			t.Errorf("got %q", result)
		}
	})

	t.Run("shorthand_indexed", func(t *testing.T) {
		result := SubstituteArguments("$0 and $1", `["first","second"]`)
		if result != "first and second" {
			t.Errorf("got %q", result)
		}
	})

	t.Run("non_array_full_replacement", func(t *testing.T) {
		// When input is not an array, $ARGUMENTS[N] has $ARGUMENTS replaced,
		// leaving the [N] suffix (indexed replacement is a no-op for non-arrays)
		result := SubstituteArguments("val=$ARGUMENTS", `{"key":"val"}`)
		if result != `val={"key":"val"}` {
			t.Errorf("got %q", result)
		}
	})
}

func TestStructuredOutputToolPrompt(t *testing.T) {
	// Source: hookHelpers.ts:61
	expected := "Use this tool to return your verification result. You MUST call this tool exactly once at the end of your response."
	if StructuredOutputToolPrompt != expected {
		t.Errorf("got %q", StructuredOutputToolPrompt)
	}
}

func TestStructuredOutputEnforcementMessage(t *testing.T) {
	// Source: hookHelpers.ts:80
	msg := StructuredOutputEnforcementMessage()
	if msg != "You MUST call the "+SyntheticOutputToolName+" tool to complete this request. Call this tool now." {
		t.Errorf("got %q", msg)
	}
}

func TestStructuredOutputEnforcementTimeout(t *testing.T) {
	// Source: hookHelpers.ts:81
	if StructuredOutputEnforcementTimeout != 5000 {
		t.Errorf("expected 5000, got %d", StructuredOutputEnforcementTimeout)
	}
}
