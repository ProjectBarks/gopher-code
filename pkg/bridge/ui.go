// Package bridge — ui.go implements the bridge UI rendering: status bar,
// footer text, dialog fragments, and glyph/color constants.
// Source: src/bridge/bridgeUI.ts, src/bridge/bridgeStatusUtil.ts,
//         src/constants/figures.ts
package bridge

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/projectbarks/gopher-code/pkg/ui/figures"
)

// ---------------------------------------------------------------------------
// Glyph constants — delegated to pkg/ui/figures
// ---------------------------------------------------------------------------

// BridgeSpinnerFrames is the animation frames for the connecting/reconnecting
// spinner. Each frame is a 3-char glyph with middle-dot separators.
var BridgeSpinnerFrames = figures.BridgeSpinnerFrames[:]

// BridgeReadyIndicator is shown when the bridge is idle or attached.
const BridgeReadyIndicator = figures.BridgeReadyIndicator

// BridgeFailedIndicator is shown when the bridge has failed.
const BridgeFailedIndicator = figures.BridgeFailedIndicator

// MiddleDot is the separator used between status segments.
const MiddleDot = "\u00b7" // ·

// ---------------------------------------------------------------------------
// Lipgloss styles — semantic colors matching TS chalk usage
// ---------------------------------------------------------------------------

var (
	// styleYellow is used for connecting/reconnecting/ANT-ONLY labels.
	styleYellow = lipgloss.NewStyle().Foreground(lipgloss.Color("3")) // ANSI yellow
	// styleRed is used for failed/error states.
	styleRed = lipgloss.NewStyle().Foreground(lipgloss.Color("1")) // ANSI red
	// styleGreen is used for ready/connected/completed/sandbox-enabled.
	styleGreen = lipgloss.NewStyle().Foreground(lipgloss.Color("2")) // ANSI green
	// styleCyan is used for the attached/titled indicator and state text.
	styleCyan = lipgloss.NewStyle().Foreground(lipgloss.Color("6")) // ANSI cyan
	// styleDim is used for metadata, separators, timestamps, and footer text.
	styleDim = lipgloss.NewStyle().Faint(true)
	// styleDimItalic is used for QR-toggle and spawn-mode hints.
	styleDimItalic = lipgloss.NewStyle().Faint(true).Italic(true)
)

// ---------------------------------------------------------------------------
// Truncation widths (from TS source)
// ---------------------------------------------------------------------------

const (
	// TruncateWidthMultiSessionTitle is the max width for a session title in the
	// multi-session bullet list.
	TruncateWidthMultiSessionTitle = 35
	// TruncateWidthMainStatusTitle is the max width for a title on the main
	// status line (single-session mode).
	TruncateWidthMainStatusTitle = 40
	// TruncateWidthActivityMulti is the max width for activity text in multi-session.
	TruncateWidthActivityMulti = 40
	// TruncateWidthActivitySingle is the max width for tool activity in single-session.
	TruncateWidthActivitySingle = 60
	// TruncateWidthLogPrompt is the max width for a prompt in verbose log lines.
	TruncateWidthLogPrompt = 80
)

// ---------------------------------------------------------------------------
// Status bar rendering
// ---------------------------------------------------------------------------

// RenderConnectingLine returns the connecting spinner status line.
// tick selects the spinner frame; repoName and branch are optional suffixes.
func RenderConnectingLine(tick int, repoName, branch string) string {
	frame := BridgeSpinnerFrames[tick%len(BridgeSpinnerFrames)]
	var b strings.Builder
	b.WriteString(styleYellow.Render(frame))
	b.WriteString(" ")
	b.WriteString(styleYellow.Render("Connecting"))
	if repoName != "" {
		b.WriteString(styleDim.Render(" " + MiddleDot + " "))
		b.WriteString(styleDim.Render(repoName))
	}
	if branch != "" {
		b.WriteString(styleDim.Render(" " + MiddleDot + " "))
		b.WriteString(styleDim.Render(branch))
	}
	return b.String()
}

// RenderIdleStatusLine returns the status line for the idle (Ready) state.
// repoName and branch are optional suffixes.
func RenderIdleStatusLine(repoName, branch string, spawnMode SpawnMode) string {
	return renderReadyStatusLine(BridgeReadyIndicator, "Ready", true, repoName, branch, spawnMode)
}

// RenderConnectedStatusLine returns the status line for the attached (Connected) state.
// repoName and branch are optional suffixes.
func RenderConnectedStatusLine(repoName, branch string, spawnMode SpawnMode) string {
	return renderReadyStatusLine(BridgeReadyIndicator, "Connected", false, repoName, branch, spawnMode)
}

// RenderTitledStatusLine returns the status line when a session title is shown.
func RenderTitledStatusLine(title, repoName, branch string, spawnMode SpawnMode) string {
	return renderReadyStatusLine(BridgeReadyIndicator, title, false, repoName, branch, spawnMode)
}

