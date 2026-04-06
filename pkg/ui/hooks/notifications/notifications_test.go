package notifications

import (
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Rate-limit notification tests
// ---------------------------------------------------------------------------

func TestCheckRateLimit_OverageTriggersImmediate(t *testing.T) {
	notifs := CheckRateLimit(RateLimitState{
		IsUsingOverage: true,
		OverageText:    "You have exceeded your Pro plan limit",
	})
	if len(notifs) == 0 {
		t.Fatal("expected overage notification, got none")
	}
	got := notifs[0]
	if got.Key != "limit-reached" {
		t.Errorf("key = %q, want %q", got.Key, "limit-reached")
	}
	if got.Priority != PriorityImmediate {
		t.Errorf("priority = %d, want PriorityImmediate (%d)", got.Priority, PriorityImmediate)
	}
	if got.Message != "You have exceeded your Pro plan limit" {
		t.Errorf("message = %q, want overage text", got.Message)
	}
}

func TestCheckRateLimit_WarningAtThreshold(t *testing.T) {
	notifs := CheckRateLimit(RateLimitState{
		WarningText: "Approaching rate limit for claude-sonnet-4-20250514",
	})
	if len(notifs) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notifs))
	}
	got := notifs[0]
	if got.Key != "rate-limit-warning" {
		t.Errorf("key = %q, want %q", got.Key, "rate-limit-warning")
	}
	if got.Priority != PriorityHigh {
		t.Errorf("priority = %d, want PriorityHigh (%d)", got.Priority, PriorityHigh)
	}
	if got.Color != ColorWarning {
		t.Errorf("color = %q, want %q", got.Color, ColorWarning)
	}
}

func TestCheckRateLimit_TeamWithoutBillingSkipsOverage(t *testing.T) {
	notifs := CheckRateLimit(RateLimitState{
		IsUsingOverage:   true,
		SubscriptionType: "team",
		HasBillingAccess: false,
	})
	// Should NOT include overage notification for team without billing.
	for _, n := range notifs {
		if n.Key == "limit-reached" {
			t.Error("team without billing should not see overage notification")
		}
	}
}

func TestCheckRateLimit_TeamWithBillingShowsOverage(t *testing.T) {
	notifs := CheckRateLimit(RateLimitState{
		IsUsingOverage:   true,
		SubscriptionType: "team",
		HasBillingAccess: true,
		OverageText:      "Team limit exceeded",
	})
	found := false
	for _, n := range notifs {
		if n.Key == "limit-reached" {
			found = true
		}
	}
	if !found {
		t.Error("team with billing should see overage notification")
	}
}

func TestCheckRateLimit_EnterpriseWithoutBillingSkipsOverage(t *testing.T) {
	notifs := CheckRateLimit(RateLimitState{
		IsUsingOverage:   true,
		SubscriptionType: "enterprise",
		HasBillingAccess: false,
	})
	for _, n := range notifs {
		if n.Key == "limit-reached" {
			t.Error("enterprise without billing should not see overage notification")
		}
	}
}

func TestCheckRateLimit_BothOverageAndWarning(t *testing.T) {
	notifs := CheckRateLimit(RateLimitState{
		IsUsingOverage: true,
		OverageText:    "Over limit",
		WarningText:    "Approaching limit",
	})
	if len(notifs) != 2 {
		t.Fatalf("expected 2 notifications, got %d", len(notifs))
	}
	if notifs[0].Key != "limit-reached" {
		t.Errorf("first notification key = %q, want limit-reached", notifs[0].Key)
	}
	if notifs[1].Key != "rate-limit-warning" {
		t.Errorf("second notification key = %q, want rate-limit-warning", notifs[1].Key)
	}
}

func TestCheckRateLimit_NoIssues(t *testing.T) {
	notifs := CheckRateLimit(RateLimitState{})
	if len(notifs) != 0 {
		t.Errorf("expected no notifications, got %d", len(notifs))
	}
}

// ---------------------------------------------------------------------------
// Settings errors tests
// ---------------------------------------------------------------------------

func TestCheckSettingsErrors_SurfacesValidationIssues(t *testing.T) {
	errs := []SettingsError{
		{Path: "model", Message: "must be a string"},
		{Path: "maxTokens", Message: "must be >= 1"},
	}
	n := CheckSettingsErrors(errs)
	if n == nil {
		t.Fatal("expected notification, got nil")
	}
	if n.Key != "settings-errors" {
		t.Errorf("key = %q, want %q", n.Key, "settings-errors")
	}
	if !strings.Contains(n.Message, "2 settings issues") {
		t.Errorf("message = %q, want to contain '2 settings issues'", n.Message)
	}
	if !strings.Contains(n.Message, "/doctor") {
		t.Errorf("message = %q, want to contain '/doctor'", n.Message)
	}
	if n.TimeoutMs != 60_000 {
		t.Errorf("timeoutMs = %d, want 60000", n.TimeoutMs)
	}
	if n.Priority != PriorityHigh {
		t.Errorf("priority = %d, want PriorityHigh", n.Priority)
	}
	if n.Color != ColorWarning {
		t.Errorf("color = %q, want %q", n.Color, ColorWarning)
	}
}

