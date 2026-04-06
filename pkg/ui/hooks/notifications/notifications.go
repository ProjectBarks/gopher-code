// Package notifications provides notification hooks that check conditions and
// return Notification structs for display. Each hook is a compact function that
// evaluates a specific concern (rate limits, settings errors, model migrations,
// etc.) and returns zero or more notifications.
//
// Source: src/hooks/notifs/
package notifications

import (
	"fmt"
	"os"
	"time"
)

// Priority determines notification ordering and display urgency.
type Priority int

const (
	// PriorityLow is for informational notifications.
	PriorityLow Priority = iota
	// PriorityHigh is for important warnings (settings errors, model migrations).
	PriorityHigh
	// PriorityImmediate is for urgent alerts (rate limits, overage).
	PriorityImmediate
)

// Color names matching the TS theme keys.
const (
	ColorWarning    = "warning"
	ColorSuggestion = "suggestion"
	ColorFastMode   = "fastMode"
)

// Notification is the output of a notification hook check.
type Notification struct {
	// Key uniquely identifies this notification for dedup/invalidation.
	Key string
	// Message is the user-visible text.
	Message string
	// Priority determines display ordering.
	Priority Priority
	// Color is the theme color key (warning, suggestion, fastMode).
	Color string
	// TimeoutMs is how long the notification stays visible (0 = default 3s).
	TimeoutMs int
}

// Separator used between notification text segments (matches TS " · ").
const Separator = " \u00b7 "

// ---------------------------------------------------------------------------
// Rate-limit warning — Source: src/hooks/notifs/useRateLimitWarningNotification.tsx
// ---------------------------------------------------------------------------

// RateLimitState holds live rate-limit information from the API.
type RateLimitState struct {
	// IsUsingOverage indicates the user has exceeded their plan limit.
	IsUsingOverage bool
	// WarningText is the pre-formatted approaching-limit warning (per-model).
	// Empty string means no warning.
	WarningText string
	// OverageText is the pre-formatted overage message.
	OverageText string
	// SubscriptionType is "team", "enterprise", or empty for individual.
	SubscriptionType string
	// HasBillingAccess indicates the user can manage billing.
	HasBillingAccess bool
}

// CheckRateLimit evaluates rate-limit state and returns notifications.
// Team/enterprise users without billing access skip the overage notification.
// Source: useRateLimitWarningNotification (113 LOC)
func CheckRateLimit(state RateLimitState) []Notification {
	var out []Notification

	// Overage notification — immediate priority, shown once per overage entry.
	if state.IsUsingOverage {
		// Team/enterprise without billing access → skip (not their decision).
		isBillingRestricted := state.SubscriptionType == "team" || state.SubscriptionType == "enterprise"
		if !isBillingRestricted || state.HasBillingAccess {
			text := state.OverageText
			if text == "" {
				text = "You have exceeded your usage limit"
			}
			out = append(out, Notification{
				Key:      "limit-reached",
				Message:  text,
				Priority: PriorityImmediate,
			})
		}
	}

	// Approaching-limit warning — high priority, deduped by text content.
	if state.WarningText != "" {
		out = append(out, Notification{
			Key:      "rate-limit-warning",
			Message:  state.WarningText,
			Priority: PriorityHigh,
			Color:    ColorWarning,
		})
	}

	return out
}

// ---------------------------------------------------------------------------
// Settings errors — Source: src/hooks/notifs/useSettingsErrors.tsx
// ---------------------------------------------------------------------------

// SettingsError represents a single validation issue in user settings.
type SettingsError struct {
	Path    string
	Message string
}

// CheckSettingsErrors returns a notification if there are settings validation
// issues. The notification tells the user to run /doctor for details.
// Source: useSettingsErrors (68 LOC)
func CheckSettingsErrors(errors []SettingsError) *Notification {
	if len(errors) == 0 {
		return nil
	}

	noun := "issues"
	if len(errors) == 1 {
		noun = "issue"
	}

	return &Notification{
		Key:       "settings-errors",
		Message:   fmt.Sprintf("Found %d settings %s"+Separator+"/doctor for details", len(errors), noun),
		Priority:  PriorityHigh,
		Color:     ColorWarning,
		TimeoutMs: 60_000, // 1 minute — longer to ensure user sees it
	}
}

// ---------------------------------------------------------------------------
// Startup notification — Source: src/hooks/notifs/useStartupNotification.ts
// ---------------------------------------------------------------------------

// StartupCheck is a function that computes notifications to show at startup.
// Returns nil if no notification should be shown.
type StartupCheck func() []Notification

// CheckStartup runs a startup check function once and returns its results.
// In the TS source this is a once-per-session React hook with a ref guard.
// In Go/bubbletea this maps to a one-shot tea.Cmd dispatched at Init() time.
// Source: useStartupNotification (41 LOC)
func CheckStartup(isRemoteMode bool, compute StartupCheck) []Notification {
	if isRemoteMode {
		return nil
	}
	if compute == nil {
		return nil
	}
	return compute()
}

// ---------------------------------------------------------------------------
// Fast-mode notification — Source: src/hooks/notifs/useFastModeNotification.tsx
// ---------------------------------------------------------------------------

// CooldownReason identifies why fast mode entered cooldown.
type CooldownReason string

const (
	CooldownOverloaded CooldownReason = "overloaded"
	CooldownRateLimit  CooldownReason = "rate_limit"
)

