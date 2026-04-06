package provider

import (
	"encoding/json"
	"testing"
)

// Source: services/api/emptyUsage.ts

func TestEmptyUsage_AllZero(t *testing.T) {
	u := EmptyUsage()

	if u.InputTokens != 0 {
		t.Errorf("InputTokens = %d, want 0", u.InputTokens)
	}
	if u.OutputTokens != 0 {
		t.Errorf("OutputTokens = %d, want 0", u.OutputTokens)
	}
	if u.CacheCreationInputTokens != 0 {
		t.Errorf("CacheCreationInputTokens = %d, want 0", u.CacheCreationInputTokens)
	}
	if u.CacheReadInputTokens != 0 {
		t.Errorf("CacheReadInputTokens = %d, want 0", u.CacheReadInputTokens)
	}
	if u.ServerToolUse.WebSearchRequests != 0 {
		t.Errorf("WebSearchRequests = %d, want 0", u.ServerToolUse.WebSearchRequests)
	}
	if u.ServerToolUse.WebFetchRequests != 0 {
		t.Errorf("WebFetchRequests = %d, want 0", u.ServerToolUse.WebFetchRequests)
	}
	if u.CacheCreation.Ephemeral1hInputTokens != 0 {
		t.Errorf("Ephemeral1hInputTokens = %d, want 0", u.CacheCreation.Ephemeral1hInputTokens)
	}
	if u.CacheCreation.Ephemeral5mInputTokens != 0 {
		t.Errorf("Ephemeral5mInputTokens = %d, want 0", u.CacheCreation.Ephemeral5mInputTokens)
	}
}

func TestEmptyUsage_ServiceTierStandard(t *testing.T) {
	// Source: emptyUsage.ts:19 — service_tier: 'standard'
	u := EmptyUsage()
	if u.ServiceTier != "standard" {
		t.Errorf("ServiceTier = %q, want 'standard'", u.ServiceTier)
	}
}

func TestEmptyUsage_SpeedStandard(t *testing.T) {
	// Source: emptyUsage.ts:22 — speed: 'standard'
	u := EmptyUsage()
	if u.Speed != "standard" {
		t.Errorf("Speed = %q, want 'standard'", u.Speed)
	}
}

func TestEmptyUsage_JSONRoundTrip(t *testing.T) {
	u := EmptyUsage()
	data, err := json.Marshal(u)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded NonNullableUsage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded != u {
		t.Errorf("round-trip mismatch: got %+v, want %+v", decoded, u)
	}
}

func TestUsageFromResponse_NilOptionals(t *testing.T) {
	u := Usage{InputTokens: 100, OutputTokens: 50}
	nu := UsageFromResponse(u)

	if nu.InputTokens != 100 {
		t.Errorf("InputTokens = %d, want 100", nu.InputTokens)
	}
	if nu.OutputTokens != 50 {
		t.Errorf("OutputTokens = %d, want 50", nu.OutputTokens)
	}
	if nu.CacheCreationInputTokens != 0 {
		t.Errorf("CacheCreationInputTokens = %d, want 0 (nil source)", nu.CacheCreationInputTokens)
	}
	if nu.CacheReadInputTokens != 0 {
		t.Errorf("CacheReadInputTokens = %d, want 0 (nil source)", nu.CacheReadInputTokens)
	}
	if nu.ServiceTier != "standard" {
		t.Errorf("ServiceTier = %q, want 'standard'", nu.ServiceTier)
	}
}

func TestUsageFromResponse_WithCacheTokens(t *testing.T) {
	cacheCreate := 200
	cacheRead := 300
	u := Usage{
		InputTokens:              1000,
		OutputTokens:             500,
		CacheCreationInputTokens: &cacheCreate,
		CacheReadInputTokens:     &cacheRead,
	}
	nu := UsageFromResponse(u)

	if nu.CacheCreationInputTokens != 200 {
		t.Errorf("CacheCreationInputTokens = %d, want 200", nu.CacheCreationInputTokens)
	}
	if nu.CacheReadInputTokens != 300 {
		t.Errorf("CacheReadInputTokens = %d, want 300", nu.CacheReadInputTokens)
	}
}
