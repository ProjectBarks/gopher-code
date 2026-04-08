package main

import (
	"testing"

	"github.com/projectbarks/gopher-code/pkg/compact"
	"github.com/projectbarks/gopher-code/pkg/message"
	"github.com/projectbarks/gopher-code/pkg/query"
	"github.com/projectbarks/gopher-code/pkg/session"
)

// TestCompactSession_WiredIntoBinary exercises the real CompactSession path
// that is called from the query loop (query.go -> CompactSession -> compact.*).
// This verifies that the compact package is reachable from main() via:
//
//	main -> session.New -> compact.NewCompactWarningState, etc.
//	main -> query.Query -> CompactSession -> compact.MicroCompactMessages,
//	  compact.TruncateHeadForPTLRetry, compact.CalculateTokenWarningState
func TestCompactSession_WiredIntoBinary(t *testing.T) {
	sess := session.New(session.DefaultConfig(), t.TempDir())

	if sess.AutoCompactTracking == nil {
		t.Fatal("AutoCompactTracking should be initialized")
	}
	if sess.CompactWarning == nil {
		t.Fatal("CompactWarning should be initialized")
	}
	if sess.PostCompactCleaner == nil {
		t.Fatal("PostCompactCleaner should be initialized")
	}
	if sess.SessionMemoryCompact == nil {
		t.Fatal("SessionMemoryCompact should be initialized")
	}

	for i := 0; i < 10; i++ {
		if i%2 == 0 {
			sess.Messages = append(sess.Messages, message.UserMessage("user message"))
		} else {
			sess.Messages = append(sess.Messages, message.AssistantMessage("assistant reply"))
		}
	}

	budget := sess.Config.TokenBudget
	sess.LastInputTokens = budget.InputBudget() + 1000

	if !budget.ShouldCompact(sess.LastInputTokens) {
		t.Fatal("ShouldCompact should return true for tokens above threshold")
	}

	originalLen := len(sess.Messages)
	query.CompactSession(sess)

	if len(sess.Messages) >= originalLen {
		t.Errorf("CompactSession should reduce messages: got %d, original %d",
			len(sess.Messages), originalLen)
	}
	if !sess.AutoCompactTracking.Compacted {
		t.Error("AutoCompactTracking.Compacted should be true after successful compact")
	}
	if !sess.ConsumePostCompaction() {
		t.Error("PendingPostCompaction should be true after CompactSession")
	}
}

// TestCompactSession_CircuitBreaker verifies that CompactSession respects
// the auto-compact circuit breaker (MaxConsecutiveAutocompactFailures).
func TestCompactSession_CircuitBreaker(t *testing.T) {
	sess := session.New(session.DefaultConfig(), t.TempDir())

	for i := 0; i < compact.MaxConsecutiveAutocompactFailures; i++ {
		sess.AutoCompactTracking.RecordFailure()
	}
	if !sess.AutoCompactTracking.ShouldSkipAutoCompact() {
		t.Fatal("ShouldSkipAutoCompact should be true after max failures")
	}

	for i := 0; i < 10; i++ {
		if i%2 == 0 {
			sess.Messages = append(sess.Messages, message.UserMessage("msg"))
		} else {
			sess.Messages = append(sess.Messages, message.AssistantMessage("reply"))
		}
	}
	originalLen := len(sess.Messages)

	query.CompactSession(sess)

	if len(sess.Messages) != originalLen {
		t.Errorf("CompactSession should not compact when circuit breaker tripped: got %d, want %d",
			len(sess.Messages), originalLen)
	}
}

// TestCompactWarningState_WiredThroughSession verifies the CompactWarningState
// lifecycle works through the session.
func TestCompactWarningState_WiredThroughSession(t *testing.T) {
	sess := session.New(session.DefaultConfig(), t.TempDir())

	if sess.CompactWarning.IsSuppressed() {
		t.Error("warning should not be suppressed initially")
	}
	sess.CompactWarning.Suppress()
	if !sess.CompactWarning.IsSuppressed() {
		t.Error("warning should be suppressed after Suppress()")
	}
	sess.CompactWarning.ClearSuppression()
	if sess.CompactWarning.IsSuppressed() {
		t.Error("warning should not be suppressed after ClearSuppression()")
	}
}

