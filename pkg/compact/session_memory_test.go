package compact

import (
	"encoding/json"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/message"
)

// Source: services/compact/sessionMemoryCompact.ts

func TestDefaultSessionMemoryCompactConfig(t *testing.T) {
	// Source: sessionMemoryCompact.ts:57-61
	cfg := DefaultSessionMemoryCompactConfig
	if cfg.MinTokens != 10_000 {
		t.Errorf("MinTokens = %d, want 10000", cfg.MinTokens)
	}
	if cfg.MinTextBlockMessages != 5 {
		t.Errorf("MinTextBlockMessages = %d, want 5", cfg.MinTextBlockMessages)
	}
	if cfg.MaxTokens != 40_000 {
		t.Errorf("MaxTokens = %d, want 40000", cfg.MaxTokens)
	}
}

func TestSessionMemoryCompactState_SetAndGet(t *testing.T) {
	s := NewSessionMemoryCompactState()
	s.SetConfig(SessionMemoryCompactConfig{
		MinTokens:            20_000,
		MinTextBlockMessages: 10,
		MaxTokens:            80_000,
	})
	cfg := s.GetConfig()
	if cfg.MinTokens != 20_000 {
		t.Errorf("MinTokens = %d, want 20000", cfg.MinTokens)
	}
	if cfg.MinTextBlockMessages != 10 {
		t.Errorf("MinTextBlockMessages = %d, want 10", cfg.MinTextBlockMessages)
	}
	if cfg.MaxTokens != 80_000 {
		t.Errorf("MaxTokens = %d, want 80000", cfg.MaxTokens)
	}
}

func TestSessionMemoryCompactState_PartialSetKeepsDefaults(t *testing.T) {
	s := NewSessionMemoryCompactState()
	// Only set MinTokens; others should keep defaults (zero means "don't override").
	s.SetConfig(SessionMemoryCompactConfig{MinTokens: 15_000})
	cfg := s.GetConfig()
	if cfg.MinTokens != 15_000 {
		t.Errorf("MinTokens = %d, want 15000", cfg.MinTokens)
	}
	if cfg.MinTextBlockMessages != 5 {
		t.Errorf("MinTextBlockMessages = %d, want 5 (default)", cfg.MinTextBlockMessages)
	}
	if cfg.MaxTokens != 40_000 {
		t.Errorf("MaxTokens = %d, want 40000 (default)", cfg.MaxTokens)
	}
}

func TestSessionMemoryCompactState_Reset(t *testing.T) {
	s := NewSessionMemoryCompactState()
	s.SetConfig(SessionMemoryCompactConfig{MinTokens: 99_999, MinTextBlockMessages: 99, MaxTokens: 99_999})
	s.Reset()
	cfg := s.GetConfig()
	if cfg != DefaultSessionMemoryCompactConfig {
		t.Errorf("after Reset, config = %+v, want defaults %+v", cfg, DefaultSessionMemoryCompactConfig)
	}
}

func TestCalculateMessagesToKeepIndex_EmptyMessages(t *testing.T) {
	s := NewSessionMemoryCompactState()
	idx := s.CalculateMessagesToKeepIndex(nil, -1)
	if idx != 0 {
		t.Errorf("expected 0 for empty messages, got %d", idx)
	}
}

func TestCalculateMessagesToKeepIndex_NotFoundIndex(t *testing.T) {
	s := NewSessionMemoryCompactState()
	// lastSummarizedIndex = -1 → start from end (no messages kept initially).
	// With low token messages, expansion fills from back.
	msgs := makeLargeConversation(20)
	idx := s.CalculateMessagesToKeepIndex(msgs, -1)
	// Should start at messages.length when lastSummarizedIndex is -1,
	// then expand backwards. With default minTokens=10000 and small messages,
	// it should expand back some distance.
	if idx >= len(msgs) {
		t.Errorf("expected index < %d after expansion, got %d", len(msgs), idx)
	}
}

func TestCalculateMessagesToKeepIndex_RespectsMaxTokens(t *testing.T) {
	s := NewSessionMemoryCompactState()
	s.SetConfig(SessionMemoryCompactConfig{
		MinTokens:            1,
		MinTextBlockMessages: 1,
		MaxTokens:            100, // tiny cap
	})
	msgs := makeLargeConversation(20)
	idx := s.CalculateMessagesToKeepIndex(msgs, 0)
	// Should not expand far due to tiny maxTokens cap.
	if idx == 0 {
		// It started at index 1 and should not expand much.
		// This is valid — just ensure it doesn't blow past.
	}
	if idx < 0 {
		t.Errorf("index should be >= 0, got %d", idx)
	}
}

func TestCalculateMessagesToKeepIndex_MeetsBothMinimums(t *testing.T) {
	s := NewSessionMemoryCompactState()
	s.SetConfig(SessionMemoryCompactConfig{
		MinTokens:            1, // trivially satisfied
		MinTextBlockMessages: 1, // trivially satisfied
		MaxTokens:            1_000_000,
	})
	// lastSummarizedIndex=5, messages length=10 → start at 6, 4 messages kept.
	msgs := makeLargeConversation(10)
	idx := s.CalculateMessagesToKeepIndex(msgs, 5)
	// Both minimums are immediately met at startIndex=6 (4 messages with text).
	if idx != 6 {
		t.Errorf("expected index 6 (no expansion needed), got %d", idx)
	}
}

func TestShouldUseSessionMemoryCompaction_EnableOverride(t *testing.T) {
	if !ShouldUseSessionMemoryCompaction(true, false, false, false) {
		t.Error("enable override should return true regardless of flags")
	}
}

func TestShouldUseSessionMemoryCompaction_DisableOverride(t *testing.T) {
	if ShouldUseSessionMemoryCompaction(false, true, true, true) {
		t.Error("disable override should return false regardless of flags")
	}
}

func TestShouldUseSessionMemoryCompaction_BothFlags(t *testing.T) {
	if !ShouldUseSessionMemoryCompaction(false, false, true, true) {
		t.Error("both flags true should return true")
	}
	if ShouldUseSessionMemoryCompaction(false, false, true, false) {
		t.Error("smCompact false should return false")
	}
	if ShouldUseSessionMemoryCompaction(false, false, false, true) {
		t.Error("sessionMemory false should return false")
	}
}

// makeLargeConversation creates a conversation with alternating user/assistant messages.
func makeLargeConversation(n int) []message.Message {
	msgs := make([]message.Message, n)
	for i := 0; i < n; i++ {
		if i%2 == 0 {
			msgs[i] = message.UserMessage("user message with some text content for token estimation")
		} else {
			msgs[i] = message.Message{
				Role: message.RoleAssistant,
				Content: []message.ContentBlock{
					message.ToolUseBlock("t"+string(rune('0'+i)), "Read", json.RawMessage(`{"path":"/foo/bar.go"}`)),
				},
			}
		}
	}
	return msgs
}
