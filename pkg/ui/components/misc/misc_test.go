package misc

import (
	"strings"
	"testing"
	"time"

	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

func TestEffortSymbol(t *testing.T) {
	if EffortSymbol(EffortLow) != "⚡" {
		t.Error("low")
	}
	if EffortSymbol(EffortMax) != "⚡⚡⚡⚡" {
		t.Error("max")
	}
	if EffortSymbol("") != "" {
		t.Error("empty should be empty")
	}
}

func TestRenderEffortBadge(t *testing.T) {
	got := RenderEffortBadge(EffortHigh)
	if !strings.Contains(got, "high") {
		t.Errorf("should contain 'high': %q", got)
	}
	if RenderEffortBadge("") != "" {
		t.Error("empty should return empty")
	}
}

func TestRenderSessionBackgroundHint(t *testing.T) {
	got := RenderSessionBackgroundHint("my-session")
	if !strings.Contains(got, "my-session") {
		t.Error("should contain session name")
	}
	if !strings.Contains(got, "background") {
		t.Error("should mention background")
	}
}

func TestRenderTurnStats(t *testing.T) {
	stats := TurnStats{
		Duration:     3500 * time.Millisecond,
		InputTokens:  25000,
		OutputTokens: 1500,
		CacheRead:    10000,
		Cost:         0.0035,
	}
	got := RenderTurnStats(stats)
	if !strings.Contains(got, "3.5s") {
		t.Errorf("should contain duration: %q", got)
	}
	if !strings.Contains(got, "25.0K") {
		t.Errorf("should contain input tokens: %q", got)
	}
	if !strings.Contains(got, "1.5K") {
		t.Errorf("should contain output tokens: %q", got)
	}
}

func TestRenderTurnStats_Empty(t *testing.T) {
	got := RenderTurnStats(TurnStats{Duration: 100 * time.Millisecond})
	if !strings.Contains(got, "100ms") {
		t.Errorf("should contain ms duration: %q", got)
	}
}

func TestRenderShortcutHint(t *testing.T) {
	got := RenderShortcutHint("Ctrl+C", "cancel")
	if !strings.Contains(got, "cancel") {
		t.Error("should contain description")
	}
}

func TestRenderShortcutBar(t *testing.T) {
	hints := []ShortcutHint{
		{Key: "Enter", Description: "submit"},
		{Key: "Esc", Description: "cancel"},
	}
	got := RenderShortcutBar(hints)
	if !strings.Contains(got, "submit") {
		t.Error("should contain submit")
	}
	if !strings.Contains(got, "cancel") {
		t.Error("should contain cancel")
	}
}

func TestRenderNotices(t *testing.T) {
	notices := []StatusNotice{
		{ID: "a", Message: "Rate limit approaching", Color: "warning"},
		{ID: "b", Message: "New version available"},
	}
	got := RenderNotices(notices)
	if !strings.Contains(got, "Rate limit approaching") {
		t.Error("should contain first notice")
	}
	if !strings.Contains(got, "New version available") {
		t.Error("should contain second notice")
	}
}

func TestRenderNotices_Empty(t *testing.T) {
	if RenderNotices(nil) != "" {
		t.Error("empty should return empty")
	}
}

func TestRenderRoleBadge(t *testing.T) {
	roles := []string{"human", "assistant", "system", "tool", "unknown"}
	for _, role := range roles {
		got := RenderRoleBadge(role)
		if !strings.Contains(got, role) {
			t.Errorf("badge for %q should contain role: %q", role, got)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{500 * time.Millisecond, "500ms"},
		{3500 * time.Millisecond, "3.5s"},
		{90 * time.Second, "1m30s"},
	}
	for _, tt := range tests {
		got := formatDuration(tt.d)
		if got != tt.want {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestFormatTokens(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{500, "500"},
		{2500, "2.5K"},
		{1500000, "1.5M"},
	}
	for _, tt := range tests {
		got := formatTokens(tt.n)
		if got != tt.want {
			t.Errorf("formatTokens(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestResolveColor(t *testing.T) {
	colors := theme.Current().Colors()
	if resolveColor("warning", colors) != colors.Warning {
		t.Error("should resolve warning")
	}
	if resolveColor("#ff0000", colors) != "#ff0000" {
		t.Error("raw color should pass through")
	}
}