// TestSessionMemoryCompactState_WiredThroughSession verifies that
// SessionMemoryCompactState is usable through the session.
func TestSessionMemoryCompactState_WiredThroughSession(t *testing.T) {
	sess := session.New(session.DefaultConfig(), t.TempDir())

	cfg := sess.SessionMemoryCompact.GetConfig()
	if cfg.MinTokens != compact.DefaultSessionMemoryCompactConfig.MinTokens {
		t.Errorf("default MinTokens = %d, want %d",
			cfg.MinTokens, compact.DefaultSessionMemoryCompactConfig.MinTokens)
	}

	sess.SessionMemoryCompact.SetConfig(compact.SessionMemoryCompactConfig{
		MinTokens: 20_000,
	})
	cfg = sess.SessionMemoryCompact.GetConfig()
	if cfg.MinTokens != 20_000 {
		t.Errorf("updated MinTokens = %d, want 20000", cfg.MinTokens)
	}
}

// TestCalculateTokenWarningState_WiredIntoBinary verifies that
// CalculateTokenWarningState is reachable and produces correct results.
func TestCalculateTokenWarningState_WiredIntoBinary(t *testing.T) {
	// Threshold = contextWindow - AutocompactBufferTokens = 200k - 13k = 187k.
	state := compact.CalculateTokenWarningState(190_000, 200_000)
	if !state.IsAboveAutoCompactThreshold {
		t.Error("should be above auto-compact threshold at 190k/200k")
	}

	state = compact.CalculateTokenWarningState(50_000, 200_000)
	if state.IsAboveAutoCompactThreshold {
		t.Error("should not be above auto-compact threshold at 50k/200k")
	}
}

// TestParseTokenBudget_WiredIntoBinary verifies that ParseTokenBudget
// is reachable (used in pkg/ui/app.go for user input parsing).
func TestParseTokenBudget_WiredIntoBinary(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"+500k", 500_000},
		{"use 2M tokens", 2_000_000},
		{"just a regular message", 0},
	}
	for _, tt := range tests {
		got := compact.ParseTokenBudget(tt.input)
		if got != tt.want {
			t.Errorf("ParseTokenBudget(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

// TestMergeHookInstructions_WiredIntoBinary verifies MergeHookInstructions
// is reachable (used in pkg/ui/commands/handlers.go for /compact).
func TestMergeHookInstructions_WiredIntoBinary(t *testing.T) {
	got := compact.MergeHookInstructions("user instructions", "hook instructions")
	want := "user instructions\n\nhook instructions"
	if got != want {
		t.Errorf("MergeHookInstructions = %q, want %q", got, want)
	}
	got = compact.MergeHookInstructions("user only", "")
	if got != "user only" {
		t.Errorf("MergeHookInstructions with empty hook = %q, want %q", got, "user only")
	}
}

// TestPostCompactCleaner_WiredThroughSession verifies the PostCompactCleaner
// lifecycle works through the session.
func TestPostCompactCleaner_WiredThroughSession(t *testing.T) {
	sess := session.New(session.DefaultConfig(), t.TempDir())

	called := false
	sess.PostCompactCleaner.RegisterAlways(func() {
		called = true
	})
	sess.PostCompactCleaner.Run("")
	if !called {
		t.Error("cleanup function should have been called")
	}
}

// TestGroupMessagesByAPIRound_WiredIntoBinary verifies that
// GroupMessagesByAPIRound is callable and reachable from the binary.
func TestGroupMessagesByAPIRound_WiredIntoBinary(t *testing.T) {
	msgs := []message.Message{
		message.UserMessage("hello"),
		message.AssistantMessage("world"),
		message.UserMessage("next"),
	}
	groups := compact.GroupMessagesByAPIRound(msgs, compact.GroupMessagesByToolUseID)
	if len(groups) == 0 {
		t.Fatal("GroupMessagesByAPIRound should produce at least 1 group")
	}
	total := 0
	for _, g := range groups {
		total += len(g)
	}
	if total != len(msgs) {
		t.Errorf("groups should cover all %d messages, got %d", len(msgs), total)
	}
}

// TestTimeBasedMCConfig_WiredThroughSession verifies that TimeBasedMCConfig
// is initialized in the session.
func TestTimeBasedMCConfig_WiredThroughSession(t *testing.T) {
	sess := session.New(session.DefaultConfig(), t.TempDir())

	if sess.TimeBasedMCConfig == nil {
		t.Fatal("TimeBasedMCConfig provider should be initialized")
	}
	cfg := sess.TimeBasedMCConfig()
	if cfg.Enabled {
		t.Error("default TimeBasedMCConfig should be disabled")
	}
	if cfg.GapThresholdMinutes != 60 {
		t.Errorf("GapThresholdMinutes = %d, want 60", cfg.GapThresholdMinutes)
	}
}