// FastModeEvent describes a fast-mode lifecycle transition.
type FastModeEvent struct {
	// Type is the event kind.
	Type FastModeEventType
	// OrgEnabled is set for OrgChanged events.
	OrgEnabled bool
	// ResetAt is set for CooldownTriggered events.
	ResetAt time.Time
	// Reason is set for CooldownTriggered events.
	Reason CooldownReason
	// Message is set for OverageRejection events.
	Message string
}

// FastModeEventType identifies the fast-mode event kind.
type FastModeEventType int

const (
	FastModeOrgChanged FastModeEventType = iota
	FastModeOverageRejection
	FastModeCooldownTriggered
	FastModeCooldownExpired
)

// CheckFastModeEvent returns a notification for a fast-mode lifecycle event.
// Source: useFastModeNotification (161 LOC)
func CheckFastModeEvent(event FastModeEvent) *Notification {
	switch event.Type {
	case FastModeOrgChanged:
		if event.OrgEnabled {
			return &Notification{
				Key:      "fast-mode-org-changed",
				Message:  "Fast mode is now available" + Separator + "/fast to turn on",
				Priority: PriorityImmediate,
				Color:    ColorFastMode,
			}
		}
		return &Notification{
			Key:      "fast-mode-org-changed",
			Message:  "Fast mode has been disabled by your organization",
			Priority: PriorityImmediate,
			Color:    ColorWarning,
		}

	case FastModeOverageRejection:
		msg := event.Message
		if msg == "" {
			msg = "Fast mode overage rejected"
		}
		return &Notification{
			Key:      "fast-mode-overage-rejected",
			Message:  msg,
			Priority: PriorityImmediate,
			Color:    ColorWarning,
		}

	case FastModeCooldownTriggered:
		dur := time.Until(event.ResetAt)
		if dur < 0 {
			dur = 0
		}
		durStr := formatDuration(dur)
		var msg string
		switch event.Reason {
		case CooldownOverloaded:
			msg = "Fast mode overloaded and is temporarily unavailable" + Separator + "resets in " + durStr
		default: // rate_limit
			msg = "Fast limit reached and temporarily disabled" + Separator + "resets in " + durStr
		}
		return &Notification{
			Key:      "fast-mode-cooldown-started",
			Message:  msg,
			Priority: PriorityImmediate,
			Color:    ColorWarning,
		}

	case FastModeCooldownExpired:
		return &Notification{
			Key:      "fast-mode-cooldown-expired",
			Message:  "Fast limit reset" + Separator + "now using fast mode",
			Priority: PriorityImmediate,
			Color:    ColorFastMode,
		}
	}

	return nil
}

// formatDuration formats a duration into a compact human-readable string,
// hiding trailing zero components (matches TS formatDuration with hideTrailingZeros).
func formatDuration(d time.Duration) string {
	if d <= 0 {
		return "0s"
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60

	switch {
	case h > 0 && m > 0:
		return fmt.Sprintf("%dh%dm", h, m)
	case h > 0:
		return fmt.Sprintf("%dh", h)
	case m > 0 && s > 0:
		return fmt.Sprintf("%dm%ds", m, s)
	case m > 0:
		return fmt.Sprintf("%dm", m)
	default:
		return fmt.Sprintf("%ds", s)
	}
}

// ---------------------------------------------------------------------------
// Model migration — Source: src/hooks/notifs/useModelMigrationNotifications.tsx
// ---------------------------------------------------------------------------

// MigrationConfig holds timestamps for model migrations from global config.
type MigrationConfig struct {
	Sonnet45To46Timestamp     *time.Time
	OpusProTimestamp           *time.Time
	LegacyOpusTimestamp       *time.Time
}

// CheckModelMigrations returns notifications for any model migrations that
// happened recently (within 3 seconds — i.e., this launch).
// Source: useModelMigrationNotifications (51 LOC)
func CheckModelMigrations(cfg MigrationConfig) []Notification {
	// CLAUDE_CODE_DISABLE_LEGACY_MODEL_REMAP=1 opts out of legacy remap notifications.
	disableLegacyRemap := os.Getenv("CLAUDE_CODE_DISABLE_LEGACY_MODEL_REMAP") == "1"

	const recentWindow = 3 * time.Second
	now := time.Now()
	recent := func(ts *time.Time) bool {
		return ts != nil && now.Sub(*ts) < recentWindow
	}

	var out []Notification

	// Sonnet 4.5 → 4.6 migration
	if recent(cfg.Sonnet45To46Timestamp) {
		out = append(out, Notification{
			Key:       "sonnet-46-update",
			Message:   "Model updated to Sonnet 4.6",
			Priority:  PriorityHigh,
			Color:     ColorSuggestion,
			TimeoutMs: 3_000,
		})
	}

	// Opus Pro migration (two variants: legacy-remap vs standard)
	if !disableLegacyRemap && recent(cfg.LegacyOpusTimestamp) {
		out = append(out, Notification{
			Key:       "opus-pro-update",
			Message:   "Model updated to Opus 4.6" + Separator + "Set CLAUDE_CODE_DISABLE_LEGACY_MODEL_REMAP=1 to opt out",
			Priority:  PriorityHigh,
			Color:     ColorSuggestion,
			TimeoutMs: 8_000, // Longer so user can read opt-out instructions
		})
	} else if recent(cfg.OpusProTimestamp) {
		out = append(out, Notification{
			Key:       "opus-pro-update",
			Message:   "Model updated to Opus 4.6",
			Priority:  PriorityHigh,
			Color:     ColorSuggestion,
			TimeoutMs: 3_000,
		})
	}

	return out
}
