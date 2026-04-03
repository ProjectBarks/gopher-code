package components

import (
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

func TestWelcomeScreenCreation(t *testing.T) {
	ws := NewWelcomeScreen(theme.Current(), "claude-opus", "/home/user")
	if ws == nil {
		t.Fatal("WelcomeScreen should not be nil")
	}
}

func TestWelcomeScreenView(t *testing.T) {
	ws := NewWelcomeScreen(theme.Current(), "claude-opus", "/home/user/project")
	view := ws.View()
	plain := stripANSI(view.Content)

	if !strings.Contains(plain, "Welcome") {
		t.Error("Expected 'Welcome' in view")
	}
	if !strings.Contains(plain, "Claude Code") {
		t.Error("Expected 'Claude Code' in view")
	}
}

func TestWelcomeScreenShowsVersion(t *testing.T) {
	ws := NewWelcomeScreen(theme.Current(), "model", "/tmp")
	view := ws.View()
	plain := stripANSI(view.Content)
	if !strings.Contains(plain, "v"+Version) {
		t.Errorf("Expected version v%s in view, got:\n%s", Version, plain)
	}
}

func TestWelcomeScreenShowsModel(t *testing.T) {
	ws := NewWelcomeScreen(theme.Current(), "claude-opus-4-6", "/tmp")
	view := ws.View()
	plain := stripANSI(view.Content)
	if !strings.Contains(plain, "claude-opus-4-6") {
		t.Error("Expected model name in view")
	}
}

func TestWelcomeScreenShowsCWD(t *testing.T) {
	ws := NewWelcomeScreen(theme.Current(), "model", "/home/user/project")
	view := ws.View()
	plain := stripANSI(view.Content)
	if !strings.Contains(plain, "project") {
		t.Error("Expected CWD in view")
	}
}

func TestWelcomeScreenShowsTips(t *testing.T) {
	ws := NewWelcomeScreen(theme.Current(), "model", "/tmp")
	view := ws.View()
	plain := stripANSI(view.Content)
	if !strings.Contains(plain, "Tips") {
		t.Error("Expected 'Tips' section in view")
	}
}

func TestWelcomeScreenShowsRecentActivity(t *testing.T) {
	ws := NewWelcomeScreen(theme.Current(), "model", "/tmp")
	view := ws.View()
	plain := stripANSI(view.Content)
	if !strings.Contains(plain, "Recent activity") {
		t.Error("Expected 'Recent activity' section in view")
	}
}

func TestWelcomeScreenHasBorder(t *testing.T) {
	ws := NewWelcomeScreen(theme.Current(), "model", "/tmp")
	view := ws.View()
	// Rounded border uses ╭ and ╮ characters
	if !strings.Contains(view.Content, "╭") && !strings.Contains(view.Content, "┌") {
		t.Error("Expected border characters in view")
	}
}

func TestWelcomeScreenSetSize(t *testing.T) {
	ws := NewWelcomeScreen(theme.Current(), "model", "/tmp")
	ws.SetSize(100, 30)
	// Width should be capped to WelcomeScreenWidth
	if ws.width > WelcomeScreenWidth {
		t.Error("Width should be capped to WelcomeScreenWidth")
	}
}

func TestAbbreviateCWD(t *testing.T) {
	tests := []struct {
		path   string
		maxLen int
	}{
		{"/short", 20},
		{"/Users/alex/very/long/path/to/project", 20},
		{"/Users/alex/project", 30},
	}
	for _, tt := range tests {
		result := abbreviateCWD(tt.path, tt.maxLen)
		runeLen := len([]rune(result))
		if runeLen > tt.maxLen && tt.maxLen > 0 {
			t.Errorf("abbreviateCWD(%q, %d) = %q (rune len %d), exceeds maxLen",
				tt.path, tt.maxLen, result, runeLen)
		}
	}
}

func TestWelcomeScreenHasMascot(t *testing.T) {
	ws := NewWelcomeScreen(theme.Current(), "model", "/tmp")
	view := ws.View()
	// Mascot uses block characters
	if !strings.Contains(view.Content, "░") {
		t.Error("Expected block characters for mascot")
	}
}
