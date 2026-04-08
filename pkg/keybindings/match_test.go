package keybindings

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestGetKeyName(t *testing.T) {
	tests := []struct {
		name string
		msg  tea.KeyPressMsg
		want string
	}{
		{"escape", tea.KeyPressMsg{Code: tea.KeyEscape}, "escape"},
		{"enter", tea.KeyPressMsg{Code: tea.KeyEnter}, "enter"},
		{"tab", tea.KeyPressMsg{Code: tea.KeyTab}, "tab"},
		{"up arrow", tea.KeyPressMsg{Code: tea.KeyUp}, "up"},
		{"down arrow", tea.KeyPressMsg{Code: tea.KeyDown}, "down"},
		{"letter a", tea.KeyPressMsg{Code: 'a', Text: "a"}, "a"},
		{"letter Z text", tea.KeyPressMsg{Text: "Z"}, "z"},
		{"digit 5", tea.KeyPressMsg{Code: '5', Text: "5"}, "5"},
		{"space", tea.KeyPressMsg{Code: tea.KeySpace}, "space"},
		{"backspace", tea.KeyPressMsg{Code: tea.KeyBackspace}, "backspace"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetKeyName(tt.msg)
			if got != tt.want {
				t.Errorf("GetKeyName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMatchesKeystroke_CtrlC(t *testing.T) {
	msg := tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	target := ParseKeystroke("ctrl+c")

	if !MatchesKeystroke(msg, target) {
		t.Error("ctrl+c should match")
	}

	// Plain 'c' should not match ctrl+c binding
	plain := tea.KeyPressMsg{Code: 'c', Text: "c"}
	if MatchesKeystroke(plain, target) {
		t.Error("plain c should not match ctrl+c")
	}
}

func TestMatchesKeystroke_AltF(t *testing.T) {
	msg := tea.KeyPressMsg{Code: 'f', Mod: tea.ModAlt, Text: "f"}
	target := ParseKeystroke("alt+f")

	if !MatchesKeystroke(msg, target) {
		t.Error("alt+f should match")
	}

	// Meta+f should also match alt+f (they're aliases in terminals)
	targetMeta := ParseKeystroke("meta+f")
	if !MatchesKeystroke(msg, targetMeta) {
		t.Error("alt key should match meta+f binding")
	}
}

func TestMatchesKeystroke_EscapeIgnoresAlt(t *testing.T) {
	// Some terminals send escape with alt modifier set
	msg := tea.KeyPressMsg{Code: tea.KeyEscape, Mod: tea.ModAlt}
	target := ParseKeystroke("escape")

	if !MatchesKeystroke(msg, target) {
		t.Error("escape with alt flag should still match bare escape binding")
	}
}

func TestMatchesBinding_SingleKeystroke(t *testing.T) {
	binding := ParsedBinding{
		Chord:   Chord{ParseKeystroke("ctrl+t")},
		Action:  "app:tasks",
		Context: "Global",
	}

	msg := tea.KeyPressMsg{Code: 't', Mod: tea.ModCtrl}
	if !MatchesBinding(msg, binding) {
		t.Error("ctrl+t should match")
	}

	// Chord bindings should not match (only single-keystroke)
	chordBinding := ParsedBinding{
		Chord:   Chord{ParseKeystroke("ctrl+k"), ParseKeystroke("ctrl+s")},
		Action:  "app:save",
		Context: "Global",
	}
	if MatchesBinding(msg, chordBinding) {
		t.Error("single key should not match chord binding")
	}
}

func TestFindMatchingAction(t *testing.T) {
	bindings := []ParsedBinding{
		{Chord: Chord{ParseKeystroke("ctrl+t")}, Action: "app:tasks", Context: "Global"},
		{Chord: Chord{ParseKeystroke("ctrl+o")}, Action: "app:transcript", Context: "Global"},
		{Chord: Chord{ParseKeystroke("escape")}, Action: "theme:cancel", Context: "ThemePicker"},
	}

	// ctrl+t with Global active → app:tasks
	msg := tea.KeyPressMsg{Code: 't', Mod: tea.ModCtrl}
	action := FindMatchingAction(msg, bindings, []Context{"Global"})
	if action != "app:tasks" {
		t.Errorf("expected app:tasks, got %q", action)
	}

	// escape with only Global active → no match (theme:cancel is ThemePicker only)
	escMsg := tea.KeyPressMsg{Code: tea.KeyEscape}
	action = FindMatchingAction(escMsg, bindings, []Context{"Global"})
	if action != "" {
		t.Errorf("expected no match, got %q", action)
	}

	// escape with ThemePicker active → theme:cancel
	action = FindMatchingAction(escMsg, bindings, []Context{"Global", "ThemePicker"})
	if action != "theme:cancel" {
		t.Errorf("expected theme:cancel, got %q", action)
	}
}
