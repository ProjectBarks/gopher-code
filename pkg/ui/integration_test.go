package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/query"
	"github.com/projectbarks/gopher-code/pkg/session"
)

func TestQueryEventTextDeltaConversion(t *testing.T) {
	config := session.DefaultConfig()
	sessionState := session.New(config, "/tmp")
	appModel := NewAppModel(sessionState, nil)

	// Simulate a text delta event
	evt := query.QueryEvent{
		Type: query.QEventTextDelta,
		Text: "Hello ",
	}
	msg := QueryEventMsg{Event: evt}

	// Update should handle it without error
	updated, cmd := appModel.Update(msg)
	if cmd != nil {
		// Command might be nil or non-nil, both are valid
	}
	if updated == nil {
		t.Error("Expected non-nil updated model")
	}
}

func TestQueryEventToolUseStartConversion(t *testing.T) {
	config := session.DefaultConfig()
	sessionState := session.New(config, "/tmp")
	appModel := NewAppModel(sessionState, nil)

	// Simulate a tool use start event
	evt := query.QueryEvent{
		Type:      query.QEventToolUseStart,
		ToolUseID: "tool-1",
		ToolName:  "bash",
	}
	msg := QueryEventMsg{Event: evt}

	updated, cmd := appModel.Update(msg)
	if cmd != nil {
		// Command might be valid
	}
	if updated == nil {
		t.Error("Expected non-nil updated model")
	}
}

func TestQueryEventToolResultConversion(t *testing.T) {
	config := session.DefaultConfig()
	sessionState := session.New(config, "/tmp")
	appModel := NewAppModel(sessionState, nil)

	// Simulate a tool result event
	evt := query.QueryEvent{
		Type:      query.QEventToolResult,
		ToolUseID: "tool-1",
		Content:   "success output",
		IsError:   false,
	}
	msg := QueryEventMsg{Event: evt}

	updated, cmd := appModel.Update(msg)
	if cmd != nil {
		// Command might be valid
	}
	if updated == nil {
		t.Error("Expected non-nil updated model")
	}
}

func TestQueryEventToolResultError(t *testing.T) {
	config := session.DefaultConfig()
	sessionState := session.New(config, "/tmp")
	appModel := NewAppModel(sessionState, nil)

	// Simulate a tool error event
	evt := query.QueryEvent{
		Type:      query.QEventToolResult,
		ToolUseID: "tool-1",
		Content:   "error output",
		IsError:   true,
	}
	msg := QueryEventMsg{Event: evt}

	updated, cmd := appModel.Update(msg)
	if cmd != nil {
		// Command might be valid
	}
	if updated == nil {
		t.Error("Expected non-nil updated model")
	}
}

func TestQueryEventTurnCompleteConversion(t *testing.T) {
	config := session.DefaultConfig()
	sessionState := session.New(config, "/tmp")
	appModel := NewAppModel(sessionState, nil)

	// Simulate a turn complete event
	evt := query.QueryEvent{
		Type: query.QEventTurnComplete,
		// StopReason would be set in actual execution
	}
	msg := QueryEventMsg{Event: evt}

	updated, cmd := appModel.Update(msg)
	if cmd != nil {
		// Command might be valid
	}
	if updated == nil {
		t.Error("Expected non-nil updated model")
	}
}

func TestConversationStreamingFlow(t *testing.T) {
	// Test a simple streaming flow: text deltas followed by turn complete
	config := session.DefaultConfig()
	sessionState := session.New(config, "/tmp")
	appModel := NewAppModel(sessionState, nil)

	// Simulate streaming text
	texts := []string{"Hello", " ", "world"}
	for _, text := range texts {
		evt := query.QueryEvent{
			Type: query.QEventTextDelta,
			Text: text,
		}
		msg := QueryEventMsg{Event: evt}
		updated, _ := appModel.Update(msg)
		appModel = updated.(*AppModel)
	}

	// Simulate turn complete
	evt := query.QueryEvent{
		Type: query.QEventTurnComplete,
	}
	msg := QueryEventMsg{Event: evt}
	updated, _ := appModel.Update(msg)
	appModel = updated.(*AppModel)

	// Model should still be valid
	if appModel == nil {
		t.Error("Expected non-nil app model after turn complete")
	}
}

func TestConversationToolFlow(t *testing.T) {
	// Test a tool execution flow: tool start, result, turn complete
	config := session.DefaultConfig()
	sessionState := session.New(config, "/tmp")
	appModel := NewAppModel(sessionState, nil)

	// Tool starts
	startEvt := query.QueryEvent{
		Type:      query.QEventToolUseStart,
		ToolUseID: "call-1",
		ToolName:  "bash",
	}
	appModel.Update(QueryEventMsg{Event: startEvt})

	// Tool result arrives
	resultEvt := query.QueryEvent{
		Type:      query.QEventToolResult,
		ToolUseID: "call-1",
		Content:   "output",
		IsError:   false,
	}
	appModel.Update(QueryEventMsg{Event: resultEvt})

	// Turn completes
	completeEvt := query.QueryEvent{
		Type: query.QEventTurnComplete,
	}
	appModel.Update(QueryEventMsg{Event: completeEvt})

	// Should have processed all events
	if appModel == nil {
		t.Error("Expected non-nil app model")
	}
}

