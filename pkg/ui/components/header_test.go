package components

import (
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

func TestHeaderCreation(t *testing.T) {
	h := NewHeader(theme.Current())
	if h == nil {
		t.Fatal("Header should not be nil")
	}
}

func TestHeaderSetModel(t *testing.T) {
	h := NewHeader(theme.Current())
	h.SetModel("claude-opus-4-6")
	if h.ModelName() != "claude-opus-4-6" {
		t.Errorf("Expected model name, got %q", h.ModelName())
	}
}

func TestHeaderSetSessionName(t *testing.T) {
	h := NewHeader(theme.Current())
	h.SetSessionName("debug-session")
	if h.SessionName() != "debug-session" {
		t.Errorf("Expected session name, got %q", h.SessionName())
	}
}

func TestHeaderSetCWD(t *testing.T) {
	h := NewHeader(theme.Current())
	h.SetCWD("/Users/dev/project")
	if h.CWD() != "/Users/dev/project" {
		t.Errorf("Expected CWD, got %q", h.CWD())
	}
}

func TestHeaderView(t *testing.T) {
	h := NewHeader(theme.Current())
	h.SetModel("opus")
	h.SetSessionName("test")
	h.SetCWD("/tmp")
	h.SetSize(80, 1)
	view := h.View()
	plain := stripANSI(view.Content)
	if !strings.Contains(plain, "Claude") {
		t.Error("Expected Claude logo in header")
	}
	if !strings.Contains(plain, "opus") {
		t.Error("Expected model name in header")
	}
}

func TestHeaderUpdateMsg(t *testing.T) {
	h := NewHeader(theme.Current())
	h.Update(HeaderUpdateMsg{Model: "sonnet", CWD: "/home"})
	if h.ModelName() != "sonnet" {
		t.Error("HeaderUpdateMsg should update model")
	}
	if h.CWD() != "/home" {
		t.Error("HeaderUpdateMsg should update CWD")
	}
}

func TestAbbreviatePath(t *testing.T) {
	tests := []struct {
		path   string
		maxLen int
		ok     bool // whether result should be <= maxLen
	}{
		{"/Users/dev/very/long/path/to/project", 20, true},
		{"/short", 20, true},
		{"", 20, true},
	}
	for _, tt := range tests {
		result := abbreviatePath(tt.path, tt.maxLen)
		if tt.ok && len(result) > tt.maxLen && tt.maxLen > 0 {
			t.Errorf("abbreviatePath(%q, %d) = %q (len %d), exceeds maxLen", tt.path, tt.maxLen, result, len(result))
		}
	}
}