func TestCheckSettingsErrors_SingularIssue(t *testing.T) {
	n := CheckSettingsErrors([]SettingsError{{Path: "model", Message: "bad"}})
	if n == nil {
		t.Fatal("expected notification, got nil")
	}
	if !strings.Contains(n.Message, "1 settings issue") {
		t.Errorf("message = %q, want singular 'issue' not 'issues'", n.Message)
	}
	// Make sure it doesn't say "issues" (plural).
	if strings.Contains(n.Message, "issues") {
		t.Errorf("message = %q, should use singular 'issue'", n.Message)
	}
}

func TestCheckSettingsErrors_NoErrors(t *testing.T) {
	n := CheckSettingsErrors(nil)
	if n != nil {
		t.Errorf("expected nil, got %+v", n)
	}
	n = CheckSettingsErrors([]SettingsError{})
	if n != nil {
		t.Errorf("expected nil for empty slice, got %+v", n)
	}
}

// ---------------------------------------------------------------------------
// Startup notification tests
// ---------------------------------------------------------------------------

func TestCheckStartup_RemoteModeSkips(t *testing.T) {
	called := false
	notifs := CheckStartup(true, func() []Notification {
		called = true
		return []Notification{{Key: "test"}}
	})
	if called {
		t.Error("compute should not be called in remote mode")
	}
	if notifs != nil {
		t.Errorf("expected nil, got %v", notifs)
	}
}

func TestCheckStartup_RunsCompute(t *testing.T) {
	notifs := CheckStartup(false, func() []Notification {
		return []Notification{
			{Key: "welcome", Message: "Welcome to Claude Code"},
		}
	})
	if len(notifs) != 1 {
		t.Fatalf("expected 1, got %d", len(notifs))
	}
	if notifs[0].Key != "welcome" {
		t.Errorf("key = %q, want %q", notifs[0].Key, "welcome")
	}
}

func TestCheckStartup_NilCompute(t *testing.T) {
	notifs := CheckStartup(false, nil)
	if notifs != nil {
		t.Errorf("expected nil, got %v", notifs)
	}
}

// ---------------------------------------------------------------------------
// Fast-mode notification tests
// ---------------------------------------------------------------------------

func TestCheckFastMode_OrgEnabled(t *testing.T) {
	n := CheckFastModeEvent(FastModeEvent{
		Type:       FastModeOrgChanged,
		OrgEnabled: true,
	})
	if n == nil {
		t.Fatal("expected notification")
	}
	if n.Key != "fast-mode-org-changed" {
		t.Errorf("key = %q", n.Key)
	}
	if !strings.Contains(n.Message, "now available") {
		t.Errorf("message = %q, want 'now available'", n.Message)
	}
	if !strings.Contains(n.Message, "/fast") {
		t.Errorf("message = %q, want '/fast'", n.Message)
	}
	if n.Color != ColorFastMode {
		t.Errorf("color = %q, want %q", n.Color, ColorFastMode)
	}
}

func TestCheckFastMode_OrgDisabled(t *testing.T) {
	n := CheckFastModeEvent(FastModeEvent{
		Type:       FastModeOrgChanged,
		OrgEnabled: false,
	})
	if n == nil {
		t.Fatal("expected notification")
	}
	if !strings.Contains(n.Message, "disabled by your organization") {
		t.Errorf("message = %q", n.Message)
	}
	if n.Color != ColorWarning {
		t.Errorf("color = %q, want warning", n.Color)
	}
}

func TestCheckFastMode_CooldownOverloaded(t *testing.T) {
	n := CheckFastModeEvent(FastModeEvent{
		Type:    FastModeCooldownTriggered,
		ResetAt: time.Now().Add(5 * time.Minute),
		Reason:  CooldownOverloaded,
	})
	if n == nil {
		t.Fatal("expected notification")
	}
	if n.Key != "fast-mode-cooldown-started" {
		t.Errorf("key = %q", n.Key)
	}
	if !strings.Contains(n.Message, "overloaded") {
		t.Errorf("message = %q, want 'overloaded'", n.Message)
	}
	if !strings.Contains(n.Message, "resets in") {
		t.Errorf("message = %q, want 'resets in'", n.Message)
	}
}

func TestCheckFastMode_CooldownRateLimit(t *testing.T) {
	n := CheckFastModeEvent(FastModeEvent{
		Type:    FastModeCooldownTriggered,
		ResetAt: time.Now().Add(30 * time.Second),
		Reason:  CooldownRateLimit,
	})
	if n == nil {
		t.Fatal("expected notification")
	}
	if !strings.Contains(n.Message, "Fast limit reached") {
		t.Errorf("message = %q, want 'Fast limit reached'", n.Message)
	}
}

