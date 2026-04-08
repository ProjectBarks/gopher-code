package keybindings

// Source: keybindings/reserved.ts

// ReservedShortcuts are key combinations that cannot be rebound.
// These are system-level shortcuts that bubbletea/terminal must handle.
var ReservedShortcuts = map[string]string{
	"ctrl+c": "Interrupt / cancel",
	"ctrl+d": "EOF / exit",
	"ctrl+z": "Suspend process",
	"ctrl+\\": "Quit (SIGQUIT)",
}

// IsReserved returns true if the keystroke is a system-reserved shortcut.
func IsReserved(keystroke string) bool {
	_, ok := ReservedShortcuts[keystroke]
	return ok
}