// renderReadyStatusLine builds the indicator + state text + suffix line.
func renderReadyStatusLine(indicator, stateText string, isIdle bool, repoName, branch string, spawnMode SpawnMode) string {
	indicatorStyle := styleGreen
	baseStyle := styleGreen
	if !isIdle {
		indicatorStyle = styleCyan
		baseStyle = styleCyan
	}

	var b strings.Builder
	b.WriteString(indicatorStyle.Render(indicator))
	b.WriteString(" ")
	b.WriteString(baseStyle.Render(stateText))
	if repoName != "" {
		b.WriteString(styleDim.Render(" " + MiddleDot + " "))
		b.WriteString(styleDim.Render(repoName))
	}
	// In worktree mode each session gets its own branch, so showing
	// the bridge's branch would be misleading.
	if branch != "" && spawnMode != SpawnModeWorktree {
		b.WriteString(styleDim.Render(" " + MiddleDot + " "))
		b.WriteString(styleDim.Render(branch))
	}
	return b.String()
}

// ---------------------------------------------------------------------------
// Reconnecting status line
// ---------------------------------------------------------------------------

// RenderReconnectingLine returns the reconnecting spinner status line.
func RenderReconnectingLine(tick int, delayStr, elapsedStr string) string {
	frame := BridgeSpinnerFrames[tick%len(BridgeSpinnerFrames)]
	var b strings.Builder
	b.WriteString(styleYellow.Render(frame))
	b.WriteString(" ")
	b.WriteString(styleYellow.Render("Reconnecting"))
	b.WriteString(" ")
	b.WriteString(styleDim.Render(MiddleDot))
	b.WriteString(" ")
	b.WriteString(styleDim.Render("retrying in " + delayStr))
	b.WriteString(" ")
	b.WriteString(styleDim.Render(MiddleDot))
	b.WriteString(" ")
	b.WriteString(styleDim.Render("disconnected " + elapsedStr))
	return b.String()
}

// ---------------------------------------------------------------------------
// Failed status line
// ---------------------------------------------------------------------------

// RenderFailedStatusLine returns the failed status block (indicator + label +
// footer + optional error).
func RenderFailedStatusLine(errorMsg, repoName, branch string) string {
	var b strings.Builder

	// First line: indicator + "Remote Control Failed" + suffix
	b.WriteString(styleRed.Render(BridgeFailedIndicator))
	b.WriteString(" ")
	b.WriteString(styleRed.Render("Remote Control Failed"))
	if repoName != "" {
		b.WriteString(styleDim.Render(" " + MiddleDot + " "))
		b.WriteString(styleDim.Render(repoName))
	}
	if branch != "" {
		b.WriteString(styleDim.Render(" " + MiddleDot + " "))
		b.WriteString(styleDim.Render(branch))
	}
	b.WriteString("\n")

	// Second line: failed footer
	b.WriteString(styleDim.Render(FailedFooterText))
	b.WriteString("\n")

	// Optional third line: the error message
	if errorMsg != "" {
		b.WriteString(styleRed.Render(errorMsg))
		b.WriteString("\n")
	}

	return b.String()
}

// ---------------------------------------------------------------------------
// ANT-ONLY debug log line
// ---------------------------------------------------------------------------

// RenderAntOnlyLogLine returns the "[ANT-ONLY] Logs: <path>" line.
func RenderAntOnlyLogLine(debugLogPath string) string {
	return styleYellow.Render("[ANT-ONLY] Logs:") + " " + styleDim.Render(debugLogPath)
}

// ---------------------------------------------------------------------------
// Session info display
// ---------------------------------------------------------------------------

// SessionDisplayInfo holds per-session display state for the multi-session
// bullet list.
type SessionDisplayInfo struct {
	Title    string
	URL      string
	Activity *SessionActivity
}

// RenderSessionBullet renders one session entry in the multi-session list.
// The title is wrapped in an OSC 8 hyperlink to the session URL.
func RenderSessionBullet(info SessionDisplayInfo) string {
	titleText := styleDim.Render("Attached")
	if info.Title != "" {
		titleText = TruncateString(info.Title, TruncateWidthMultiSessionTitle)
	}
	linked := WrapWithOSC8Link(titleText, info.URL)

	var actText string
	if info.Activity != nil && info.Activity.Type != ActivityResult && info.Activity.Type != ActivityError {
		actText = " " + styleDim.Render(TruncateString(info.Activity.Summary, TruncateWidthActivityMulti))
	}

	return "    " + linked + actText
}

// ---------------------------------------------------------------------------
// Capacity / mode lines
// ---------------------------------------------------------------------------

// RenderCapacityLine returns the capacity + mode hint line for multi-session
// (sessionMax > 1).
func RenderCapacityLine(active, max int, mode SpawnMode) string {
	modeHint := spawnModeHint(mode)
	return "    " + styleDim.Render(fmt.Sprintf("Capacity: %d/%d %s %s", active, max, MiddleDot, modeHint))
}

