// Package figures provides Unicode glyph constants for UI status indicators,
// spinners, effort levels, MCP subscriptions, and review states.
package figures

import "runtime"

// BlackCircle is set per-platform in init(); darwin uses ⏺ for better
// vertical alignment, other platforms fall back to ●.
var BlackCircle string

func init() {
	if runtime.GOOS == "darwin" {
		BlackCircle = "⏺"
	} else {
		BlackCircle = "●"
	}
}

const (
	BulletOperator   = "\u2219" // ∙
	TeardropAsterisk = "\u273B" // ✻ — Claude logo glyph
	UpArrow          = "\u2191" // ↑ — opus 1m merge notice
	DownArrow        = "\u2193" // ↓ — scroll hint
	LightningBolt    = "\u21AF" // ↯ — fast mode indicator

	// Effort level indicators.
	EffortLow    = "\u25CB" // ○
	EffortMedium = "\u25D0" // ◐
	EffortHigh   = "\u25CF" // ●
	EffortMax    = "\u25C9" // ◉ — Opus 4.6 only

	// Media/trigger status indicators.
	PlayIcon  = "\u25B6" // ▶
	PauseIcon = "\u23F8" // ⏸

	// MCP subscription indicators.
	RefreshArrow  = "\u21BB" // ↻ — resource update
	ChannelArrow  = "\u2190" // ← — inbound channel message
	InjectedArrow = "\u2192" // → — cross-session injected message
	ForkGlyph     = "\u2442" // ⑂ — fork directive

	// Review status indicators (ultrareview diamond states).
	DiamondOpen   = "\u25C7" // ◇ — running
	DiamondFilled = "\u25C6" // ◆ — completed/failed
	ReferenceMark = "\u203B" // ※ — away-summary recap marker

	// Issue flag indicator.
	FlagIcon = "\u2691" // ⚑

	// Blockquote / horizontal rule.
	BlockquoteBar   = "\u258E" // ▎ — left one-quarter block
	HeavyHorizontal = "\u2501" // ━ — heavy box-drawing horizontal

	// Bridge status indicators.
	BridgeReadyIndicator  = "\u00B7\u2714\uFE0E\u00B7" // ·✔︎·
	BridgeFailedIndicator = "\u00D7"                     // ×
)

// BridgeSpinnerFrames are the animation frames for the bridge connection spinner.
var BridgeSpinnerFrames = [4]string{
	"\u00B7|\u00B7",
	"\u00B7/\u00B7",
	"\u00B7\u2014\u00B7",
	"\u00B7\\\u00B7",
}
