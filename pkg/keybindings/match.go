package keybindings

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

// Source: keybindings/match.ts
//
// Maps bubbletea's tea.KeyPressMsg to our ParsedKeystroke and checks for
// matches. In TS this maps from Ink's Key object; in Go from bubbletea's
// KeyPressMsg (Code, Mod, Text fields).

// GetKeyName extracts a normalized key name from a bubbletea KeyPressMsg.
// Returns "" if the key can't be mapped (e.g. bare modifier press).
// Source: match.ts:29-47
func GetKeyName(msg tea.KeyPressMsg) string {
	switch msg.Code {
	case tea.KeyEscape:
		return "escape"
	case tea.KeyEnter:
		return "enter"
	case tea.KeyTab:
		return "tab"
	case tea.KeyBackspace:
		return "backspace"
	case tea.KeyDelete:
		return "delete"
	case tea.KeyUp:
		return "up"
	case tea.KeyDown:
		return "down"
	case tea.KeyLeft:
		return "left"
	case tea.KeyRight:
		return "right"
	case tea.KeyPgUp:
		return "pageup"
	case tea.KeyPgDown:
		return "pagedown"
	case tea.KeyHome:
		return "home"
	case tea.KeyEnd:
		return "end"
	case tea.KeySpace:
		return "space"
	}

	// Single printable character
	if msg.Text != "" && len([]rune(msg.Text)) == 1 {
		return strings.ToLower(msg.Text)
	}

	// Letter keys via Code (ctrl+c comes as Code='c' with Mod=ModCtrl)
	if msg.Code >= 'a' && msg.Code <= 'z' {
		return string(msg.Code)
	}
	if msg.Code >= '0' && msg.Code <= '9' {
		return string(msg.Code)
	}

	return ""
}

// extractModifiers pulls modifier flags from a bubbletea KeyPressMsg.
func extractModifiers(msg tea.KeyPressMsg) (ctrl, shift, alt, super bool) {
	ctrl = msg.Mod&tea.ModCtrl != 0
	shift = msg.Mod&tea.ModShift != 0
	alt = msg.Mod&tea.ModAlt != 0
	super = msg.Mod&tea.ModSuper != 0
	return
}

// MatchesKeystroke checks if a bubbletea key event matches a parsed keystroke.
// Source: match.ts:86-105
func MatchesKeystroke(msg tea.KeyPressMsg, target ParsedKeystroke) bool {
	keyName := GetKeyName(msg)
	if keyName == "" || keyName != target.Key {
		return false
	}

	ctrl, shift, alt, super := extractModifiers(msg)

	// Check ctrl
	if ctrl != target.Ctrl {
		return false
	}

	// Check shift
	if shift != target.Shift {
		return false
	}

	// Alt and meta both map to the alt modifier in terminals.
	// A binding with "alt" or "meta" matches when the terminal sends Alt.
	targetNeedsMeta := target.Alt || target.Meta
	// QUIRK: Escape often arrives with alt flag set in some terminals.
	// Ignore alt when matching the escape key itself.
	if msg.Code == tea.KeyEscape {
		// Skip alt check for escape
	} else if alt != targetNeedsMeta {
		return false
	}

	// Super (cmd/win) — only on kitty protocol terminals
	if super != target.Super {
		return false
	}

	return true
}

// MatchesBinding checks if a key event matches a parsed binding's first keystroke.
// For single-keystroke bindings only (chord support is in the resolver).
// Source: match.ts:111-120
func MatchesBinding(msg tea.KeyPressMsg, binding ParsedBinding) bool {
	if len(binding.Chord) != 1 {
		return false
	}
	return MatchesKeystroke(msg, binding.Chord[0])
}

// FindMatchingAction searches all bindings for the given contexts and returns
// the first matching action. Returns "" if no match.
func FindMatchingAction(msg tea.KeyPressMsg, bindings []ParsedBinding, activeContexts []Context) string {
	activeSet := make(map[Context]bool, len(activeContexts))
	for _, ctx := range activeContexts {
		activeSet[ctx] = true
	}

	for _, b := range bindings {
		if !activeSet[b.Context] {
			continue
		}
		if MatchesBinding(msg, b) {
			return b.Action
		}
	}
	return ""
}
