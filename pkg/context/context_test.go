package context

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/message"
)

// ---------------------------------------------------------------------------
// calculateContextPercentages
// ---------------------------------------------------------------------------

func TestCalculateContextPercentages_NilUsage(t *testing.T) {
	got := CalculateContextPercentages(nil, 200_000)
	if got.Used != nil || got.Remaining != nil {
		t.Fatalf("expected nil/nil, got %v/%v", got.Used, got.Remaining)
	}
}

func TestCalculateContextPercentages_Basic(t *testing.T) {
	usage := &TokenUsage{
		InputTokens:              50_000,
		CacheCreationInputTokens: 20_000,
		CacheReadInputTokens:     10_000,
	}
	got := CalculateContextPercentages(usage, 200_000)
	// (50k+20k+10k)/200k = 40%
	if got.Used == nil || *got.Used != 40 {
		t.Fatalf("expected used=40, got %v", got.Used)
	}
	if got.Remaining == nil || *got.Remaining != 60 {
		t.Fatalf("expected remaining=60, got %v", got.Remaining)
	}
}

func TestCalculateContextPercentages_ClampAt100(t *testing.T) {
	usage := &TokenUsage{
		InputTokens:              200_000,
		CacheCreationInputTokens: 50_000,
		CacheReadInputTokens:     0,
	}
	got := CalculateContextPercentages(usage, 200_000)
	if got.Used == nil || *got.Used != 100 {
		t.Fatalf("expected used=100 (clamped), got %v", got.Used)
	}
	if got.Remaining == nil || *got.Remaining != 0 {
		t.Fatalf("expected remaining=0, got %v", got.Remaining)
	}
}

func TestCalculateContextPercentages_ZeroWindow(t *testing.T) {
	// Edge case: zero window size should not panic
	usage := &TokenUsage{InputTokens: 100}
	got := CalculateContextPercentages(usage, 0)
	// Division by zero produces +Inf -> clamped to 100
	if got.Used == nil || *got.Used != 100 {
		t.Fatalf("expected used=100 (clamped), got %v", got.Used)
	}
}

// ---------------------------------------------------------------------------
// analyzeContext — token stats from messages
// ---------------------------------------------------------------------------

func TestAnalyzeContext_EmptyMessages(t *testing.T) {
	stats := AnalyzeContext(nil)
	if stats.Total != 0 {
		t.Fatalf("expected total=0, got %d", stats.Total)
	}
}

func TestAnalyzeContext_UserTextMessage(t *testing.T) {
	msgs := []message.Message{
		message.UserMessage("Hello world"),
	}
	stats := AnalyzeContext(msgs)
	if stats.HumanMessages == 0 {
		t.Fatal("expected nonzero humanMessages tokens")
	}
	if stats.Total == 0 {
		t.Fatal("expected nonzero total")
	}
	if stats.AssistantMessages != 0 {
		t.Fatalf("expected zero assistantMessages, got %d", stats.AssistantMessages)
	}
}

func TestAnalyzeContext_AssistantTextMessage(t *testing.T) {
	msgs := []message.Message{
		{Role: message.RoleAssistant, Content: []message.ContentBlock{
			message.TextBlock("I can help with that."),
		}},
	}
	stats := AnalyzeContext(msgs)
	if stats.AssistantMessages == 0 {
		t.Fatal("expected nonzero assistantMessages tokens")
	}
	if stats.HumanMessages != 0 {
		t.Fatalf("expected zero humanMessages, got %d", stats.HumanMessages)
	}
}

func TestAnalyzeContext_ToolUseAndResult(t *testing.T) {
	msgs := []message.Message{
		{Role: message.RoleAssistant, Content: []message.ContentBlock{
			message.ToolUseBlock("tu1", "Read", json.RawMessage(`{"file_path":"/tmp/foo.go"}`)),
		}},
		{Role: message.RoleUser, Content: []message.ContentBlock{
			message.ToolResultBlock("tu1", "package main\nfunc main() {}", false),
		}},
	}
	stats := AnalyzeContext(msgs)
	if stats.ToolRequests["Read"] == 0 {
		t.Fatal("expected nonzero Read tool request tokens")
	}
	if stats.ToolResults["Read"] == 0 {
		t.Fatal("expected nonzero Read tool result tokens")
	}
	if stats.Total == 0 {
		t.Fatal("expected nonzero total")
	}
}

