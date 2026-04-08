package bridge

import (
	"testing"

	"github.com/projectbarks/gopher-code/pkg/ui/figures"
)

// TestBridgeUI_UsesFiguresConstants verifies that the bridge UI rendering
// delegates to pkg/ui/figures for its glyph constants, ensuring the figures
// package is wired into the binary through the bridge → figures import chain.
func TestBridgeUI_UsesFiguresConstants(t *testing.T) {
	// BridgeReadyIndicator in bridge/ui.go must equal figures.BridgeReadyIndicator.
	if BridgeReadyIndicator != figures.BridgeReadyIndicator {
		t.Errorf("BridgeReadyIndicator = %q, want figures.BridgeReadyIndicator = %q",
			BridgeReadyIndicator, figures.BridgeReadyIndicator)
	}

	// BridgeFailedIndicator in bridge/ui.go must equal figures.BridgeFailedIndicator.
	if BridgeFailedIndicator != figures.BridgeFailedIndicator {
		t.Errorf("BridgeFailedIndicator = %q, want figures.BridgeFailedIndicator = %q",
			BridgeFailedIndicator, figures.BridgeFailedIndicator)
	}

	// BridgeSpinnerFrames must match the figures array exactly.
	if len(BridgeSpinnerFrames) != len(figures.BridgeSpinnerFrames) {
		t.Fatalf("BridgeSpinnerFrames len = %d, want %d",
			len(BridgeSpinnerFrames), len(figures.BridgeSpinnerFrames))
	}
	for i := range BridgeSpinnerFrames {
		if BridgeSpinnerFrames[i] != figures.BridgeSpinnerFrames[i] {
			t.Errorf("BridgeSpinnerFrames[%d] = %q, want %q",
				i, BridgeSpinnerFrames[i], figures.BridgeSpinnerFrames[i])
		}
	}

	// Render a status line and verify it contains the figures-sourced indicator.
	idle := RenderIdleStatusLine("repo", "main", SpawnModeSingleSession)
	if !containsPlain(idle, figures.BridgeReadyIndicator) {
		t.Errorf("RenderIdleStatusLine does not contain figures.BridgeReadyIndicator: %q", idle)
	}

	failed := RenderFailedStatusLine("err", "", "")
	if !containsPlain(failed, figures.BridgeFailedIndicator) {
		t.Errorf("RenderFailedStatusLine does not contain figures.BridgeFailedIndicator: %q", failed)
	}
}