func TestCheckFastMode_CooldownExpired(t *testing.T) {
	n := CheckFastModeEvent(FastModeEvent{
		Type: FastModeCooldownExpired,
	})
	if n == nil {
		t.Fatal("expected notification")
	}
	if n.Key != "fast-mode-cooldown-expired" {
		t.Errorf("key = %q", n.Key)
	}
	if !strings.Contains(n.Message, "Fast limit reset") {
		t.Errorf("message = %q", n.Message)
	}
	if n.Color != ColorFastMode {
		t.Errorf("color = %q, want fastMode", n.Color)
	}
}

func TestCheckFastMode_OverageRejection(t *testing.T) {
	n := CheckFastModeEvent(FastModeEvent{
		Type:    FastModeOverageRejection,
		Message: "Fast mode overage not allowed",
	})
	if n == nil {
		t.Fatal("expected notification")
	}
	if n.Key != "fast-mode-overage-rejected" {
		t.Errorf("key = %q", n.Key)
	}
	if n.Message != "Fast mode overage not allowed" {
		t.Errorf("message = %q", n.Message)
	}
}

// ---------------------------------------------------------------------------
// Model migration tests
// ---------------------------------------------------------------------------

func TestCheckModelMigrations_RecentSonnet(t *testing.T) {
	now := time.Now()
	notifs := CheckModelMigrations(MigrationConfig{
		Sonnet45To46Timestamp: &now,
	})
	if len(notifs) != 1 {
		t.Fatalf("expected 1, got %d", len(notifs))
	}
	if notifs[0].Key != "sonnet-46-update" {
		t.Errorf("key = %q", notifs[0].Key)
	}
	if notifs[0].Message != "Model updated to Sonnet 4.6" {
		t.Errorf("message = %q", notifs[0].Message)
	}
	if notifs[0].TimeoutMs != 3_000 {
		t.Errorf("timeout = %d, want 3000", notifs[0].TimeoutMs)
	}
}

func TestCheckModelMigrations_OldTimestampIgnored(t *testing.T) {
	old := time.Now().Add(-10 * time.Second)
	notifs := CheckModelMigrations(MigrationConfig{
		Sonnet45To46Timestamp: &old,
	})
	if len(notifs) != 0 {
		t.Errorf("expected 0 for old timestamp, got %d", len(notifs))
	}
}

func TestCheckModelMigrations_LegacyOpusRemap(t *testing.T) {
	now := time.Now()
	notifs := CheckModelMigrations(MigrationConfig{
		LegacyOpusTimestamp: &now,
	})
	if len(notifs) != 1 {
		t.Fatalf("expected 1, got %d", len(notifs))
	}
	if notifs[0].Key != "opus-pro-update" {
		t.Errorf("key = %q", notifs[0].Key)
	}
	if !strings.Contains(notifs[0].Message, "CLAUDE_CODE_DISABLE_LEGACY_MODEL_REMAP") {
		t.Errorf("message = %q, want opt-out text", notifs[0].Message)
	}
	if notifs[0].TimeoutMs != 8_000 {
		t.Errorf("timeout = %d, want 8000 for legacy remap", notifs[0].TimeoutMs)
	}
}

func TestCheckModelMigrations_StandardOpusPro(t *testing.T) {
	now := time.Now()
	notifs := CheckModelMigrations(MigrationConfig{
		OpusProTimestamp: &now,
	})
	if len(notifs) != 1 {
		t.Fatalf("expected 1, got %d", len(notifs))
	}
	if notifs[0].Message != "Model updated to Opus 4.6" {
		t.Errorf("message = %q", notifs[0].Message)
	}
	if notifs[0].TimeoutMs != 3_000 {
		t.Errorf("timeout = %d, want 3000", notifs[0].TimeoutMs)
	}
}

func TestCheckModelMigrations_MultipleMigrations(t *testing.T) {
	now := time.Now()
	notifs := CheckModelMigrations(MigrationConfig{
		Sonnet45To46Timestamp: &now,
		OpusProTimestamp:       &now,
	})
	if len(notifs) != 2 {
		t.Fatalf("expected 2, got %d", len(notifs))
	}
}

func TestCheckModelMigrations_NoMigrations(t *testing.T) {
	notifs := CheckModelMigrations(MigrationConfig{})
	if len(notifs) != 0 {
		t.Errorf("expected 0, got %d", len(notifs))
	}
}

// ---------------------------------------------------------------------------
// formatDuration tests
// ---------------------------------------------------------------------------

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{0, "0s"},
		{30 * time.Second, "30s"},
		{5 * time.Minute, "5m"},
		{5*time.Minute + 30*time.Second, "5m30s"},
		{2 * time.Hour, "2h"},
		{2*time.Hour + 15*time.Minute, "2h15m"},
	}
	for _, tt := range tests {
		got := formatDuration(tt.d)
		if got != tt.want {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}