func TestAnalyzeContext_DuplicateFileReads(t *testing.T) {
	msgs := []message.Message{
		{Role: message.RoleAssistant, Content: []message.ContentBlock{
			message.ToolUseBlock("tu1", "Read", json.RawMessage(`{"file_path":"/tmp/foo.go"}`)),
		}},
		{Role: message.RoleUser, Content: []message.ContentBlock{
			message.ToolResultBlock("tu1", "file content here", false),
		}},
		{Role: message.RoleAssistant, Content: []message.ContentBlock{
			message.ToolUseBlock("tu2", "Read", json.RawMessage(`{"file_path":"/tmp/foo.go"}`)),
		}},
		{Role: message.RoleUser, Content: []message.ContentBlock{
			message.ToolResultBlock("tu2", "file content here", false),
		}},
	}
	stats := AnalyzeContext(msgs)
	dup, ok := stats.DuplicateFileReads["/tmp/foo.go"]
	if !ok {
		t.Fatal("expected duplicate file read entry for /tmp/foo.go")
	}
	if dup.Count != 2 {
		t.Fatalf("expected count=2, got %d", dup.Count)
	}
	if dup.Tokens == 0 {
		t.Fatal("expected nonzero duplicate tokens")
	}
}

func TestAnalyzeContext_LocalCommandOutput(t *testing.T) {
	msgs := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentBlock{
			{Type: message.ContentText, Text: "local-command-stdout: some output from a command"},
		}},
	}
	stats := AnalyzeContext(msgs)
	if stats.LocalCommandOutputs == 0 {
		t.Fatal("expected nonzero localCommandOutputs")
	}
	if stats.HumanMessages != 0 {
		t.Fatalf("expected humanMessages=0 for local command, got %d", stats.HumanMessages)
	}
}

func TestAnalyzeContext_OtherBlockTypes(t *testing.T) {
	msgs := []message.Message{
		{Role: message.RoleAssistant, Content: []message.ContentBlock{
			{Type: message.ContentThinking, Thinking: "Let me think about this..."},
		}},
	}
	stats := AnalyzeContext(msgs)
	if stats.Other == 0 {
		t.Fatal("expected nonzero other tokens for thinking blocks")
	}
}

// ---------------------------------------------------------------------------
// tokenStatsToMetrics
// ---------------------------------------------------------------------------

func TestTokenStatsToMetrics_Basic(t *testing.T) {
	stats := &TokenStats{
		Total:           1000,
		HumanMessages:   200,
		AssistantMessages: 300,
		LocalCommandOutputs: 100,
		Other:           50,
		ToolRequests:    map[string]int{"Read": 150, "Bash": 100},
		ToolResults:     map[string]int{"Read": 80, "Bash": 20},
		DuplicateFileReads: map[string]DuplicateRead{
			"/tmp/a.go": {Count: 2, Tokens: 40},
		},
	}
	m := TokenStatsToMetrics(stats)

	assertMetric(t, m, "total_tokens", 1000)
	assertMetric(t, m, "human_message_tokens", 200)
	assertMetric(t, m, "assistant_message_tokens", 300)
	assertMetric(t, m, "local_command_output_tokens", 100)
	assertMetric(t, m, "other_tokens", 50)
	assertMetric(t, m, "tool_request_Read_tokens", 150)
	assertMetric(t, m, "tool_result_Read_tokens", 80)
	assertMetric(t, m, "duplicate_read_tokens", 40)
	assertMetric(t, m, "duplicate_read_file_count", 1)

	// Percentages
	assertMetric(t, m, "human_message_percent", 20)
	assertMetric(t, m, "assistant_message_percent", 30)
	assertMetric(t, m, "tool_request_percent", 25) // (150+100)/1000 = 25%
	assertMetric(t, m, "tool_result_percent", 10)  // (80+20)/1000 = 10%
}

func TestTokenStatsToMetrics_ZeroTotal(t *testing.T) {
	stats := &TokenStats{
		ToolRequests:       map[string]int{},
		ToolResults:        map[string]int{},
		DuplicateFileReads: map[string]DuplicateRead{},
	}
	m := TokenStatsToMetrics(stats)
	// Should not panic, no percentage keys
	if _, ok := m["human_message_percent"]; ok {
		t.Fatal("should not have percent keys when total=0")
	}
}

// ---------------------------------------------------------------------------
// insertBlockAfterToolResults
// ---------------------------------------------------------------------------

func TestInsertBlockAfterToolResults_WithToolResults(t *testing.T) {
	content := []message.ContentBlock{
		message.ToolResultBlock("tu1", "result1", false),
		message.ToolResultBlock("tu2", "result2", false),
		message.TextBlock("hello"),
	}
	block := message.TextBlock("injected")
	result := InsertBlockAfterToolResults(content, block)

	// Should be inserted after last tool_result (index 2), before "hello"
	if len(result) != 4 {
		t.Fatalf("expected 4 blocks, got %d", len(result))
	}
	if result[2].Text != "injected" {
		t.Fatalf("expected injected block at index 2, got %q", result[2].Text)
	}
}

func TestInsertBlockAfterToolResults_NoToolResults(t *testing.T) {
	content := []message.ContentBlock{
		message.TextBlock("first"),
		message.TextBlock("second"),
	}
	block := message.TextBlock("injected")
	result := InsertBlockAfterToolResults(content, block)

	// No tool_results: insert before last block
	if len(result) != 3 {
		t.Fatalf("expected 3 blocks, got %d", len(result))
	}
	if result[1].Text != "injected" {
		t.Fatalf("expected injected at index 1, got %q", result[1].Text)
	}
}

