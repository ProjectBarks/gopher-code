package bridge

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Glyph constants
// ---------------------------------------------------------------------------

func TestBridgeSpinnerFrames(t *testing.T) {
	want := []string{"·|·", "·/·", "·—·", "·\\·"}
	if len(BridgeSpinnerFrames) != len(want) {
		t.Fatalf("BridgeSpinnerFrames len = %d, want %d", len(BridgeSpinnerFrames), len(want))
	}
	for i, w := range want {
		if BridgeSpinnerFrames[i] != w {
			t.Errorf("BridgeSpinnerFrames[%d] = %q, want %q", i, BridgeSpinnerFrames[i], w)
		}
	}
}

func TestBridgeReadyIndicator(t *testing.T) {
	if BridgeReadyIndicator != "·✔︎·" {
		t.Errorf("BridgeReadyIndicator = %q, want %q", BridgeReadyIndicator, "·✔︎·")
	}
}

func TestBridgeFailedIndicator(t *testing.T) {
	if BridgeFailedIndicator != "×" {
		t.Errorf("BridgeFailedIndicator = %q, want %q", BridgeFailedIndicator, "×")
	}
}

// ---------------------------------------------------------------------------
// Status bar text — Connecting
// ---------------------------------------------------------------------------

func TestRenderConnectingLine_plainText(t *testing.T) {
	line := RenderConnectingLine(0, "", "")
	if !containsPlain(line, "Connecting") {
		t.Errorf("connecting line missing 'Connecting': %q", line)
	}
	// Frame 0 = ·|·
	if !containsPlain(line, "·|·") {
		t.Errorf("connecting line missing spinner frame 0: %q", line)
	}
}

func TestRenderConnectingLine_withRepoAndBranch(t *testing.T) {
	line := RenderConnectingLine(1, "my-repo", "main")
	if !containsPlain(line, "Connecting") {
		t.Errorf("missing 'Connecting': %q", line)
	}
	if !containsPlain(line, "my-repo") {
		t.Errorf("missing repo name: %q", line)
	}
	if !containsPlain(line, "main") {
		t.Errorf("missing branch: %q", line)
	}
	// Frame 1 = ·/·
	if !containsPlain(line, "·/·") {
		t.Errorf("missing spinner frame 1: %q", line)
	}
}

func TestRenderConnectingLine_cyclesFrames(t *testing.T) {
	for i, want := range BridgeSpinnerFrames {
		line := RenderConnectingLine(i, "", "")
		if !containsPlain(line, want) {
			t.Errorf("tick %d: missing frame %q in %q", i, want, line)
		}
	}
	// Verify wrapping: tick 4 == frame 0
	line := RenderConnectingLine(4, "", "")
	if !containsPlain(line, BridgeSpinnerFrames[0]) {
		t.Errorf("tick 4 should wrap to frame 0: %q", line)
	}
}

// ---------------------------------------------------------------------------
// Status bar text — Idle (Ready)
// ---------------------------------------------------------------------------

func TestRenderIdleStatusLine_containsReady(t *testing.T) {
	line := RenderIdleStatusLine("", "", SpawnModeSingleSession)
	if !containsPlain(line, "Ready") {
		t.Errorf("idle line missing 'Ready': %q", line)
	}
	if !containsPlain(line, BridgeReadyIndicator) {
		t.Errorf("idle line missing indicator: %q", line)
	}
}

func TestRenderIdleStatusLine_withRepoAndBranch(t *testing.T) {
	line := RenderIdleStatusLine("my-repo", "main", SpawnModeSingleSession)
	if !containsPlain(line, "my-repo") {
		t.Errorf("missing repo: %q", line)
	}
	if !containsPlain(line, "main") {
		t.Errorf("missing branch: %q", line)
	}
}

func TestRenderIdleStatusLine_worktreeHidesBranch(t *testing.T) {
	line := RenderIdleStatusLine("my-repo", "main", SpawnModeWorktree)
	if !containsPlain(line, "my-repo") {
		t.Errorf("missing repo: %q", line)
	}
	if containsPlain(line, "main") {
		t.Errorf("worktree mode should hide branch: %q", line)
	}
}

// ---------------------------------------------------------------------------
// Status bar text — Connected
// ---------------------------------------------------------------------------

func TestRenderConnectedStatusLine_containsConnected(t *testing.T) {
	line := RenderConnectedStatusLine("", "", SpawnModeSingleSession)
	if !containsPlain(line, "Connected") {
		t.Errorf("connected line missing 'Connected': %q", line)
	}
	if !containsPlain(line, BridgeReadyIndicator) {
		t.Errorf("connected line missing indicator: %q", line)
	}
}

