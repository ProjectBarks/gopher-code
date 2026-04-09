package terminal_setup

import (
	"strings"
	"testing"
)

func TestDetectTerminal(t *testing.T) {
	// Just verify it doesn't panic and returns a valid value
	term := DetectTerminal()
	_ = DisplayName(term)
}

func TestDisplayName(t *testing.T) {
	tests := []struct {
		term Terminal
		want string
	}{
		{TerminalApple, "Apple Terminal"},
		{TerminalVSCode, "VS Code"},
		{TerminalCursor, "Cursor"},
		{TerminalGhostty, "Ghostty"},
		{TerminalKitty, "Kitty"},
		{TerminalITerm2, "iTerm2"},
		{TerminalUnknown, "Unknown terminal"},
	}
	for _, tt := range tests {
		if got := DisplayName(tt.term); got != tt.want {
			t.Errorf("DisplayName(%q) = %q, want %q", tt.term, got, tt.want)
		}
	}
}

func TestHasNativeCSIu(t *testing.T) {
	natives := []Terminal{TerminalGhostty, TerminalKitty, TerminalITerm2, TerminalWezTerm, TerminalWarp}
	for _, term := range natives {
		if !HasNativeCSIu(term) {
			t.Errorf("%q should have native CSI u", term)
		}
	}

	nonNatives := []Terminal{TerminalApple, TerminalVSCode, TerminalAlacritty, TerminalUnknown}
	for _, term := range nonNatives {
		if HasNativeCSIu(term) {
			t.Errorf("%q should not have native CSI u", term)
		}
	}
}

func TestNeedsSetup(t *testing.T) {
	if !NeedsSetup(TerminalVSCode) {
		t.Error("VS Code should need setup")
	}
	if !NeedsSetup(TerminalAlacritty) {
		t.Error("Alacritty should need setup")
	}
	if NeedsSetup(TerminalGhostty) {
		t.Error("Ghostty should not need setup")
	}
	if NeedsSetup(TerminalUnknown) {
		t.Error("unknown should not need setup")
	}
}

func TestCheck(t *testing.T) {
	result := Check()
	// Should produce a valid result regardless of environment
	if result.Message == "" {
		t.Error("should have a message")
	}
}

func TestCheck_NativeTerminal(t *testing.T) {
	// Simulate by testing the logic directly
	for term := range NativeCSIuTerminals {
		if !HasNativeCSIu(term) {
			t.Errorf("%q should be native", term)
		}
	}
}

func TestRender(t *testing.T) {
	result := SetupResult{
		Terminal:    TerminalVSCode,
		NeedsSetup: true,
		Message:     "VS Code needs Shift+Enter keybinding",
	}
	got := Render(result)
	if !strings.Contains(got, "Terminal Setup") {
		t.Error("should contain title")
	}
	if !strings.Contains(got, "VS Code") {
		t.Error("should contain terminal name")
	}
	if !strings.Contains(got, "Setup recommended") {
		t.Error("should show setup recommended")
	}
}

func TestRender_Native(t *testing.T) {
	result := SetupResult{
		Terminal: TerminalGhostty,
		IsNative: true,
		Message:  "Native support",
	}
	got := Render(result)
	if !strings.Contains(got, "Native") {
		t.Error("should show native status")
	}
}

func TestRender_Ready(t *testing.T) {
	result := SetupResult{
		Terminal: TerminalUnknown,
		Message:  "No setup required",
	}
	got := Render(result)
	if !strings.Contains(got, "Ready") {
		t.Error("should show ready status")
	}
}

func TestSetupInstructions(t *testing.T) {
	tests := []struct {
		term Terminal
		want string
	}{
		{TerminalApple, "Option key"},
		{TerminalVSCode, "Shift+Enter"},
		{TerminalAlacritty, "alacritty.toml"},
	}
	for _, tt := range tests {
		got := setupInstructions(tt.term)
		if !strings.Contains(got, tt.want) {
			t.Errorf("instructions for %q should contain %q: %q", tt.term, tt.want, got)
		}
	}
}

func TestTerminalConstants(t *testing.T) {
	if TerminalApple != "Apple_Terminal" {
		t.Error("wrong")
	}
	if TerminalVSCode != "vscode" {
		t.Error("wrong")
	}
}

func TestIsVSCodeRemoteSSH(t *testing.T) {
	// On most test machines this should be false
	// Just verify it doesn't panic
	_ = IsVSCodeRemoteSSH()
}