func TestInsertBlockAfterToolResults_AppendsContinuation(t *testing.T) {
	// When inserted block becomes the last element, a continuation text is appended
	content := []message.ContentBlock{
		message.ToolResultBlock("tu1", "result1", false),
	}
	block := message.TextBlock("injected")
	result := InsertBlockAfterToolResults(content, block)

	// tool_result, injected, continuation "."
	if len(result) != 3 {
		t.Fatalf("expected 3 blocks (with continuation), got %d", len(result))
	}
	if result[2].Text != "." {
		t.Fatalf("expected continuation block '.', got %q", result[2].Text)
	}
}

func TestInsertBlockAfterToolResults_EmptyContent(t *testing.T) {
	var content []message.ContentBlock
	block := message.TextBlock("injected")
	result := InsertBlockAfterToolResults(content, block)

	// Empty: insert at index 0
	if len(result) != 1 {
		t.Fatalf("expected 1 block, got %d", len(result))
	}
}

// ---------------------------------------------------------------------------
// Context suggestions
// ---------------------------------------------------------------------------

func TestGenerateContextSuggestions_NearCapacity(t *testing.T) {
	data := ContextData{
		Percentage:           85,
		RawMaxTokens:         200_000,
		IsAutoCompactEnabled: true,
	}
	suggestions := GenerateContextSuggestions(data)
	if len(suggestions) == 0 {
		t.Fatal("expected at least one suggestion for 85% usage")
	}
	if suggestions[0].Severity != SeverityWarning {
		t.Fatalf("expected warning severity, got %s", suggestions[0].Severity)
	}
}

func TestGenerateContextSuggestions_AutoCompactDisabled(t *testing.T) {
	data := ContextData{
		Percentage:           60,
		RawMaxTokens:         200_000,
		IsAutoCompactEnabled: false,
	}
	suggestions := GenerateContextSuggestions(data)
	found := false
	for _, s := range suggestions {
		if s.Title == "Autocompact is disabled" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected autocompact disabled suggestion")
	}
}

func TestGenerateContextSuggestions_LargeToolResults(t *testing.T) {
	data := ContextData{
		Percentage:           50,
		RawMaxTokens:         200_000,
		IsAutoCompactEnabled: true,
		MessageBreakdown: &MessageBreakdown{
			ToolCallsByType: []ToolCallBreakdown{
				{Name: "Bash", CallTokens: 5_000, ResultTokens: 30_000},
			},
		},
	}
	suggestions := GenerateContextSuggestions(data)
	if len(suggestions) == 0 {
		t.Fatal("expected suggestion for large Bash results")
	}
}

func TestGenerateContextSuggestions_SortOrder(t *testing.T) {
	data := ContextData{
		Percentage:           85,
		RawMaxTokens:         200_000,
		IsAutoCompactEnabled: true,
		MessageBreakdown: &MessageBreakdown{
			ToolCallsByType: []ToolCallBreakdown{
				{Name: "Bash", CallTokens: 5_000, ResultTokens: 30_000},
			},
		},
	}
	suggestions := GenerateContextSuggestions(data)
	// Warnings should come before info
	if len(suggestions) < 2 {
		t.Fatalf("expected at least 2 suggestions, got %d", len(suggestions))
	}
	if suggestions[0].Severity != SeverityWarning {
		t.Fatal("expected first suggestion to be warning")
	}
}

func TestGenerateContextSuggestions_MemoryBloat(t *testing.T) {
	data := ContextData{
		Percentage:           50,
		RawMaxTokens:         200_000,
		IsAutoCompactEnabled: true,
		MemoryFiles: []MemoryFile{
			{Path: "/project/.claude/settings.md", Tokens: 12_000},
		},
	}
	suggestions := GenerateContextSuggestions(data)
	found := false
	for _, s := range suggestions {
		if s.SavingsTokens > 0 {
			found = true
		}
	}
	if !found {
		t.Fatal("expected memory bloat suggestion with savings")
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func assertMetric(t *testing.T, m map[string]int, key string, expected int) {
	t.Helper()
	got, ok := m[key]
	if !ok {
		t.Fatalf("missing metric %q", key)
	}
	if got != expected {
		t.Fatalf("metric %q: expected %d, got %d", key, expected, got)
	}
}

// Sanity: rough token estimation roundtrip
func TestRoughTokenEstimation_Sanity(t *testing.T) {
	// "Hello world" is 11 chars -> ceil(11/4) = 3 tokens
	got := roughTokenCount("Hello world")
	want := int(math.Ceil(11.0 / 4.0))
	if got != want {
		t.Fatalf("expected %d, got %d", want, got)
	}
}
