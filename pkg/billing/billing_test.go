package billing

import (
	"encoding/json"
	"testing"
)

// Source: services/api/usage.ts, services/api/metricsOptOut.ts

func TestUtilizationJSON(t *testing.T) {
	pct := 42.5
	resetTime := "2025-01-15T12:00:00Z"
	u := Utilization{
		FiveHour: &RateLimit{Utilization: &pct, ResetsAt: &resetTime},
		SevenDay: &RateLimit{Utilization: nil, ResetsAt: nil},
	}

	data, err := json.Marshal(u)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded Utilization
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.FiveHour == nil {
		t.Fatal("FiveHour should not be nil after round-trip")
	}
	if decoded.FiveHour.Utilization == nil || *decoded.FiveHour.Utilization != 42.5 {
		t.Errorf("FiveHour.Utilization = %v, want 42.5", decoded.FiveHour.Utilization)
	}
	if decoded.FiveHour.ResetsAt == nil || *decoded.FiveHour.ResetsAt != resetTime {
		t.Errorf("FiveHour.ResetsAt = %v, want %s", decoded.FiveHour.ResetsAt, resetTime)
	}
	if decoded.SevenDay == nil {
		t.Fatal("SevenDay should not be nil after round-trip")
	}
	if decoded.SevenDay.Utilization != nil {
		t.Errorf("SevenDay.Utilization = %v, want nil", decoded.SevenDay.Utilization)
	}
}

func TestUtilizationAllDimensions(t *testing.T) {
	// Source: usage.ts — 6 rate-limit dimensions
	raw := `{
		"five_hour": {"utilization": 10, "resets_at": "2025-01-15T12:00:00Z"},
		"seven_day": {"utilization": 20, "resets_at": null},
		"seven_day_oauth_apps": {"utilization": 30, "resets_at": null},
		"seven_day_opus": {"utilization": 40, "resets_at": null},
		"seven_day_sonnet": {"utilization": 50, "resets_at": null},
		"extra_usage": {"is_enabled": true, "monthly_limit": 100, "used_credits": 25, "utilization": 25}
	}`

	var u Utilization
	if err := json.Unmarshal([]byte(raw), &u); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if u.FiveHour == nil || u.FiveHour.Utilization == nil || *u.FiveHour.Utilization != 10 {
		t.Error("FiveHour dimension missing or wrong")
	}
	if u.SevenDayOpus == nil || u.SevenDayOpus.Utilization == nil || *u.SevenDayOpus.Utilization != 40 {
		t.Error("SevenDayOpus dimension missing or wrong")
	}
	if u.ExtraUsage == nil {
		t.Fatal("ExtraUsage should not be nil")
	}
	if !u.ExtraUsage.IsEnabled {
		t.Error("ExtraUsage.IsEnabled should be true")
	}
	if u.ExtraUsage.MonthlyLimit == nil || *u.ExtraUsage.MonthlyLimit != 100 {
		t.Errorf("ExtraUsage.MonthlyLimit = %v, want 100", u.ExtraUsage.MonthlyLimit)
	}
}

func TestUtilizationOmitEmpty(t *testing.T) {
	u := Utilization{} // all nil
	data, err := json.Marshal(u)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(data) != "{}" {
		t.Errorf("empty Utilization should marshal to {}, got %s", data)
	}
}

func TestMetricsEnabledFailOpen(t *testing.T) {
	// Source: metricsOptOut.ts — fail-open returns enabled:false, hasError:true
	ms := MetricsEnabledFailOpen()
	if ms.Enabled {
		t.Error("fail-open should have Enabled=false")
	}
	if !ms.HasError {
		t.Error("fail-open should have HasError=true")
	}
}

func TestMetricsDisabledNoScope(t *testing.T) {
	// Source: metricsOptOut.ts:133 — no profile scope → disabled, no error
	ms := MetricsDisabledNoScope()
	if ms.Enabled {
		t.Error("no-scope should have Enabled=false")
	}
	if ms.HasError {
		t.Error("no-scope should have HasError=false")
	}
}

func TestEndpointConstants(t *testing.T) {
	// Source: metricsOptOut.ts:46, usage.ts:55
	if MetricsEndpoint != "/api/claude_code/organizations/metrics_enabled" {
		t.Errorf("MetricsEndpoint = %q", MetricsEndpoint)
	}
	if UsageEndpoint != "/api/oauth/usage" {
		t.Errorf("UsageEndpoint = %q", UsageEndpoint)
	}
	if UsageTimeoutMs != 5000 {
		t.Errorf("UsageTimeoutMs = %d, want 5000", UsageTimeoutMs)
	}
}

func TestCacheTTLConstants(t *testing.T) {
	// Source: metricsOptOut.ts:22,27
	if MetricsCacheTTLMs != 3600000 {
		t.Errorf("MetricsCacheTTLMs = %d, want 3600000", MetricsCacheTTLMs)
	}
	if MetricsDiskCacheTTLMs != 86400000 {
		t.Errorf("MetricsDiskCacheTTLMs = %d, want 86400000", MetricsDiskCacheTTLMs)
	}
}

func TestExtraUsageJSON(t *testing.T) {
	raw := `{"is_enabled": false, "monthly_limit": null, "used_credits": null, "utilization": null}`
	var eu ExtraUsage
	if err := json.Unmarshal([]byte(raw), &eu); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if eu.IsEnabled {
		t.Error("IsEnabled should be false")
	}
	if eu.MonthlyLimit != nil {
		t.Error("MonthlyLimit should be nil")
	}
	if eu.UsedCredits != nil {
		t.Error("UsedCredits should be nil")
	}
	if eu.Utilization != nil {
		t.Error("Utilization should be nil")
	}
}
