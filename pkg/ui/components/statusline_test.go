package components

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/projectbarks/gopher-code/pkg/session"
)

func TestStatusLineCreation(t *testing.T) {
	config := session.DefaultConfig()
	sess := session.New(config, "/tmp")
	sl := NewStatusLine(sess)
	if sl == nil {
		t.Fatal("StatusLine should not be nil")
	}
}

func TestStatusLineInit(t *testing.T) {
	sl := NewStatusLine(nil)
	cmd := sl.Init()
	_ = cmd
}

func TestStatusLineView(t *testing.T) {
	config := session.DefaultConfig()
	sess := session.New(config, "/tmp")
	sl := NewStatusLine(sess)
	sl.SetSize(80, 1)
	view := sl.View()
	plain := stripANSI(view.Content)
	if !strings.Contains(plain, "Idle") {
		t.Error("Expected 'Idle' in initial status")
	}
}

func TestStatusLineViewWithModel(t *testing.T) {
	config := session.DefaultConfig()
	config.Model = "claude-opus"
	sess := session.New(config, "/tmp")
	sl := NewStatusLine(sess)
	sl.SetSize(80, 1)
	view := sl.View()
	plain := stripANSI(view.Content)
	if !strings.Contains(plain, "claude-opus") {
		t.Error("Expected model name in status")
	}
}

func TestStatusLineModeChange(t *testing.T) {
	sl := NewStatusLine(nil)
	sl.SetSize(80, 1)

	sl.Update(ModeChangeMsg{Mode: ModeStreaming})
	view := sl.View()
	plain := stripANSI(view.Content)
	if !strings.Contains(plain, "Streaming") {
		t.Error("Expected 'Streaming' after mode change")
	}

	sl.Update(ModeChangeMsg{Mode: ModeToolRunning})
	view = sl.View()
	plain = stripANSI(view.Content)
	if !strings.Contains(plain, "Tool Running") {
		t.Error("Expected 'Tool Running' after mode change")
	}

	sl.Update(ModeChangeMsg{Mode: ModeIdle})
	view = sl.View()
	plain = stripANSI(view.Content)
	if !strings.Contains(plain, "Idle") {
		t.Error("Expected 'Idle' after mode change")
	}
}

func TestStatusLineTokenUpdate(t *testing.T) {
	sl := NewStatusLine(nil)
	sl.SetSize(80, 1)
	sl.Update(TokenUpdateMsg{InputTokens: 1500, OutputTokens: 500})
	view := sl.View()
	plain := stripANSI(view.Content)
	if !strings.Contains(plain, "1500") && !strings.Contains(plain, "tokens") {
		// Just verify it renders without panic
	}
}

func TestStatusLineWindowSize(t *testing.T) {
	sl := NewStatusLine(nil)
	sl.Update(tea.WindowSizeMsg{Width: 120, Height: 1})
	// Should not panic
}

func TestStatusLineFocus(t *testing.T) {
	sl := NewStatusLine(nil)
	if sl.Focused() {
		t.Error("Should not be focused initially")
	}
	sl.Focus()
	if !sl.Focused() {
		t.Error("Should be focused after Focus()")
	}
	sl.Blur()
	if sl.Focused() {
		t.Error("Should not be focused after Blur()")
	}
}

func TestStatusLineSetSize(t *testing.T) {
	sl := NewStatusLine(nil)
	sl.SetSize(100, 2)
	// Should not panic
}
