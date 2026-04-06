package keybindings

import (
	"runtime"
	"strings"
)

// ParsedKeystroke represents a single parsed key combination with modifiers.
type ParsedKeystroke struct {
	Key   string
	Ctrl  bool
	Alt   bool
	Shift bool
	Meta  bool
	Super bool
}

// Chord is a sequence of keystrokes (e.g. "ctrl+k ctrl+s" is a chord of 2).
type Chord []ParsedKeystroke

// Platform controls modifier display names.
type Platform string

const (
	PlatformMacOS   Platform = "macos"
	PlatformLinux   Platform = "linux"
	PlatformWindows Platform = "windows"
	PlatformWSL     Platform = "wsl"
	PlatformUnknown Platform = "unknown"
)

// ParsedBinding is a flattened binding: a chord, an action name, and the context.
type ParsedBinding struct {
	Chord   Chord
	Action  string
	Context string
}

// ParseKeystroke parses a string like "ctrl+shift+k" into a ParsedKeystroke.
// Supports aliases: ctrl/control, alt/opt/option, meta, cmd/command/super/win,
// esc, return, space, and arrow glyphs.
func ParseKeystroke(input string) ParsedKeystroke {
	parts := strings.Split(input, "+")
	ks := ParsedKeystroke{}
	for _, part := range parts {
		switch strings.ToLower(part) {
		case "ctrl", "control":
			ks.Ctrl = true
		case "alt", "opt", "option":
			ks.Alt = true
		case "shift":
			ks.Shift = true
		case "meta":
			ks.Meta = true
		case "cmd", "command", "super", "win":
			ks.Super = true
		case "esc":
			ks.Key = "escape"
		case "return":
			ks.Key = "enter"
		case "space":
			ks.Key = " "
		case "\u2191": // ↑
			ks.Key = "up"
		case "\u2193": // ↓
			ks.Key = "down"
		case "\u2190": // ←
			ks.Key = "left"
		case "\u2192": // →
			ks.Key = "right"
		default:
			ks.Key = strings.ToLower(part)
		}
	}
	return ks
}

// ParseChord parses a chord string like "ctrl+k ctrl+s" into a Chord.
// A lone space " " is the space-key binding, not a separator.
func ParseChord(input string) Chord {
	if input == " " {
		return Chord{ParseKeystroke("space")}
	}
	fields := strings.Fields(strings.TrimSpace(input))
	chord := make(Chord, len(fields))
	for i, f := range fields {
		chord[i] = ParseKeystroke(f)
	}
	return chord
}

// KeystrokeToString returns the canonical string for a keystroke,
// e.g. "ctrl+alt+shift+meta+cmd+k".
func KeystrokeToString(ks ParsedKeystroke) string {
	parts := make([]string, 0, 6)
	if ks.Ctrl {
		parts = append(parts, "ctrl")
	}
	if ks.Alt {
		parts = append(parts, "alt")
	}
	if ks.Shift {
		parts = append(parts, "shift")
	}
	if ks.Meta {
		parts = append(parts, "meta")
	}
	if ks.Super {
		parts = append(parts, "cmd")
	}
	parts = append(parts, keyDisplayName(ks.Key))
	return strings.Join(parts, "+")
}

// ChordToString returns the canonical string for a chord.
func ChordToString(chord Chord) string {
	strs := make([]string, len(chord))
	for i, ks := range chord {
		strs[i] = KeystrokeToString(ks)
	}
	return strings.Join(strs, " ")
}

// KeystrokeToDisplayString returns a platform-appropriate display string.
// On macOS alt/meta become "opt" and super becomes "cmd".
// On other platforms alt/meta become "alt" and super becomes "super".
func KeystrokeToDisplayString(ks ParsedKeystroke, platform Platform) string {
	parts := make([]string, 0, 6)
	if ks.Ctrl {
		parts = append(parts, "ctrl")
	}
	if ks.Alt || ks.Meta {
		if platform == PlatformMacOS {
			parts = append(parts, "opt")
		} else {
			parts = append(parts, "alt")
		}
	}
	if ks.Shift {
		parts = append(parts, "shift")
	}
	if ks.Super {
		if platform == PlatformMacOS {
			parts = append(parts, "cmd")
		} else {
			parts = append(parts, "super")
		}
	}
	parts = append(parts, keyDisplayName(ks.Key))
	return strings.Join(parts, "+")
}

// ChordToDisplayString returns a platform-appropriate display string for a chord.
func ChordToDisplayString(chord Chord, platform Platform) string {
	strs := make([]string, len(chord))
	for i, ks := range chord {
		strs[i] = KeystrokeToDisplayString(ks, platform)
	}
	return strings.Join(strs, " ")
}

// ParseBindings flattens keybinding blocks into a list of ParsedBindings.
func ParseBindings(blocks []KeybindingBlock) []ParsedBinding {
	var out []ParsedBinding
	for _, block := range blocks {
		for key, action := range block.Bindings {
			out = append(out, ParsedBinding{
				Chord:   ParseChord(key),
				Action:  action,
				Context: string(block.Context),
			})
		}
	}
	return out
}

// CurrentPlatform returns the Platform for the running OS.
func CurrentPlatform() Platform {
	switch runtime.GOOS {
	case "darwin":
		return PlatformMacOS
	case "windows":
		return PlatformWindows
	default:
		return PlatformLinux
	}
}

// keyDisplayName maps internal key names to human-readable display names.
func keyDisplayName(key string) string {
	switch key {
	case "escape":
		return "Esc"
	case " ":
		return "Space"
	case "tab":
		return "tab"
	case "enter":
		return "Enter"
	case "backspace":
		return "Backspace"
	case "delete":
		return "Delete"
	case "up":
		return "\u2191"
	case "down":
		return "\u2193"
	case "left":
		return "\u2190"
	case "right":
		return "\u2192"
	case "pageup":
		return "PageUp"
	case "pagedown":
		return "PageDown"
	case "home":
		return "Home"
	case "end":
		return "End"
	default:
		return key
	}
}
