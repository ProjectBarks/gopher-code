package main

import (
	"testing"

	"github.com/projectbarks/gopher-code/pkg/billing"
	"github.com/projectbarks/gopher-code/pkg/provider"
	"github.com/projectbarks/gopher-code/pkg/ui/hooks/notifications"
)

// TestBillingUtilization_IntegrationThroughNotifications verifies that the
// billing.Utilization type flows through the notification system via
// RateLimitStateFromUtilization, which is the production code path that
// connects billing data to the UI notification pipeline.
func TestBillingUtilization_IntegrationThroughNotifications(t *testing.T) {
	pct := 92.0
	resetAt := "2026-01-15T12:00:00Z"

	u := &billing.Utilization{
		FiveHour: &billing.RateLimit{
			Utilization: &pct,
			ResetsAt:    &resetAt,
		},
	}

	state := notifications.RateLimitStateFromUtilization(u, "", true)

	// 92% should trigger the approaching-limit warning
	if state.WarningText == "" {
		t.Fatal("expected warning text for 92% utilization")
	}
	if state.IsUsingOverage {
		t.Error("should not be overage with just high utilization")
	}
}

// TestBillingExtraUsageOverage_IntegrationThroughNotifications verifies that
// extra_usage overage from billing flows through to rate-limit notifications.
func TestBillingExtraUsageOverage_IntegrationThroughNotifications(t *testing.T) {
	usedCredits := 75.0
	monthlyLimit := 100.0
	overagePct := 75.0

	u := &billing.Utilization{
		ExtraUsage: &billing.ExtraUsage{
			IsEnabled:    true,
			UsedCredits:  &usedCredits,
			MonthlyLimit: &monthlyLimit,
			Utilization:  &overagePct,
		},
	}

	state := notifications.RateLimitStateFromUtilization(u, "team", true)

	if !state.IsUsingOverage {
		t.Fatal("expected overage when extra_usage has utilization > 0")
	}
	if state.SubscriptionType != "team" {
		t.Errorf("subscription type = %q, want team", state.SubscriptionType)
	}

	// Feed the state through the full notification pipeline
	mgr := notifications.NewManager()
	mgr.RunStartupChecks(notifications.StartupOptions{
		RateLimit: state,
	})

	notifs := mgr.Notifications()
	found := false
	for _, n := range notifs {
		if n.Key == "limit-reached" {
			found = true
			if n.Priority != notifications.PriorityImmediate {
				t.Errorf("overage notification priority = %d, want PriorityImmediate", n.Priority)
			}
		}
	}
	if !found {
		t.Error("expected limit-reached notification from overage state")
	}
}

// TestBillingNilUtilization_ProducesEmptyState verifies nil utilization
// produces a safe, empty RateLimitState.
func TestBillingNilUtilization_ProducesEmptyState(t *testing.T) {
	state := notifications.RateLimitStateFromUtilization(nil, "enterprise", false)

	if state.IsUsingOverage {
		t.Error("nil utilization should not produce overage")
	}
	if state.WarningText != "" {
		t.Error("nil utilization should not produce warning text")
	}
	if state.SubscriptionType != "enterprise" {
		t.Errorf("subscription type = %q, want enterprise", state.SubscriptionType)
	}
}

// TestBillingMetricsEndpoints_Constants verifies billing endpoint constants
// are accessible from the binary.
func TestBillingMetricsEndpoints_Constants(t *testing.T) {
	if billing.MetricsEndpoint == "" {
		t.Error("MetricsEndpoint should not be empty")
	}
	if billing.UsageEndpoint == "" {
		t.Error("UsageEndpoint should not be empty")
	}
	if billing.UsageTimeoutMs != 5000 {
		t.Errorf("UsageTimeoutMs = %d, want 5000", billing.UsageTimeoutMs)
	}
}

// TestBillingMetricsStatus_FailOpen verifies the fail-open behavior.
func TestBillingMetricsStatus_FailOpen(t *testing.T) {
	ms := billing.MetricsEnabledFailOpen()
	if ms.Enabled {
		t.Error("fail-open should have Enabled=false")
	}
	if !ms.HasError {
		t.Error("fail-open should have HasError=true")
	}
}

// TestProviderCostCalculation_IntegrationThroughBinary verifies the provider
// cost calculation utilities are compiled into the binary and produce correct
// results through the real code path.
func TestProviderCostCalculation_IntegrationThroughBinary(t *testing.T) {
	// Verify cost calculation for a known model
	cost := provider.CalculateUSDCost("claude-sonnet-4-6", provider.TokenUsage{
		InputTokens:  1_000_000,
		OutputTokens: 500_000,
	})
	// Sonnet: $3/Mtok input + $15/Mtok output → $3 + $7.5 = $10.5
	if cost < 10.4 || cost > 10.6 {
		t.Errorf("cost = %f, want ~10.5", cost)
	}

	formatted := provider.FormatCost(cost)
	if formatted != "$10.50" {
		t.Errorf("FormatCost(%f) = %q, want $10.50", cost, formatted)
	}
}

