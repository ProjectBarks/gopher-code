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

func TestStatusLineViewIdle(t *testing.T) {
	config := session.DefaultConfig()
	sess := session.New(config, "/tmp")
	sl := NewStatusLine(sess)
	sl.SetSize(80, 1)
	view := sl.View()
	plain := stripANSI(view.Content)
	// Idle mode shows model name
	if !strings.Contains(plain, sess.Config.Model) {
		t.Errorf("Expected model name in idle status, got %q", plain)
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

func TestStatusLineStreamingShowsEscHint(t *testing.T) {
	sl := NewStatusLine(nil)
	sl.SetSize(80, 1)
	sl.Update(ModeChangeMsg{Mode: ModeStreaming})
	view := sl.View()
	plain := stripANSI(view.Content)
	if !strings.Contains(plain, "esc to interrupt") {
		t.Errorf("Expected 'esc to interrupt' during streaming, got %q", plain)
	}
}

func TestStatusLineToolRunningShowsEscHint(t *testing.T) {
	sl := NewStatusLine(nil)
	sl.SetSize(80, 1)
	sl.Update(ModeChangeMsg{Mode: ModeToolRunning})
	view := sl.View()
	plain := stripANSI(view.Content)
	if !strings.Contains(plain, "esc to interrupt") {
		t.Errorf("Expected 'esc to interrupt' during tool running, got %q", plain)
	}
}

func TestStatusLineBackToIdleShowsModel(t *testing.T) {
	config := session.DefaultConfig()
	config.Model = "test-model"
	sess := session.New(config, "/tmp")
	sl := NewStatusLine(sess)
	sl.SetSize(80, 1)

	// Go streaming then back to idle
	sl.Update(ModeChangeMsg{Mode: ModeStreaming})
	sl.Update(ModeChangeMsg{Mode: ModeIdle})
	view := sl.View()
	plain := stripANSI(view.Content)
	if !strings.Contains(plain, "test-model") {
		t.Errorf("Expected model name after return to idle, got %q", plain)
	}
}

func TestStatusLineTokenUpdate(t *testing.T) {
	config := session.DefaultConfig()
	sess := session.New(config, "/tmp")
	sl := NewStatusLine(sess)
	sl.SetSize(80, 1)
	sl.Update(TokenUpdateMsg{InputTokens: 1500, OutputTokens: 500})
	view := sl.View()
	plain := stripANSI(view.Content)
	if !strings.Contains(plain, "1500") {
		t.Error("Expected token count in idle status")
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
