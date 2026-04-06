package billing

// Source: services/api/usage.ts, services/api/metricsOptOut.ts

// RateLimit represents a single rate-limit dimension.
// Source: usage.ts — RateLimit type
type RateLimit struct {
	Utilization *float64 `json:"utilization"` // percentage 0–100, nil if unknown
	ResetsAt    *string  `json:"resets_at"`   // ISO 8601 timestamp, nil if unknown
}

// ExtraUsage represents overage/extra-credit usage information.
// Source: usage.ts — ExtraUsage type
type ExtraUsage struct {
	IsEnabled   bool     `json:"is_enabled"`
	MonthlyLimit *float64 `json:"monthly_limit"`
	UsedCredits  *float64 `json:"used_credits"`
	Utilization  *float64 `json:"utilization"`
}

// Utilization holds all rate-limit dimensions returned by /api/oauth/usage.
// Source: usage.ts — Utilization type (6 dimensions)
type Utilization struct {
	FiveHour         *RateLimit  `json:"five_hour,omitempty"`
	SevenDay         *RateLimit  `json:"seven_day,omitempty"`
	SevenDayOAuthApps *RateLimit `json:"seven_day_oauth_apps,omitempty"`
	SevenDayOpus     *RateLimit  `json:"seven_day_opus,omitempty"`
	SevenDaySonnet   *RateLimit  `json:"seven_day_sonnet,omitempty"`
	ExtraUsage       *ExtraUsage `json:"extra_usage,omitempty"`
}

// MetricsStatus represents the org-level metrics opt-out state.
// Source: metricsOptOut.ts — MetricsStatus type
type MetricsStatus struct {
	Enabled  bool `json:"enabled"`
	HasError bool `json:"has_error"`
}

// MetricsEnabledFailOpen returns a MetricsStatus that fails open (enabled)
// when the metrics endpoint cannot be reached.
// Source: metricsOptOut.ts — fail-open behavior on API error
func MetricsEnabledFailOpen() MetricsStatus {
	return MetricsStatus{Enabled: false, HasError: true}
}

// MetricsDisabledNoScope returns a MetricsStatus for sessions lacking
// profile scope (service-key OAuth). Not persisted to disk cache.
// Source: metricsOptOut.ts:133-134
func MetricsDisabledNoScope() MetricsStatus {
	return MetricsStatus{Enabled: false, HasError: false}
}

// MetricsEndpoint is the API path for checking org-level metrics opt-out.
// Source: metricsOptOut.ts:46
const MetricsEndpoint = "/api/claude_code/organizations/metrics_enabled"

// UsageEndpoint is the API path for fetching subscriber rate-limit utilization.
// Source: usage.ts:55
const UsageEndpoint = "/api/oauth/usage"

// UsageTimeoutMs is the timeout for the usage endpoint.
// Source: usage.ts:59
const UsageTimeoutMs = 5000

// MetricsCacheTTLMs is the in-memory cache TTL for metrics status.
// Source: metricsOptOut.ts:22
const MetricsCacheTTLMs = 60 * 60 * 1000 // 1 hour

// MetricsDiskCacheTTLMs is the disk cache TTL for metrics status.
// Source: metricsOptOut.ts:27
const MetricsDiskCacheTTLMs = 24 * 60 * 60 * 1000 // 24 hours
