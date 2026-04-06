package provider

// Source: services/api/emptyUsage.ts

// ServerToolUse tracks server-side tool use counts within a usage response.
// Source: emptyUsage.ts:13
type ServerToolUse struct {
	WebSearchRequests int `json:"web_search_requests"`
	WebFetchRequests  int `json:"web_fetch_requests"`
}

// CacheCreation holds per-tier cache creation token counts.
// Source: emptyUsage.ts:16-17
type CacheCreation struct {
	Ephemeral1hInputTokens int `json:"ephemeral_1h_input_tokens"`
	Ephemeral5mInputTokens int `json:"ephemeral_5m_input_tokens"`
}

// NonNullableUsage is the full usage struct with all fields non-nil.
// Source: emptyUsage.ts:8-22 (NonNullableUsage shape)
type NonNullableUsage struct {
	InputTokens              int            `json:"input_tokens"`
	OutputTokens             int            `json:"output_tokens"`
	CacheCreationInputTokens int            `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int            `json:"cache_read_input_tokens"`
	ServerToolUse            ServerToolUse  `json:"server_tool_use"`
	ServiceTier              string         `json:"service_tier"`
	CacheCreation            CacheCreation  `json:"cache_creation"`
	InferenceGeo             string         `json:"inference_geo"`
	Speed                    string         `json:"speed"`
}

// EmptyUsage returns a zero-initialized NonNullableUsage.
// Source: emptyUsage.ts:8-22 — EMPTY_USAGE constant
func EmptyUsage() NonNullableUsage {
	return NonNullableUsage{
		ServiceTier: "standard",
		Speed:       "standard",
	}
}

// UsageFromResponse converts a basic Usage (from API response) into a
// NonNullableUsage with nil-pointer fields defaulted to zero.
func UsageFromResponse(u Usage) NonNullableUsage {
	nu := EmptyUsage()
	nu.InputTokens = u.InputTokens
	nu.OutputTokens = u.OutputTokens
	if u.CacheCreationInputTokens != nil {
		nu.CacheCreationInputTokens = *u.CacheCreationInputTokens
	}
	if u.CacheReadInputTokens != nil {
		nu.CacheReadInputTokens = *u.CacheReadInputTokens
	}
	return nu
}