// ---------------------------------------------------------------------------
// Status bar text — Titled
// ---------------------------------------------------------------------------

func TestRenderTitledStatusLine(t *testing.T) {
	line := RenderTitledStatusLine("Fix the bug", "repo", "dev", SpawnModeSameDir)
	if !containsPlain(line, "Fix the bug") {
		t.Errorf("titled line missing title: %q", line)
	}
	if !containsPlain(line, "repo") {
		t.Errorf("titled line missing repo: %q", line)
	}
	if !containsPlain(line, "dev") {
		t.Errorf("titled line missing branch: %q", line)
	}
}

// ---------------------------------------------------------------------------
// Reconnecting status line
// ---------------------------------------------------------------------------

func TestRenderReconnectingLine(t *testing.T) {
	line := RenderReconnectingLine(2, "5s", "30s")
	if !containsPlain(line, "Reconnecting") {
		t.Errorf("missing 'Reconnecting': %q", line)
	}
	if !containsPlain(line, "retrying in 5s") {
		t.Errorf("missing 'retrying in 5s': %q", line)
	}
	if !containsPlain(line, "disconnected 30s") {
		t.Errorf("missing 'disconnected 30s': %q", line)
	}
	// Frame 2 = ·—·
	if !containsPlain(line, "·—·") {
		t.Errorf("missing spinner frame 2: %q", line)
	}
}

// ---------------------------------------------------------------------------
// Failed status line
// ---------------------------------------------------------------------------

func TestRenderFailedStatusLine_verbatimStrings(t *testing.T) {
	out := RenderFailedStatusLine("connection lost", "my-repo", "main")
	if !containsPlain(out, "Remote Control Failed") {
		t.Errorf("missing 'Remote Control Failed': %q", out)
	}
	if !containsPlain(out, BridgeFailedIndicator) {
		t.Errorf("missing failed indicator: %q", out)
	}
	if !containsPlain(out, FailedFooterText) {
		t.Errorf("missing FailedFooterText verbatim: %q", out)
	}
	if !containsPlain(out, "connection lost") {
		t.Errorf("missing error message: %q", out)
	}
	if !containsPlain(out, "my-repo") {
		t.Errorf("missing repo: %q", out)
	}
	if !containsPlain(out, "main") {
		t.Errorf("missing branch: %q", out)
	}
}

func TestRenderFailedStatusLine_noError(t *testing.T) {
	out := RenderFailedStatusLine("", "", "")
	if !containsPlain(out, "Remote Control Failed") {
		t.Errorf("missing 'Remote Control Failed': %q", out)
	}
	if !containsPlain(out, FailedFooterText) {
		t.Errorf("missing footer: %q", out)
	}
	// Should be exactly 2 lines (no error line)
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines without error, got %d: %v", len(lines), lines)
	}
}

func TestRenderFailedStatusLine_withError(t *testing.T) {
	out := RenderFailedStatusLine("timeout", "", "")
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 lines with error, got %d: %v", len(lines), lines)
	}
}

// ---------------------------------------------------------------------------
// ANT-ONLY log line
// ---------------------------------------------------------------------------

func TestRenderAntOnlyLogLine(t *testing.T) {
	line := RenderAntOnlyLogLine("/tmp/debug.log")
	if !containsPlain(line, "[ANT-ONLY] Logs:") {
		t.Errorf("missing '[ANT-ONLY] Logs:': %q", line)
	}
	if !containsPlain(line, "/tmp/debug.log") {
		t.Errorf("missing path: %q", line)
	}
}

// ---------------------------------------------------------------------------
// Capacity lines
// ---------------------------------------------------------------------------

func TestRenderCapacityLine_worktree(t *testing.T) {
	line := RenderCapacityLine(2, 5, SpawnModeWorktree)
	if !containsPlain(line, "Capacity: 2/5") {
		t.Errorf("missing 'Capacity: 2/5': %q", line)
	}
	if !containsPlain(line, "New sessions will be created in an isolated worktree") {
		t.Errorf("missing worktree hint: %q", line)
	}
}

func TestRenderCapacityLine_sameDir(t *testing.T) {
	line := RenderCapacityLine(1, 3, SpawnModeSameDir)
	if !containsPlain(line, "Capacity: 1/3") {
		t.Errorf("missing 'Capacity: 1/3': %q", line)
	}
	if !containsPlain(line, "New sessions will be created in the current directory") {
		t.Errorf("missing same-dir hint: %q", line)
	}
}