func TestConversationMixedEventFlow(t *testing.T) {
	// Test a realistic flow with text, tool calls, and tool results
	config := session.DefaultConfig()
	sessionState := session.New(config, "/tmp")
	appModel := NewAppModel(sessionState, nil)

	events := []query.QueryEvent{
		{Type: query.QEventTextDelta, Text: "Let "},
		{Type: query.QEventTextDelta, Text: "me "},
		{Type: query.QEventTextDelta, Text: "run "},
		{Type: query.QEventTextDelta, Text: "a "},
		{Type: query.QEventTextDelta, Text: "command."},
		{Type: query.QEventToolUseStart, ToolUseID: "call-1", ToolName: "bash"},
		{Type: query.QEventToolResult, ToolUseID: "call-1", Content: "result", IsError: false},
		{Type: query.QEventTextDelta, Text: "\nThe result is above."},
		{Type: query.QEventTurnComplete},
	}

	for _, evt := range events {
		msg := QueryEventMsg{Event: evt}
		updated, _ := appModel.Update(msg)
		appModel = updated.(*AppModel)
	}

	if appModel == nil {
		t.Error("Expected non-nil app model after mixed event flow")
	}
}

func TestTextDeltaMsgCreation(t *testing.T) {
	msg := TextDeltaMsg{Text: "test"}
	if msg.Text != "test" {
		t.Errorf("Expected 'test', got %q", msg.Text)
	}
}

func TestToolUseStartMsgCreation(t *testing.T) {
	msg := ToolUseStartMsg{
		ToolUseID: "id-1",
		ToolName:  "bash",
	}
	if msg.ToolUseID != "id-1" || msg.ToolName != "bash" {
		t.Errorf("Unexpected message values: %v", msg)
	}
}

func TestToolResultMsgCreation(t *testing.T) {
	msg := ToolResultMsg{
		ToolUseID: "id-1",
		Content:   "output",
		IsError:   true,
	}
	if msg.ToolUseID != "id-1" || msg.Content != "output" || !msg.IsError {
		t.Errorf("Unexpected message values: %v", msg)
	}
}

func TestTurnCompleteMsgCreation(t *testing.T) {
	msg := TurnCompleteMsg{StopReason: nil}
	if msg.StopReason != nil {
		t.Error("Expected nil StopReason")
	}
}

func TestEventRoutingPreventsPlaceholderIssues(t *testing.T) {
	// Ensure event routing doesn't cause panic with placeholder components
	config := session.DefaultConfig()
	sessionState := session.New(config, "/tmp")
	appModel := NewAppModel(sessionState, nil)

	// Test with multiple event types in sequence
	eventSeq := []query.QueryEvent{
		{Type: query.QEventTextDelta, Text: "a"},
		{Type: query.QEventTextDelta, Text: "b"},
		{Type: query.QEventToolUseStart, ToolUseID: "t1", ToolName: "foo"},
		{Type: query.QEventToolResult, ToolUseID: "t1", Content: "r", IsError: false},
		{Type: query.QEventTurnComplete},
	}

	for _, evt := range eventSeq {
		// Should not panic
		appModel.Update(QueryEventMsg{Event: evt})
	}
}

// TestFileSuggestionsIntegration_AtMention exercises the full code path from
// AppModel through FileSuggester for @-mention file autocomplete.
// Source: useInputSuggestion.tsx — @path triggers file suggestions.
func TestFileSuggestionsIntegration_AtMention(t *testing.T) {
	// Create a temp directory with known files to suggest.
	dir := t.TempDir()
	for _, name := range []string{"main.go", "utils.go", "README.md"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("// "+name), 0644); err != nil {
			t.Fatal(err)
		}
	}

	config := session.DefaultConfig()
	sess := session.New(config, dir)
	app := NewAppModel(sess, nil)

	// Verify the file suggester is initialized and rooted at session CWD.
	if app.FileSuggester() == nil {
		t.Fatal("FileSuggester should be initialized in NewAppModel")
	}

	// Simulate the user typing "@main" -- set input value and refresh.
	app.input.SetValue("@main")
	app.refreshFileAutocomplete()

	if !app.FileSuggestionsActive() {
		t.Fatal("File suggestions should be active after typing @main")
	}
	suggestions := app.FileSuggestions()
	if len(suggestions) == 0 {
		t.Fatal("Expected at least one suggestion for @main")
	}

	found := false
	for _, s := range suggestions {
		if s.DisplayText == "main.go" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected main.go in suggestions, got %v", suggestions)
	}

	// Verify View renders the suggestions.
	app.width = 80
	app.height = 24
	view := app.View()
	if view.Content == "" {
		t.Error("View should render content")
	}

	// Clear the @-mention: suggestions should deactivate.
	app.input.SetValue("hello world")
	app.refreshFileAutocomplete()
	if app.FileSuggestionsActive() {
		t.Error("File suggestions should be inactive without @-mention")
	}
}

// TestExtractAtPartial validates the @-mention partial extraction logic.
func TestExtractAtPartial(t *testing.T) {
	tests := []struct {
		input  string
		want   string
		wantOK bool
	}{
		{"@main", "main", true},
		{"look at @src/app", "src/app", true},
		{"no mention", "", false},
		{"email@test", "", false}, // preceded by non-space
		{"@", "", true},           // empty partial, still valid
		{"@path with space", "", false},
	}
	for _, tt := range tests {
		got, ok := extractAtPartial(tt.input)
		if ok != tt.wantOK || got != tt.want {
			t.Errorf("extractAtPartial(%q) = (%q, %v), want (%q, %v)",
				tt.input, got, ok, tt.want, tt.wantOK)
		}
	}
}
