// Package misc provides small rendering components used across the TUI.
//
// Source: components/EffortCallout.tsx, StatusNotices.tsx, SessionBackgroundHint.tsx,
//         Stats.tsx, EffortIndicator.tsx, ConfigurableShortcutHint.tsx, etc.
//
// These are the "20 small components" — avatars, badges, effort indicators,
// status notices, session hints, and stat displays that don't warrant their
// own package.
package misc

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// ---------------------------------------------------------------------------
// Effort Indicator — Source: components/EffortIndicator.tsx
// ---------------------------------------------------------------------------

// EffortLevel describes reasoning effort.
type EffortLevel string

const (
	EffortLow    EffortLevel = "low"
	EffortMedium EffortLevel = "medium"
	EffortHigh   EffortLevel = "high"
	EffortMax    EffortLevel = "max"
)

// EffortSymbol returns the display symbol for an effort level.
func EffortSymbol(level EffortLevel) string {
	switch level {
	case EffortLow:
		return "⚡"
	case EffortMedium:
		return "⚡⚡"
	case EffortHigh:
		return "⚡⚡⚡"
	case EffortMax:
		return "⚡⚡⚡⚡"
	default:
		return ""
	}
}

// RenderEffortBadge returns a styled effort level badge.
func RenderEffortBadge(level EffortLevel) string {
	if level == "" {
		return ""
	}
	colors := theme.Current().Colors()
	style := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Warning))
	return style.Render(fmt.Sprintf("%s %s", EffortSymbol(level), string(level)))
}

// ---------------------------------------------------------------------------
// Session Background Hint — Source: components/SessionBackgroundHint.tsx
// ---------------------------------------------------------------------------

// RenderSessionBackgroundHint returns the hint shown when a session runs in background.
func RenderSessionBackgroundHint(sessionName string) string {
	colors := theme.Current().Colors()
	dimStyle := lipgloss.NewStyle().Faint(true)
	nameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Accent)).Bold(true)

	return dimStyle.Render("Session ") + nameStyle.Render(sessionName) +
		dimStyle.Render(" is running in the background")
}

// ---------------------------------------------------------------------------
// Stats — Source: components/Stats.tsx (turn completion stats)
// ---------------------------------------------------------------------------

// TurnStats holds statistics for a completed turn.
type TurnStats struct {
	Duration     time.Duration
	InputTokens  int
	OutputTokens int
	CacheRead    int
	CacheWrite   int
	Cost         float64
}

// RenderTurnStats returns a formatted stats line for a completed turn.
func RenderTurnStats(stats TurnStats) string {
	colors := theme.Current().Colors()
	dimStyle := lipgloss.NewStyle().Faint(true)
	accentStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Accent))

	var parts []string

	// Duration
	parts = append(parts, accentStyle.Render(formatDuration(stats.Duration)))

	// Tokens
	if stats.InputTokens > 0 || stats.OutputTokens > 0 {
		parts = append(parts, dimStyle.Render(fmt.Sprintf("%s in / %s out",
			formatTokens(stats.InputTokens), formatTokens(stats.OutputTokens))))
	}

	// Cache
	if stats.CacheRead > 0 || stats.CacheWrite > 0 {
		parts = append(parts, dimStyle.Render(fmt.Sprintf("cache: %s read, %s write",
			formatTokens(stats.CacheRead), formatTokens(stats.CacheWrite))))
	}

	// Cost
	if stats.Cost > 0 {
		parts = append(parts, dimStyle.Render(fmt.Sprintf("$%.4f", stats.Cost)))
	}

	return strings.Join(parts, dimStyle.Render(" · "))
}

// ---------------------------------------------------------------------------
// Shortcut Hint — Source: components/ConfigurableShortcutHint.tsx
// ---------------------------------------------------------------------------

// RenderShortcutHint returns a styled keyboard shortcut hint.
func RenderShortcutHint(shortcut, description string) string {
	colors := theme.Current().Colors()
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.TextSecondary)).Bold(true)
	descStyle := lipgloss.NewStyle().Faint(true)
	return keyStyle.Render(shortcut) + " " + descStyle.Render(description)
}

// RenderShortcutBar renders a horizontal bar of keyboard shortcuts.
func RenderShortcutBar(hints []ShortcutHint) string {
	dimStyle := lipgloss.NewStyle().Faint(true)
	var parts []string
	for _, h := range hints {
		parts = append(parts, RenderShortcutHint(h.Key, h.Description))
	}
	return strings.Join(parts, dimStyle.Render("  "))
}

// ShortcutHint is a key + description pair for the shortcut bar.
type ShortcutHint struct {
	Key         string
	Description string
}

// ---------------------------------------------------------------------------
// Status Notice — Source: utils/statusNoticeDefinitions.ts
// ---------------------------------------------------------------------------

// StatusNotice is a notice shown at startup or during the session.
type StatusNotice struct {
	ID      string
	Message string
	Color   string // theme color key
}

// RenderNotices returns formatted status notices.
func RenderNotices(notices []StatusNotice) string {
	if len(notices) == 0 {
		return ""
	}
	var b strings.Builder
	for _, n := range notices {
		style := lipgloss.NewStyle().Faint(true)
		if n.Color != "" {
			colors := theme.Current().Colors()
			colorVal := resolveColor(n.Color, colors)
			if colorVal != "" {
				style = lipgloss.NewStyle().Foreground(lipgloss.Color(colorVal))
			}
		}
		b.WriteString("  " + style.Render(n.Message) + "\n")
	}
	return b.String()
}

// ---------------------------------------------------------------------------
// Avatar / Role Badge — used in message rendering
// ---------------------------------------------------------------------------

// RenderRoleBadge returns a styled role badge (Human, Assistant, System, Tool).
func RenderRoleBadge(role string) string {
	colors := theme.Current().Colors()
	var color string
	switch role {
	case "human", "user":
		color = colors.Info
	case "assistant":
		color = colors.Accent
	case "system":
		color = colors.Warning
	case "tool":
		color = colors.ToolName
	default:
		color = colors.TextMuted
	}
	style := lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Bold(true)
	return style.Render(role)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
}

func formatTokens(n int) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}

func resolveColor(name string, colors theme.ColorScheme) string {
	switch name {
	case "warning":
		return colors.Warning
	case "error":
		return colors.Error
	case "success":
		return colors.Success
	case "info":
		return colors.Info
	case "accent":
		return colors.Accent
	default:
		return name // treat as raw color value
	}
}