func TestRenderSingleModeCapacityLine_singleSession(t *testing.T) {
	line := RenderSingleModeCapacityLine(0, SpawnModeSingleSession)
	if !containsPlain(line, "Single session") {
		t.Errorf("missing 'Single session': %q", line)
	}
	if !containsPlain(line, "exits when complete") {
		t.Errorf("missing 'exits when complete': %q", line)
	}
}

func TestRenderSingleModeCapacityLine_worktree(t *testing.T) {
	line := RenderSingleModeCapacityLine(1, SpawnModeWorktree)
	if !containsPlain(line, "Capacity: 1/1") {
		t.Errorf("missing 'Capacity: 1/1': %q", line)
	}
	if !containsPlain(line, "New sessions will be created in an isolated worktree") {
		t.Errorf("missing worktree hint: %q", line)
	}
}

func TestRenderSingleModeCapacityLine_sameDir(t *testing.T) {
	line := RenderSingleModeCapacityLine(0, SpawnModeSameDir)
	if !containsPlain(line, "Capacity: 0/1") {
		t.Errorf("missing 'Capacity: 0/1': %q", line)
	}
	if !containsPlain(line, "New sessions will be created in the current directory") {
		t.Errorf("missing same-dir hint: %q", line)
	}
}

// ---------------------------------------------------------------------------
// Footer rendering
// ---------------------------------------------------------------------------

func TestRenderFooter_idle(t *testing.T) {
	url := "https://claude.ai/code?bridge=env123"
	out := RenderFooter(url, true, false, nil)
	if !containsPlain(out, "Code everywhere with the Claude app or "+url) {
		t.Errorf("missing idle footer text: %q", out)
	}
	if !containsPlain(out, "space to show QR code") {
		t.Errorf("missing QR show hint: %q", out)
	}
}

func TestRenderFooter_active(t *testing.T) {
	url := "https://claude.ai/code?bridge=env123"
	out := RenderFooter(url, false, false, nil)
	if !containsPlain(out, "Continue coding in the Claude app or "+url) {
		t.Errorf("missing active footer text: %q", out)
	}
}

func TestRenderFooter_qrVisible(t *testing.T) {
	out := RenderFooter("https://example.com", true, true, nil)
	if !containsPlain(out, "space to hide QR code") {
		t.Errorf("missing QR hide hint: %q", out)
	}
	if containsPlain(out, "space to show QR code") {
		t.Errorf("should not show 'show' when QR visible: %q", out)
	}
}

func TestRenderFooter_qrHidden(t *testing.T) {
	out := RenderFooter("https://example.com", true, false, nil)
	if !containsPlain(out, "space to show QR code") {
		t.Errorf("missing QR show hint: %q", out)
	}
}

func TestRenderFooter_spawnModeToggle(t *testing.T) {
	mode := SpawnModeWorktree
	out := RenderFooter("https://example.com", true, false, &mode)
	if !containsPlain(out, "w to toggle spawn mode") {
		t.Errorf("missing spawn mode toggle hint: %q", out)
	}
}

func TestRenderFooter_noSpawnModeToggle(t *testing.T) {
	out := RenderFooter("https://example.com", true, false, nil)
	if containsPlain(out, "w to toggle spawn mode") {
		t.Errorf("should not show spawn mode toggle when nil: %q", out)
	}
}

// ---------------------------------------------------------------------------
// Banner rendering
// ---------------------------------------------------------------------------

func TestRenderVerboseBanner_singleSession(t *testing.T) {
	cfg := BridgeConfig{SpawnMode: SpawnModeSingleSession}
	out := RenderVerboseBanner("1.0.0", cfg, "env-abc")
	if !containsPlain(out, "Remote Control") {
		t.Errorf("missing 'Remote Control': %q", out)
	}
	if !containsPlain(out, "v1.0.0") {
		t.Errorf("missing version: %q", out)
	}
	if !containsPlain(out, "Environment ID:") {
		t.Errorf("missing environment ID label: %q", out)
	}
	if !containsPlain(out, "env-abc") {
		t.Errorf("missing environment ID value: %q", out)
	}
	// Single-session should NOT show spawn mode or max sessions
	if containsPlain(out, "Spawn mode:") {
		t.Errorf("single-session should not show spawn mode: %q", out)
	}
}