// RenderSingleModeCapacityLine returns the mode line when sessionMax == 1.
func RenderSingleModeCapacityLine(active int, mode SpawnMode) string {
	var modeText string
	switch mode {
	case SpawnModeSingleSession:
		modeText = "Single session " + MiddleDot + " exits when complete"
	case SpawnModeWorktree:
		modeText = fmt.Sprintf("Capacity: %d/1 %s New sessions will be created in an isolated worktree", active, MiddleDot)
	default: // same-dir
		modeText = fmt.Sprintf("Capacity: %d/1 %s New sessions will be created in the current directory", active, MiddleDot)
	}
	return "    " + styleDim.Render(modeText)
}

// spawnModeHint returns the human-readable hint for a spawn mode.
func spawnModeHint(mode SpawnMode) string {
	switch mode {
	case SpawnModeWorktree:
		return "New sessions will be created in an isolated worktree"
	default:
		return "New sessions will be created in the current directory"
	}
}

// ---------------------------------------------------------------------------
// Footer rendering
// ---------------------------------------------------------------------------

// RenderFooter returns the footer block: footer text + QR hint + optional
// spawn-mode toggle hint.
func RenderFooter(url string, isIdle, qrVisible bool, spawnModeDisplay *SpawnMode) string {
	var footerText string
	if isIdle {
		footerText = BuildIdleFooterText(url)
	} else {
		footerText = BuildActiveFooterText(url)
	}

	var qrHint string
	if qrVisible {
		qrHint = styleDimItalic.Render("space to hide QR code")
	} else {
		qrHint = styleDimItalic.Render("space to show QR code")
	}

	var toggleHint string
	if spawnModeDisplay != nil {
		toggleHint = styleDimItalic.Render(" " + MiddleDot + " w to toggle spawn mode")
	}

	return styleDim.Render(footerText) + "\n" + qrHint + toggleHint
}

// ---------------------------------------------------------------------------
// Banner rendering (verbose mode)
// ---------------------------------------------------------------------------

// RenderVerboseBanner returns the verbose banner lines shown at startup.
func RenderVerboseBanner(version string, config BridgeConfig, environmentID string) string {
	var b strings.Builder
	b.WriteString(styleDim.Render("Remote Control") + " v" + version + "\n")
	if config.SpawnMode != SpawnModeSingleSession {
		b.WriteString(styleDim.Render("Spawn mode: ") + string(config.SpawnMode) + "\n")
		b.WriteString(styleDim.Render("Max concurrent sessions: ") + fmt.Sprintf("%d", config.MaxSessions) + "\n")
	}
	b.WriteString(styleDim.Render("Environment ID: ") + environmentID + "\n")
	return b.String()
}

// RenderSandboxLine returns the "Sandbox: Enabled" line.
func RenderSandboxLine() string {
	return styleDim.Render("Sandbox:") + " " + styleGreen.Render("Enabled")
}

// ---------------------------------------------------------------------------
// Log line rendering
// ---------------------------------------------------------------------------

// RenderLogSessionComplete returns the session-completed log line.
func RenderLogSessionComplete(ts, sessionID, duration string) string {
	return styleDim.Render("["+ts+"]") + " Session " +
		styleGreen.Render("completed") + " (" + duration + ") " +
		styleDim.Render(sessionID)
}

// RenderLogSessionFailed returns the session-failed log line.
func RenderLogSessionFailed(ts, sessionID, errMsg string) string {
	return styleDim.Render("["+ts+"]") + " Session " +
		styleRed.Render("failed") + ": " + errMsg + " " +
		styleDim.Render(sessionID)
}

// RenderLogSessionStart returns the verbose session-started log line.
func RenderLogSessionStart(ts, sessionID, prompt string) string {
	return styleDim.Render("["+ts+"]") + " Session started: \"" + prompt + "\" (" +
		styleDim.Render(sessionID) + ")"
}

// RenderLogStatus returns a timestamped status log line.
func RenderLogStatus(ts, message string) string {
	return styleDim.Render("["+ts+"]") + " " + message
}

// RenderLogError returns a timestamped error log line (all red).
func RenderLogError(ts, message string) string {
	return styleRed.Render("[" + ts + "] Error: " + message)
}

// RenderLogReconnected returns the "Reconnected after <duration>" log line.
func RenderLogReconnected(ts, duration string) string {
	return styleDim.Render("["+ts+"]") + " " +
		styleGreen.Render("Reconnected") + " after " + duration
}

// ---------------------------------------------------------------------------
// OSC 8 hyperlink wrapper
// ---------------------------------------------------------------------------

// WrapWithOSC8Link wraps text in an OSC 8 terminal hyperlink sequence.
func WrapWithOSC8Link(text, url string) string {
	return "\x1b]8;;" + url + "\x07" + text + "\x1b]8;;\x07"
}

// ---------------------------------------------------------------------------
// String truncation helper
// ---------------------------------------------------------------------------

// TruncateString truncates s to maxWidth runes, appending "..." if truncated.
// This is a simplified version; a production implementation should use
// grapheme-aware width (uniseg).
func TruncateString(s string, maxWidth int) string {
	runes := []rune(s)
	if len(runes) <= maxWidth {
		return s
	}
	if maxWidth <= 3 {
		return string(runes[:maxWidth])
	}
	return string(runes[:maxWidth-3]) + "..."
}
