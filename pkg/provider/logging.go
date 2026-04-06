package provider

import (
	"log/slog"
	"time"
)

// Source: services/api/logging.ts

// GlobalCacheStrategy indicates the prompt caching strategy used for a request.
// Source: logging.ts:47 — 3-value enum
type GlobalCacheStrategy string

const (
	CacheStrategyToolBased    GlobalCacheStrategy = "tool_based"
	CacheStrategySystemPrompt GlobalCacheStrategy = "system_prompt"
	CacheStrategyNone         GlobalCacheStrategy = "none"
)

// APIQueryEvent holds metadata logged at the start of an API request.
// Source: logging.ts:171-233 — logAPIQuery params
type APIQueryEvent struct {
	Model          string
	MessagesLength int
	Temperature    float64
	Betas          []string
	PermissionMode string
	QuerySource    string
	QueryChainID   string
	QueryDepth     int
	ThinkingType   string // "adaptive", "enabled", "disabled"
	EffortValue    string
	FastMode       bool
}

// APIErrorEvent holds metadata logged when an API request fails.
// Source: logging.ts:235-396 — logAPIError params
type APIErrorEvent struct {
	Error                       string
	ErrorType                   APIErrorType
	Model                       string
	MessageCount                int
	MessageTokens               int
	DurationMs                  int64
	DurationMsIncludingRetries  int64
	Attempt                     int
	RequestID                   string
	ClientRequestID             string
	DidFallBackToNonStreaming    bool
	QuerySource                 string
	FastMode                    bool
}

// APISuccessEvent holds metadata logged when an API request succeeds.
// Source: logging.ts:398-577 — logAPISuccess params
type APISuccessEvent struct {
	Model                       string
	PreNormalizedModel          string
	MessageCount                int
	MessageTokens               int
	Usage                       NonNullableUsage
	DurationMs                  int64
	DurationMsIncludingRetries  int64
	Attempt                     int
	TTFTMs                      *int64 // time to first token, nil if unknown
	RequestID                   string
	StopReason                  *StopReason
	CostUSD                     float64
	DidFallBackToNonStreaming    bool
	QuerySource                 string
	GlobalCacheStrategy         GlobalCacheStrategy
	FastMode                    bool
}

// LogAPIQuery logs a structured event at the start of an API request.
// Source: logging.ts:171-233 — tengu_api_query event
func LogAPIQuery(evt APIQueryEvent) {
	attrs := []any{
		slog.String("event", "tengu_api_query"),
		slog.String("model", evt.Model),
		slog.Int("messages_length", evt.MessagesLength),
		slog.Float64("temperature", evt.Temperature),
		slog.String("query_source", evt.QuerySource),
		slog.Bool("fast_mode", evt.FastMode),
	}
	if evt.ThinkingType != "" {
		attrs = append(attrs, slog.String("thinking_type", evt.ThinkingType))
	}
	if evt.EffortValue != "" {
		attrs = append(attrs, slog.String("effort_value", evt.EffortValue))
	}
	if evt.PermissionMode != "" {
		attrs = append(attrs, slog.String("permission_mode", evt.PermissionMode))
	}
	if evt.QueryChainID != "" {
		attrs = append(attrs, slog.String("query_chain_id", evt.QueryChainID))
		attrs = append(attrs, slog.Int("query_depth", evt.QueryDepth))
	}
	slog.Debug("api query", attrs...)
}

// LogAPIError logs a structured event when an API request fails.
// Source: logging.ts:235-396 — tengu_api_error event
func LogAPIError(evt APIErrorEvent) {
	attrs := []any{
		slog.String("event", "tengu_api_error"),
		slog.String("model", evt.Model),
		slog.String("error", evt.Error),
		slog.String("error_type", string(evt.ErrorType)),
		slog.Int("message_count", evt.MessageCount),
		slog.Int64("duration_ms", evt.DurationMs),
		slog.Int("attempt", evt.Attempt),
		slog.Bool("fast_mode", evt.FastMode),
	}
	if evt.RequestID != "" {
		attrs = append(attrs, slog.String("request_id", evt.RequestID))
	}
	if evt.ClientRequestID != "" {
		attrs = append(attrs, slog.String("client_request_id", evt.ClientRequestID))
	}
	slog.Debug("api error", attrs...)
}

// LogAPISuccess logs a structured event when an API request succeeds.
// Source: logging.ts:398-577 — tengu_api_success event
func LogAPISuccess(evt APISuccessEvent) {
	attrs := []any{
		slog.String("event", "tengu_api_success"),
		slog.String("model", evt.Model),
		slog.Int("message_count", evt.MessageCount),
		slog.Int("input_tokens", evt.Usage.InputTokens),
		slog.Int("output_tokens", evt.Usage.OutputTokens),
		slog.Int("cached_input_tokens", evt.Usage.CacheReadInputTokens),
		slog.Int("uncached_input_tokens", evt.Usage.CacheCreationInputTokens),
		slog.Int64("duration_ms", evt.DurationMs),
		slog.Int("attempt", evt.Attempt),
		slog.Float64("cost_usd", evt.CostUSD),
		slog.String("query_source", evt.QuerySource),
		slog.Bool("fast_mode", evt.FastMode),
	}
	if evt.TTFTMs != nil {
		attrs = append(attrs, slog.Int64("ttft_ms", *evt.TTFTMs))
	}
	if evt.RequestID != "" {
		attrs = append(attrs, slog.String("request_id", evt.RequestID))
	}
	if evt.StopReason != nil {
		attrs = append(attrs, slog.String("stop_reason", string(*evt.StopReason)))
	}
	if evt.GlobalCacheStrategy != "" {
		attrs = append(attrs, slog.String("global_cache_strategy", string(evt.GlobalCacheStrategy)))
	}
	slog.Debug("api success", attrs...)
}

// ComputeDurationMs returns the elapsed milliseconds since start.
func ComputeDurationMs(start time.Time) int64 {
	return time.Since(start).Milliseconds()
}
