package remote

import (
	"encoding/json"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// T84: ConvertedMessage union tests
// T83: ConvertSDKMessage converter tests
// T85: Predicate tests
// ---------------------------------------------------------------------------

func TestConvertSDKMessage_Assistant(t *testing.T) {
	raw := json.RawMessage(`{
		"type": "assistant",
		"uuid": "u-asst",
		"session_id": "s1",
		"message": {"role": "assistant", "content": [{"type": "text", "text": "Hello"}]}
	}`)

	result := ConvertSDKMessage(raw, nil)

	if result.Type != ConvertedMsg {
		t.Fatalf("Type = %q, want %q", result.Type, ConvertedMsg)
	}
	if result.Message == nil {
		t.Fatal("Message is nil")
	}
	if result.Message.Kind != "assistant" {
		t.Errorf("Kind = %q", result.Message.Kind)
	}
	if result.Message.UUID != "u-asst" {
		t.Errorf("UUID = %q", result.Message.UUID)
	}
	if result.Message.Raw == nil {
		t.Error("Raw should contain original message")
	}
}

func TestConvertSDKMessage_StreamEvent(t *testing.T) {
	raw := json.RawMessage(`{
		"type": "stream_event",
		"event": {"type": "content_block_delta", "delta": {"type": "text_delta", "text": "hi"}},
		"uuid": "u1",
		"session_id": "s1"
	}`)

	result := ConvertSDKMessage(raw, nil)

	if result.Type != ConvertedStreamEvent {
		t.Fatalf("Type = %q, want %q", result.Type, ConvertedStreamEvent)
	}
	if result.StreamEvent == nil {
		t.Fatal("StreamEvent is nil")
	}
	if result.StreamEvent.Event == nil {
		t.Error("Event should not be nil")
	}
}

func TestConvertSDKMessage_ResultError(t *testing.T) {
	raw := json.RawMessage(`{
		"type": "result",
		"subtype": "error_during_execution",
		"errors": ["something broke", "very bad"],
		"uuid": "u-err",
		"session_id": "s1"
	}`)

	result := ConvertSDKMessage(raw, nil)

	if result.Type != ConvertedMsg {
		t.Fatalf("Type = %q, want message", result.Type)
	}
	if result.Message.Kind != "system" {
		t.Errorf("Kind = %q", result.Message.Kind)
	}
	if result.Message.Level != "warning" {
		t.Errorf("Level = %q", result.Message.Level)
	}
	if !strings.Contains(result.Message.Content, "something broke") {
		t.Errorf("Content = %q", result.Message.Content)
	}
	if !strings.Contains(result.Message.Content, "very bad") {
		t.Errorf("Content should contain both errors: %q", result.Message.Content)
	}
}

func TestConvertSDKMessage_ResultSuccess_Ignored(t *testing.T) {
	raw := json.RawMessage(`{
		"type": "result",
		"subtype": "success",
		"result": "done",
		"uuid": "u-ok",
		"session_id": "s1"
	}`)

	result := ConvertSDKMessage(raw, nil)

	if result.Type != ConvertedIgnored {
		t.Errorf("Type = %q, want ignored (success results are noise)", result.Type)
	}
}

func TestConvertSDKMessage_SystemInit(t *testing.T) {
	raw := json.RawMessage(`{
		"type": "system",
		"subtype": "init",
		"model": "claude-3-opus",
		"uuid": "u-init",
		"session_id": "s1"
	}`)

	result := ConvertSDKMessage(raw, nil)

	if result.Type != ConvertedMsg {
		t.Fatalf("Type = %q", result.Type)
	}
	if !strings.Contains(result.Message.Content, "claude-3-opus") {
		t.Errorf("Content = %q", result.Message.Content)
	}
	if !strings.Contains(result.Message.Content, "Remote session initialized") {
		t.Errorf("Content = %q", result.Message.Content)
	}
}

func TestConvertSDKMessage_SystemStatusCompacting(t *testing.T) {
	raw := json.RawMessage(`{
		"type": "system",
		"subtype": "status",
		"status": "compacting",
		"uuid": "u-st",
		"session_id": "s1"
	}`)

	result := ConvertSDKMessage(raw, nil)

	if result.Type != ConvertedMsg {
		t.Fatalf("Type = %q", result.Type)
	}
	// Source: sdkMessageAdapter.ts:98 — "Compacting conversation…"
	if result.Message.Content != "Compacting conversation\u2026" {
		t.Errorf("Content = %q", result.Message.Content)
	}
}

func TestConvertSDKMessage_SystemStatusEmpty_Ignored(t *testing.T) {
	raw := json.RawMessage(`{
		"type": "system",
		"subtype": "status",
		"status": "",
		"uuid": "u-st",
		"session_id": "s1"
	}`)

	result := ConvertSDKMessage(raw, nil)

	if result.Type != ConvertedIgnored {
		t.Errorf("Type = %q, want ignored for empty status", result.Type)
	}
}

func TestConvertSDKMessage_SystemCompactBoundary(t *testing.T) {
	raw := json.RawMessage(`{
		"type": "system",
		"subtype": "compact_boundary",
		"uuid": "u-cb",
		"session_id": "s1",
		"compact_metadata": {"trigger": "auto", "pre_tokens": 5000}
	}`)

	result := ConvertSDKMessage(raw, nil)

	if result.Type != ConvertedMsg {
		t.Fatalf("Type = %q", result.Type)
	}
	if result.Message.Content != "Conversation compacted" {
		t.Errorf("Content = %q", result.Message.Content)
	}
	if result.Message.Subtype != "compact_boundary" {
		t.Errorf("Subtype = %q", result.Message.Subtype)
	}
}

func TestConvertSDKMessage_ToolProgress(t *testing.T) {
	raw := json.RawMessage(`{
		"type": "tool_progress",
		"tool_name": "Bash",
		"tool_use_id": "tu-1",
		"elapsed_time_seconds": 42,
		"uuid": "u-tp",
		"session_id": "s1"
	}`)

	result := ConvertSDKMessage(raw, nil)

	if result.Type != ConvertedMsg {
		t.Fatalf("Type = %q", result.Type)
	}
	if !strings.Contains(result.Message.Content, "Bash") {
		t.Errorf("Content = %q, should mention tool name", result.Message.Content)
	}
	if !strings.Contains(result.Message.Content, "42s") {
		t.Errorf("Content = %q, should mention elapsed time", result.Message.Content)
	}
	if result.Message.ToolUseID != "tu-1" {
		t.Errorf("ToolUseID = %q", result.Message.ToolUseID)
	}
}

func TestConvertSDKMessage_UserIgnoredByDefault(t *testing.T) {
	raw := json.RawMessage(`{
		"type": "user",
		"uuid": "u-user",
		"session_id": "s1",
		"message": {"role": "user", "content": "hello"},
		"parent_tool_use_id": null
	}`)

	result := ConvertSDKMessage(raw, nil)

	if result.Type != ConvertedIgnored {
		t.Errorf("Type = %q, want ignored (user messages ignored by default)", result.Type)
	}
}

func TestConvertSDKMessage_UserWithConvertTextOption(t *testing.T) {
	raw := json.RawMessage(`{
		"type": "user",
		"uuid": "u-user",
		"session_id": "s1",
		"message": {"role": "user", "content": "hello world"},
		"parent_tool_use_id": null
	}`)

	result := ConvertSDKMessage(raw, &ConvertOptions{ConvertUserTextMessages: true})

	if result.Type != ConvertedMsg {
		t.Fatalf("Type = %q, want message with ConvertUserTextMessages", result.Type)
	}
	if result.Message.Kind != "user" {
		t.Errorf("Kind = %q", result.Message.Kind)
	}
	if result.Message.Content != "hello world" {
		t.Errorf("Content = %q", result.Message.Content)
	}
}

func TestConvertSDKMessage_UserToolResult(t *testing.T) {
	raw := json.RawMessage(`{
		"type": "user",
		"uuid": "u-tr",
		"session_id": "s1",
		"message": {"role": "user", "content": [{"type": "tool_result", "tool_use_id": "tu-1", "content": "ok"}]},
		"parent_tool_use_id": null
	}`)

	result := ConvertSDKMessage(raw, &ConvertOptions{ConvertToolResults: true})

	if result.Type != ConvertedMsg {
		t.Fatalf("Type = %q, want message with ConvertToolResults", result.Type)
	}
	if result.Message.Subtype != "tool_result" {
		t.Errorf("Subtype = %q", result.Message.Subtype)
	}
}

func TestConvertSDKMessage_IgnoredTypes(t *testing.T) {
	ignoredTypes := []string{
		`{"type": "auth_status", "uuid": "u1"}`,
		`{"type": "tool_use_summary", "uuid": "u2"}`,
		`{"type": "rate_limit_event", "uuid": "u3"}`,
		`{"type": "totally_unknown_type", "uuid": "u4"}`,
	}

	for _, raw := range ignoredTypes {
		result := ConvertSDKMessage(json.RawMessage(raw), nil)
		if result.Type != ConvertedIgnored {
			t.Errorf("Type = %q for %s, want ignored", result.Type, raw)
		}
	}
}

func TestConvertSDKMessage_SystemHookResponse_Ignored(t *testing.T) {
	raw := json.RawMessage(`{
		"type": "system",
		"subtype": "hook_response",
		"uuid": "u-hr",
		"session_id": "s1"
	}`)

	result := ConvertSDKMessage(raw, nil)

	if result.Type != ConvertedIgnored {
		t.Errorf("Type = %q, want ignored for hook_response", result.Type)
	}
}

// ---------------------------------------------------------------------------
// T85: Predicate tests
// ---------------------------------------------------------------------------

func TestIsSessionEndMessage(t *testing.T) {
	tests := []struct {
		raw  string
		want bool
	}{
		{`{"type":"result","subtype":"success"}`, true},
		{`{"type":"result","subtype":"error_during_execution"}`, true},
		{`{"type":"assistant"}`, false},
		{`{"type":"system","subtype":"init"}`, false},
		{`{}`, false},
		{`not-json`, false},
	}

	for _, tt := range tests {
		got := IsSessionEndMessage(json.RawMessage(tt.raw))
		if got != tt.want {
			t.Errorf("IsSessionEndMessage(%s) = %v, want %v", tt.raw, got, tt.want)
		}
	}
}

func TestIsSuccessResult(t *testing.T) {
	tests := []struct {
		raw  string
		want bool
	}{
		{`{"subtype":"success"}`, true},
		{`{"subtype":"error_during_execution"}`, false},
		{`{"subtype":"error_max_turns"}`, false},
		{`{}`, false},
	}

	for _, tt := range tests {
		got := IsSuccessResult(json.RawMessage(tt.raw))
		if got != tt.want {
			t.Errorf("IsSuccessResult(%s) = %v, want %v", tt.raw, got, tt.want)
		}
	}
}

func TestGetResultText(t *testing.T) {
	t.Run("success_with_result", func(t *testing.T) {
		raw := json.RawMessage(`{"subtype":"success","result":"Task completed"}`)
		got := GetResultText(raw)
		if got != "Task completed" {
			t.Errorf("GetResultText = %q, want %q", got, "Task completed")
		}
	})

	t.Run("error_returns_empty", func(t *testing.T) {
		raw := json.RawMessage(`{"subtype":"error_during_execution","errors":["oops"]}`)
		got := GetResultText(raw)
		if got != "" {
			t.Errorf("GetResultText = %q, want empty for error", got)
		}
	})

	t.Run("invalid_json_returns_empty", func(t *testing.T) {
		got := GetResultText(json.RawMessage(`not-json`))
		if got != "" {
			t.Errorf("GetResultText = %q, want empty for invalid JSON", got)
		}
	})
}