func TestRenderVerboseBanner_worktree(t *testing.T) {
	cfg := BridgeConfig{SpawnMode: SpawnModeWorktree, MaxSessions: 5}
	out := RenderVerboseBanner("2.0.0", cfg, "env-xyz")
	if !containsPlain(out, "Spawn mode:") {
		t.Errorf("missing spawn mode label: %q", out)
	}
	if !containsPlain(out, "worktree") {
		t.Errorf("missing spawn mode value: %q", out)
	}
	if !containsPlain(out, "Max concurrent sessions:") {
		t.Errorf("missing max sessions label: %q", out)
	}
	if !containsPlain(out, "5") {
		t.Errorf("missing max sessions value: %q", out)
	}
}

func TestRenderSandboxLine(t *testing.T) {
	line := RenderSandboxLine()
	if !containsPlain(line, "Sandbox:") {
		t.Errorf("missing 'Sandbox:': %q", line)
	}
	if !containsPlain(line, "Enabled") {
		t.Errorf("missing 'Enabled': %q", line)
	}
}

// ---------------------------------------------------------------------------
// Log line rendering
// ---------------------------------------------------------------------------

func TestRenderLogSessionComplete(t *testing.T) {
	out := RenderLogSessionComplete("12:34:56", "sess-1", "2m30s")
	if !containsPlain(out, "[12:34:56]") {
		t.Errorf("missing timestamp: %q", out)
	}
	if !containsPlain(out, "completed") {
		t.Errorf("missing 'completed': %q", out)
	}
	if !containsPlain(out, "2m30s") {
		t.Errorf("missing duration: %q", out)
	}
	if !containsPlain(out, "sess-1") {
		t.Errorf("missing session ID: %q", out)
	}
}

func TestRenderLogSessionFailed(t *testing.T) {
	out := RenderLogSessionFailed("12:34:56", "sess-2", "timeout")
	if !containsPlain(out, "[12:34:56]") {
		t.Errorf("missing timestamp: %q", out)
	}
	if !containsPlain(out, "failed") {
		t.Errorf("missing 'failed': %q", out)
	}
	if !containsPlain(out, "timeout") {
		t.Errorf("missing error: %q", out)
	}
	if !containsPlain(out, "sess-2") {
		t.Errorf("missing session ID: %q", out)
	}
}

func TestRenderLogSessionStart(t *testing.T) {
	out := RenderLogSessionStart("09:00:00", "sess-3", "Fix the flaky test")
	if !containsPlain(out, "[09:00:00]") {
		t.Errorf("missing timestamp: %q", out)
	}
	if !containsPlain(out, "Session started:") {
		t.Errorf("missing 'Session started:': %q", out)
	}
	if !containsPlain(out, "Fix the flaky test") {
		t.Errorf("missing prompt: %q", out)
	}
	if !containsPlain(out, "sess-3") {
		t.Errorf("missing session ID: %q", out)
	}
}

func TestRenderLogStatus(t *testing.T) {
	out := RenderLogStatus("10:00:00", "something happened")
	if !containsPlain(out, "[10:00:00]") {
		t.Errorf("missing timestamp: %q", out)
	}
	if !containsPlain(out, "something happened") {
		t.Errorf("missing message: %q", out)
	}
}

func TestRenderLogError(t *testing.T) {
	out := RenderLogError("11:00:00", "bad thing")
	if !containsPlain(out, "[11:00:00] Error: bad thing") {
		t.Errorf("missing error format: %q", out)
	}
}

func TestRenderLogReconnected(t *testing.T) {
	out := RenderLogReconnected("12:00:00", "45s")
	if !containsPlain(out, "[12:00:00]") {
		t.Errorf("missing timestamp: %q", out)
	}
	if !containsPlain(out, "Reconnected") {
		t.Errorf("missing 'Reconnected': %q", out)
	}
	if !containsPlain(out, "after 45s") {
		t.Errorf("missing 'after 45s': %q", out)
	}
}

// ---------------------------------------------------------------------------
// Session bullet rendering
// ---------------------------------------------------------------------------

func TestRenderSessionBullet_withTitle(t *testing.T) {
	info := SessionDisplayInfo{Title: "Fix the bug", URL: "https://example.com/sess-1"}
	out := RenderSessionBullet(info)
	if !containsPlain(out, "Fix the bug") {
		t.Errorf("missing title: %q", out)
	}
	// Should contain OSC 8 link
	if !strings.Contains(out, "\x1b]8;;") {
		t.Errorf("missing OSC 8 link: %q", out)
	}
}

func TestRenderSessionBullet_noTitle(t *testing.T) {
	info := SessionDisplayInfo{URL: "https://example.com/sess-1"}
	out := RenderSessionBullet(info)
	if !containsPlain(out, "Attached") {
		t.Errorf("missing fallback 'Attached': %q", out)
	}
}

