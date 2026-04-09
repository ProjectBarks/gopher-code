package ui

import (
	"bytes"
	"strings"
	"testing"
)

func TestTerminalNotifier_NotifyITerm2(t *testing.T) {
	var buf bytes.Buffer
	n := &TerminalNotifier{w: &buf}

	n.NotifyITerm2("hello world", "")
	got := buf.String()
	// Should contain OSC 9 prefix
	if !strings.Contains(got, "\x1b]9;") {
		t.Errorf("expected OSC 9 sequence, got %q", got)
	}
	if !strings.Contains(got, "hello world") {
		t.Errorf("should contain message, got %q", got)
	}

	buf.Reset()
	n.NotifyITerm2("body", "Title")
	got = buf.String()
	if !strings.Contains(got, "Title:\nbody") {
		t.Errorf("should contain title:body, got %q", got)
	}
}

func TestTerminalNotifier_NotifyKitty(t *testing.T) {
	var buf bytes.Buffer
	n := &TerminalNotifier{w: &buf, isKitty: true}

	n.NotifyKitty("message body", "My Title", 42)
	got := buf.String()
	// Kitty uses OSC 99 and ST terminator
	if !strings.Contains(got, "\x1b]99;") {
		t.Errorf("expected OSC 99 sequence, got %q", got)
	}
	if !strings.Contains(got, "i=42") {
		t.Errorf("should contain notification id, got %q", got)
	}
	if !strings.Contains(got, "My Title") {
		t.Errorf("should contain title, got %q", got)
	}
	// Kitty uses ST terminator
	if !strings.Contains(got, "\x1b\\") {
		t.Errorf("kitty should use ST terminator, got %q", got)
	}
}

func TestTerminalNotifier_NotifyGhostty(t *testing.T) {
	var buf bytes.Buffer
	n := &TerminalNotifier{w: &buf}

	n.NotifyGhostty("notification body", "Alert")
	got := buf.String()
	if !strings.Contains(got, "\x1b]777;") {
		t.Errorf("expected OSC 777 sequence, got %q", got)
	}
	if !strings.Contains(got, "notify") {
		t.Errorf("should contain 'notify', got %q", got)
	}
	if !strings.Contains(got, "Alert") {
		t.Errorf("should contain title, got %q", got)
	}
}

func TestTerminalNotifier_NotifyBell(t *testing.T) {
	var buf bytes.Buffer
	n := &TerminalNotifier{w: &buf}

	n.NotifyBell()
	if buf.String() != "\x07" {
		t.Errorf("expected BEL, got %q", buf.String())
	}
}

func TestTerminalNotifier_Progress(t *testing.T) {
	t.Run("running", func(t *testing.T) {
		var buf bytes.Buffer
		n := &TerminalNotifier{w: &buf}
		n.Progress(ProgressRunning, 50)
		got := buf.String()
		// OSC 9;4;1;50
		if !strings.Contains(got, "\x1b]9;4;1;50") {
			t.Errorf("expected progress set sequence, got %q", got)
		}
	})

	t.Run("clear", func(t *testing.T) {
		var buf bytes.Buffer
		n := &TerminalNotifier{w: &buf}
		n.ClearProgress()
		got := buf.String()
		if !strings.Contains(got, "\x1b]9;4;0;") {
			t.Errorf("expected progress clear, got %q", got)
		}
	})

	t.Run("error", func(t *testing.T) {
		var buf bytes.Buffer
		n := &TerminalNotifier{w: &buf}
		n.Progress(ProgressError, 75)
		got := buf.String()
		if !strings.Contains(got, "\x1b]9;4;2;75") {
			t.Errorf("expected progress error, got %q", got)
		}
	})

	t.Run("indeterminate", func(t *testing.T) {
		var buf bytes.Buffer
		n := &TerminalNotifier{w: &buf}
		n.Progress(ProgressIndeterminate, 0)
		got := buf.String()
		if !strings.Contains(got, "\x1b]9;4;3;") {
			t.Errorf("expected progress indeterminate, got %q", got)
		}
	})

	t.Run("clamped", func(t *testing.T) {
		var buf bytes.Buffer
		n := &TerminalNotifier{w: &buf}
		n.Progress(ProgressRunning, 200)
		got := buf.String()
		// Percentage should be clamped to 100
		if !strings.Contains(got, ";100") {
			t.Errorf("percentage should be clamped to 100, got %q", got)
		}
	})
}

func TestTerminalNotifier_SetTitle(t *testing.T) {
	var buf bytes.Buffer
	n := &TerminalNotifier{w: &buf}

	n.SetTitle("my window")
	got := buf.String()
	if !strings.Contains(got, "\x1b]0;my window") {
		t.Errorf("expected OSC 0 title sequence, got %q", got)
	}
}

func TestTerminalNotifier_ClearTitle(t *testing.T) {
	var buf bytes.Buffer
	n := &TerminalNotifier{w: &buf}

	n.ClearTitle()
	got := buf.String()
	if !strings.Contains(got, "\x1b]0;") {
		t.Errorf("expected OSC 0 clear, got %q", got)
	}
}

func TestTerminalNotifier_TmuxWrapping(t *testing.T) {
	var buf bytes.Buffer
	n := &TerminalNotifier{w: &buf, isTmux: true}

	n.NotifyITerm2("test", "")
	got := buf.String()
	// Should be wrapped in DCS passthrough: ESC P tmux; ... ESC \
	if !strings.HasPrefix(got, "\x1bPtmux;") {
		t.Errorf("should start with tmux DCS passthrough, got %q", got)
	}
	if !strings.HasSuffix(got, "\x1b\\") {
		t.Errorf("should end with ST, got %q", got)
	}
	// ESC inside should be double-escaped
	if !strings.Contains(got, "\x1b\x1b]") {
		t.Errorf("should contain double-escaped OSC, got %q", got)
	}
}

func TestTerminalNotifier_BellNotWrappedInTmux(t *testing.T) {
	var buf bytes.Buffer
	n := &TerminalNotifier{w: &buf, isTmux: true}

	n.NotifyBell()
	// BEL should NOT be wrapped in tmux DCS
	if buf.String() != "\x07" {
		t.Errorf("BEL should not be wrapped, got %q", buf.String())
	}
}

func TestNewTerminalNotifier(t *testing.T) {
	var buf bytes.Buffer
	n := NewTerminalNotifier(&buf)
	if n == nil {
		t.Fatal("should not be nil")
	}
	if n.w != &buf {
		t.Error("writer should be set")
	}
}
