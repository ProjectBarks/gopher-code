package provider

import (
	"testing"
	"time"
)

// Source: services/api/logging.ts

func TestGlobalCacheStrategy_Values(t *testing.T) {
	// Source: logging.ts:47 — 3 literal values
	if CacheStrategyToolBased != "tool_based" {
		t.Errorf("CacheStrategyToolBased = %q", CacheStrategyToolBased)
	}
	if CacheStrategySystemPrompt != "system_prompt" {
		t.Errorf("CacheStrategySystemPrompt = %q", CacheStrategySystemPrompt)
	}
	if CacheStrategyNone != "none" {
		t.Errorf("CacheStrategyNone = %q", CacheStrategyNone)
	}
}

func TestLogAPIQuery_DoesNotPanic(t *testing.T) {
	// Smoke test: calling LogAPIQuery with minimal data should not panic.
	LogAPIQuery(APIQueryEvent{
		Model:          "claude-sonnet-4-20250514",
		MessagesLength: 5,
		Temperature:    1.0,
		QuerySource:    "user",
	})
}

func TestLogAPIQuery_WithAllFields(t *testing.T) {
	LogAPIQuery(APIQueryEvent{
		Model:          "claude-sonnet-4-20250514",
		MessagesLength: 10,
		Temperature:    0.7,
		Betas:          []string{"beta1", "beta2"},
		PermissionMode: "auto",
		QuerySource:    "tool_use",
		QueryChainID:   "chain-123",
		QueryDepth:     3,
		ThinkingType:   "adaptive",
		EffortValue:    "high",
		FastMode:       true,
	})
}

func TestLogAPIError_DoesNotPanic(t *testing.T) {
	LogAPIError(APIErrorEvent{
		Error:      "rate limit exceeded",
		ErrorType:  ErrRateLimit,
		Model:      "claude-sonnet-4-20250514",
		DurationMs: 1500,
		Attempt:    2,
	})
}

func TestLogAPIError_WithAllFields(t *testing.T) {
	LogAPIError(APIErrorEvent{
		Error:                      "server overloaded",
		ErrorType:                  ErrServerOverload,
		Model:                      "claude-sonnet-4-20250514",
		MessageCount:               5,
		MessageTokens:              1000,
		DurationMs:                 2000,
		DurationMsIncludingRetries: 5000,
		Attempt:                    3,
		RequestID:                  "req-abc",
		ClientRequestID:            "cli-xyz",
		DidFallBackToNonStreaming:   true,
		QuerySource:                "user",
		FastMode:                   false,
	})
}

func TestLogAPISuccess_DoesNotPanic(t *testing.T) {
	ttft := int64(200)
	stop := StopReasonEndTurn
	LogAPISuccess(APISuccessEvent{
		Model:                      "claude-sonnet-4-20250514",
		PreNormalizedModel:         "claude-sonnet-4-20250514",
		MessageCount:               3,
		MessageTokens:              500,
		Usage:                      EmptyUsage(),
		DurationMs:                 1000,
		DurationMsIncludingRetries: 1200,
		Attempt:                    1,
		TTFTMs:                     &ttft,
		RequestID:                  "req-123",
		StopReason:                 &stop,
		CostUSD:                    0.01,
		QuerySource:                "user",
		GlobalCacheStrategy:        CacheStrategyToolBased,
		FastMode:                   false,
	})
}

func TestLogAPISuccess_NilOptionals(t *testing.T) {
	// TTFTMs, StopReason, GlobalCacheStrategy can all be zero/nil
	LogAPISuccess(APISuccessEvent{
		Model:       "claude-sonnet-4-20250514",
		Usage:       EmptyUsage(),
		DurationMs:  500,
		QuerySource: "tool_use",
	})
}

func TestComputeDurationMs(t *testing.T) {
	start := time.Now().Add(-100 * time.Millisecond)
	d := ComputeDurationMs(start)
	if d < 90 || d > 500 {
		t.Errorf("ComputeDurationMs = %d, expected ~100", d)
	}
}

func TestAPIQueryEvent_FieldTypes(t *testing.T) {
	// Verify struct fields exist and have correct types via compilation.
	var evt APIQueryEvent
	evt.Model = "m"
	evt.MessagesLength = 1
	evt.Temperature = 1.0
	evt.Betas = []string{"b"}
	evt.PermissionMode = "auto"
	evt.QuerySource = "user"
	evt.QueryChainID = "c"
	evt.QueryDepth = 1
	evt.ThinkingType = "enabled"
	evt.EffortValue = "high"
	evt.FastMode = true
	_ = evt
}

func TestAPIErrorEvent_FieldTypes(t *testing.T) {
	var evt APIErrorEvent
	evt.Error = "e"
	evt.ErrorType = ErrRateLimit
	evt.Model = "m"
	evt.MessageCount = 1
	evt.MessageTokens = 100
	evt.DurationMs = 1000
	evt.DurationMsIncludingRetries = 2000
	evt.Attempt = 1
	evt.RequestID = "r"
	evt.ClientRequestID = "c"
	evt.DidFallBackToNonStreaming = false
	evt.QuerySource = "user"
	evt.FastMode = false
	_ = evt
}

func TestAPISuccessEvent_FieldTypes(t *testing.T) {
	var evt APISuccessEvent
	evt.Model = "m"
	evt.PreNormalizedModel = "m"
	evt.MessageCount = 1
	evt.MessageTokens = 100
	evt.Usage = EmptyUsage()
	evt.DurationMs = 1000
	evt.DurationMsIncludingRetries = 2000
	evt.Attempt = 1
	ttft := int64(100)
	evt.TTFTMs = &ttft
	evt.RequestID = "r"
	stop := StopReasonEndTurn
	evt.StopReason = &stop
	evt.CostUSD = 0.01
	evt.DidFallBackToNonStreaming = false
	evt.QuerySource = "user"
	evt.GlobalCacheStrategy = CacheStrategyNone
	evt.FastMode = false
	_ = evt
}