func TestRenderSessionBullet_withActivity(t *testing.T) {
	act := &SessionActivity{Type: ActivityToolStart, Summary: "Running bash"}
	info := SessionDisplayInfo{Title: "Task", URL: "https://example.com", Activity: act}
	out := RenderSessionBullet(info)
	if !containsPlain(out, "Running bash") {
		t.Errorf("missing activity: %q", out)
	}
}

func TestRenderSessionBullet_resultActivityHidden(t *testing.T) {
	act := &SessionActivity{Type: ActivityResult, Summary: "Done"}
	info := SessionDisplayInfo{Title: "Task", URL: "https://example.com", Activity: act}
	out := RenderSessionBullet(info)
	if containsPlain(out, "Done") {
		t.Errorf("result activity should be hidden: %q", out)
	}
}

func TestRenderSessionBullet_errorActivityHidden(t *testing.T) {
	act := &SessionActivity{Type: ActivityError, Summary: "Oops"}
	info := SessionDisplayInfo{Title: "Task", URL: "https://example.com", Activity: act}
	out := RenderSessionBullet(info)
	if containsPlain(out, "Oops") {
		t.Errorf("error activity should be hidden: %q", out)
	}
}

// ---------------------------------------------------------------------------
// OSC 8 link
// ---------------------------------------------------------------------------

func TestWrapWithOSC8Link(t *testing.T) {
	got := WrapWithOSC8Link("click me", "https://example.com")
	want := "\x1b]8;;https://example.com\x07click me\x1b]8;;\x07"
	if got != want {
		t.Errorf("WrapWithOSC8Link = %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// TruncateString
// ---------------------------------------------------------------------------

func TestTruncateString_short(t *testing.T) {
	if got := TruncateString("hello", 10); got != "hello" {
		t.Errorf("TruncateString = %q, want %q", got, "hello")
	}
}

func TestTruncateString_exact(t *testing.T) {
	if got := TruncateString("hello", 5); got != "hello" {
		t.Errorf("TruncateString = %q, want %q", got, "hello")
	}
}

func TestTruncateString_truncated(t *testing.T) {
	got := TruncateString("hello world!", 8)
	if got != "hello..." {
		t.Errorf("TruncateString = %q, want %q", got, "hello...")
	}
}

func TestTruncateString_veryShort(t *testing.T) {
	got := TruncateString("hello", 2)
	if got != "he" {
		t.Errorf("TruncateString = %q, want %q", got, "he")
	}
}

// ---------------------------------------------------------------------------
// Truncation width constants
// ---------------------------------------------------------------------------

func TestTruncationWidthConstants(t *testing.T) {
	if TruncateWidthMultiSessionTitle != 35 {
		t.Errorf("TruncateWidthMultiSessionTitle = %d, want 35", TruncateWidthMultiSessionTitle)
	}
	if TruncateWidthMainStatusTitle != 40 {
		t.Errorf("TruncateWidthMainStatusTitle = %d, want 40", TruncateWidthMainStatusTitle)
	}
	if TruncateWidthActivityMulti != 40 {
		t.Errorf("TruncateWidthActivityMulti = %d, want 40", TruncateWidthActivityMulti)
	}
	if TruncateWidthActivitySingle != 60 {
		t.Errorf("TruncateWidthActivitySingle = %d, want 60", TruncateWidthActivitySingle)
	}
	if TruncateWidthLogPrompt != 80 {
		t.Errorf("TruncateWidthLogPrompt = %d, want 80", TruncateWidthLogPrompt)
	}
}

// ---------------------------------------------------------------------------
// Helper: strip ANSI sequences for plain-text comparison
// ---------------------------------------------------------------------------

// stripANSI removes ANSI escape sequences (SGR, OSC, CSI) from a string.
func stripANSI(s string) string {
	var b strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' {
			i++
			if i < len(s) {
				switch s[i] {
				case '[': // CSI sequence
					i++
					for i < len(s) && s[i] != 'm' && s[i] != 'J' && s[i] != 'A' && s[i] != 'K' {
						i++
					}
					if i < len(s) {
						i++ // skip terminator
					}
				case ']': // OSC sequence (e.g. OSC 8)
					i++
					for i < len(s) && s[i] != '\x07' {
						// Also check for ST (\x1b\\)
						if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '\\' {
							i += 2
							break
						}
						i++
					}
					if i < len(s) && s[i] == '\x07' {
						i++
					}
				default:
					// Unknown escape — skip the escape char only
				}
			}
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

// containsPlain checks if the plain-text content of s (with ANSI stripped)
// contains the substring sub.
func containsPlain(s, sub string) bool {
	return strings.Contains(stripANSI(s), sub)
}
