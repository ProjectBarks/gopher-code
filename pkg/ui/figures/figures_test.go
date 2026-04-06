package figures

import (
	"runtime"
	"testing"
)

func TestBlackCirclePlatform(t *testing.T) {
	if runtime.GOOS == "darwin" {
		if BlackCircle != "⏺" {
			t.Errorf("BlackCircle on darwin: got %q, want %q", BlackCircle, "⏺")
		}
	} else {
		if BlackCircle != "●" {
			t.Errorf("BlackCircle on non-darwin: got %q, want %q", BlackCircle, "●")
		}
	}
}

func TestGlyphConstants(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"BulletOperator", BulletOperator, "\u2219"},
		{"TeardropAsterisk", TeardropAsterisk, "\u273B"},
		{"UpArrow", UpArrow, "\u2191"},
		{"DownArrow", DownArrow, "\u2193"},
		{"LightningBolt", LightningBolt, "\u21AF"},
		{"EffortLow", EffortLow, "\u25CB"},
		{"EffortMedium", EffortMedium, "\u25D0"},
		{"EffortHigh", EffortHigh, "\u25CF"},
		{"EffortMax", EffortMax, "\u25C9"},
		{"PlayIcon", PlayIcon, "\u25B6"},
		{"PauseIcon", PauseIcon, "\u23F8"},
		{"RefreshArrow", RefreshArrow, "\u21BB"},
		{"ChannelArrow", ChannelArrow, "\u2190"},
		{"InjectedArrow", InjectedArrow, "\u2192"},
		{"ForkGlyph", ForkGlyph, "\u2442"},
		{"DiamondOpen", DiamondOpen, "\u25C7"},
		{"DiamondFilled", DiamondFilled, "\u25C6"},
		{"ReferenceMark", ReferenceMark, "\u203B"},
		{"FlagIcon", FlagIcon, "\u2691"},
		{"BlockquoteBar", BlockquoteBar, "\u258E"},
		{"HeavyHorizontal", HeavyHorizontal, "\u2501"},
		{"BridgeFailedIndicator", BridgeFailedIndicator, "\u00D7"},
		{"BridgeReadyIndicator", BridgeReadyIndicator, "\u00B7\u2714\uFE0E\u00B7"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("%s = %q, want %q", tt.name, tt.got, tt.want)
			}
		})
	}
}

func TestBridgeSpinnerFrames(t *testing.T) {
	want := []string{
		"\u00B7|\u00B7",
		"\u00B7/\u00B7",
		"\u00B7\u2014\u00B7",
		"\u00B7\\\u00B7",
	}
	if len(BridgeSpinnerFrames) != len(want) {
		t.Fatalf("BridgeSpinnerFrames len = %d, want %d", len(BridgeSpinnerFrames), len(want))
	}
	for i, got := range BridgeSpinnerFrames {
		if got != want[i] {
			t.Errorf("BridgeSpinnerFrames[%d] = %q, want %q", i, got, want[i])
		}
	}
}