// TestProviderRetryBudget_IntegrationThroughBinary verifies the retry budget
// system is compiled into the binary and enforces correct retry policies.
func TestProviderRetryBudget_IntegrationThroughBinary(t *testing.T) {
	budget := provider.NewRetryBudget(provider.RetryBudgetConfig{
		MaxRetries:        3,
		Max529Consecutive: 2,
		QuerySource:       provider.QuerySourceREPLMainThread,
		Model:             "claude-sonnet-4-6",
	})

	// First attempt succeeds
	attempt := budget.RecordAttempt()
	if attempt != 1 {
		t.Errorf("first attempt = %d, want 1", attempt)
	}

	// Should be able to retry after a retryable error
	retryableErr := &provider.APIError{
		StatusCode: 429,
		Type:       provider.ErrRateLimit,
		Retryable:  true,
	}
	if !budget.CanRetry(retryableErr) {
		t.Error("should be able to retry 429 error")
	}
}

// TestProviderEffort_IntegrationThroughBinary verifies effort utilities are
// compiled into the binary.
func TestProviderEffort_IntegrationThroughBinary(t *testing.T) {
	level, ok := provider.ParseEffortValue("medium")
	if !ok {
		t.Fatal("should parse 'medium'")
	}
	if level != provider.EffortMedium {
		t.Errorf("level = %q, want medium", level)
	}

	desc := provider.GetEffortLevelDescription(provider.EffortHigh)
	if desc == "" {
		t.Error("effort level description should not be empty")
	}
}

// TestProviderBetas_IntegrationThroughBinary verifies beta header utilities are
// compiled into the binary.
func TestProviderBetas_IntegrationThroughBinary(t *testing.T) {
	if provider.BetaClaudeCode == "" {
		t.Error("BetaClaudeCode should not be empty")
	}
	if provider.BetaInterleavedThinking == "" {
		t.Error("BetaInterleavedThinking should not be empty")
	}
}

// TestProviderErrorClassification_IntegrationThroughBinary verifies the error
// classification and user-facing message utilities.
func TestProviderErrorClassification_IntegrationThroughBinary(t *testing.T) {
	apiErr := provider.ClassifyHTTPError(429, []byte("rate limited"), "5")
	if apiErr.Type != provider.ErrRateLimit {
		t.Errorf("type = %q, want rate_limit", apiErr.Type)
	}
	if !apiErr.Retryable {
		t.Error("429 should be retryable")
	}

	// Test FormatAPIErrorForUser
	msg := provider.FormatAPIErrorForUser(apiErr)
	if msg == "" {
		t.Error("FormatAPIErrorForUser should not return empty string")
	}
}

// TestProviderAPILimits_IntegrationThroughBinary verifies API limit constants
// are compiled into the binary.
func TestProviderAPILimits_IntegrationThroughBinary(t *testing.T) {
	if provider.APIImageMaxBase64Size != 5*1024*1024 {
		t.Errorf("APIImageMaxBase64Size = %d", provider.APIImageMaxBase64Size)
	}
	if provider.APIPDFMaxPages != 100 {
		t.Errorf("APIPDFMaxPages = %d", provider.APIPDFMaxPages)
	}
	if provider.APIMaxMediaPerRequest != 100 {
		t.Errorf("APIMaxMediaPerRequest = %d", provider.APIMaxMediaPerRequest)
	}
}

// TestProviderEmptyUsage_IntegrationThroughBinary verifies the empty usage
// defaults are accessible.
func TestProviderEmptyUsage_IntegrationThroughBinary(t *testing.T) {
	empty := provider.EmptyUsage()
	if empty.ServiceTier != "standard" {
		t.Errorf("ServiceTier = %q, want standard", empty.ServiceTier)
	}
	if empty.InputTokens != 0 {
		t.Errorf("InputTokens = %d, want 0", empty.InputTokens)
	}
}

// TestProviderQuerySource_IntegrationThroughBinary verifies query source
// classification is accessible.
func TestProviderQuerySource_IntegrationThroughBinary(t *testing.T) {
	if !provider.IsAgenticQuerySource(provider.QuerySourceREPLMainThread) {
		t.Error("REPL main thread should be agentic")
	}
	if provider.IsAgenticQuerySource(provider.QuerySourceCompact) {
		t.Error("compact should not be agentic")
	}
}

// TestProviderLogging_TypesAccessible verifies logging event types are
// compiled into the binary.
func TestProviderLogging_TypesAccessible(t *testing.T) {
	var _ provider.APIQueryEvent
	var _ provider.APIErrorEvent
	var _ provider.APISuccessEvent
	// Just verify the types compile and are accessible
}
