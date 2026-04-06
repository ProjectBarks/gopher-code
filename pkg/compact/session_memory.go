package compact

// Source: services/compact/sessionMemoryCompact.ts

import (
	"sync"

	"github.com/projectbarks/gopher-code/pkg/message"
)

// SessionMemoryCompactConfig holds thresholds for session-memory-based
// compaction. Instead of calling the LLM to summarize, this strategy
// replaces conversation head with pre-computed session memory content.
// Source: sessionMemoryCompact.ts:47-54
type SessionMemoryCompactConfig struct {
	// MinTokens is the minimum tokens to preserve after compaction.
	MinTokens int
	// MinTextBlockMessages is the minimum number of messages with text
	// blocks to keep in the tail.
	MinTextBlockMessages int
	// MaxTokens is the hard cap on tokens to preserve.
	MaxTokens int
}

// DefaultSessionMemoryCompactConfig is the production default.
// Source: sessionMemoryCompact.ts:57-61
var DefaultSessionMemoryCompactConfig = SessionMemoryCompactConfig{
	MinTokens:            10_000,
	MinTextBlockMessages: 5,
	MaxTokens:            40_000,
}

// SessionMemoryCompactState holds mutable config + initialization tracking.
// Source: sessionMemoryCompact.ts:64-69
type SessionMemoryCompactState struct {
	mu                sync.Mutex
	config            SessionMemoryCompactConfig
	configInitialized bool
}

// NewSessionMemoryCompactState creates state with default config.
func NewSessionMemoryCompactState() *SessionMemoryCompactState {
	return &SessionMemoryCompactState{
		config: DefaultSessionMemoryCompactConfig,
	}
}

// SetConfig merges partial config updates.
// Source: sessionMemoryCompact.ts:74-81
func (s *SessionMemoryCompactState) SetConfig(cfg SessionMemoryCompactConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if cfg.MinTokens > 0 {
		s.config.MinTokens = cfg.MinTokens
	}
	if cfg.MinTextBlockMessages > 0 {
		s.config.MinTextBlockMessages = cfg.MinTextBlockMessages
	}
	if cfg.MaxTokens > 0 {
		s.config.MaxTokens = cfg.MaxTokens
	}
}

// GetConfig returns a copy of the current config.
// Source: sessionMemoryCompact.ts:86-88
func (s *SessionMemoryCompactState) GetConfig() SessionMemoryCompactConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.config
}

// Reset restores default config and clears initialization flag.
// Source: sessionMemoryCompact.ts:93-96
func (s *SessionMemoryCompactState) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = DefaultSessionMemoryCompactConfig
	s.configInitialized = false
}

// CalculateMessagesToKeepIndex computes the starting index for messages
// to keep after session-memory compaction. Starts from lastSummarizedIndex,
// then expands backwards to meet minimums (minTokens, minTextBlockMessages).
// Stops expanding if maxTokens is reached. Adjusts for tool_use/tool_result
// pair integrity via AdjustIndexToPreserveAPIInvariants.
// Source: sessionMemoryCompact.ts:324-397
func (s *SessionMemoryCompactState) CalculateMessagesToKeepIndex(
	messages []message.Message,
	lastSummarizedIndex int,
) int {
	if len(messages) == 0 {
		return 0
	}

	cfg := s.GetConfig()

	// Start from the message after lastSummarizedIndex.
	// If lastSummarizedIndex is -1 (not found) or messages.length,
	// we start with no messages kept.
	startIndex := len(messages)
	if lastSummarizedIndex >= 0 {
		startIndex = lastSummarizedIndex + 1
	}

	// Calculate current tokens and text-block message count.
	totalTokens := 0
	textBlockCount := 0
	for i := startIndex; i < len(messages); i++ {
		totalTokens += EstimateMessageTokens([]message.Message{messages[i]})
		if HasTextBlocks(messages[i]) {
			textBlockCount++
		}
	}

	// Already at max cap.
	if totalTokens >= cfg.MaxTokens {
		return AdjustIndexToPreserveAPIInvariants(messages, startIndex)
	}

	// Already meets both minimums.
	if totalTokens >= cfg.MinTokens && textBlockCount >= cfg.MinTextBlockMessages {
		return AdjustIndexToPreserveAPIInvariants(messages, startIndex)
	}

	// Expand backwards until we meet both minimums or hit max cap.
	// Floor at 0 (simplified — TS also floors at last compact boundary).
	for i := startIndex - 1; i >= 0; i-- {
		msgTokens := EstimateMessageTokens([]message.Message{messages[i]})
		totalTokens += msgTokens
		if HasTextBlocks(messages[i]) {
			textBlockCount++
		}
		startIndex = i

		if totalTokens >= cfg.MaxTokens {
			break
		}
		if totalTokens >= cfg.MinTokens && textBlockCount >= cfg.MinTextBlockMessages {
			break
		}
	}

	return AdjustIndexToPreserveAPIInvariants(messages, startIndex)
}

// ShouldUseSessionMemoryCompaction checks feature flags and env vars to
// determine if session-memory compaction should be attempted.
// In Go, this checks an enable/disable flag pair rather than GrowthBook.
// Source: sessionMemoryCompact.ts:403-432
func ShouldUseSessionMemoryCompaction(enableOverride, disableOverride bool, sessionMemoryEnabled, smCompactEnabled bool) bool {
	if enableOverride {
		return true
	}
	if disableOverride {
		return false
	}
	return sessionMemoryEnabled && smCompactEnabled
}
