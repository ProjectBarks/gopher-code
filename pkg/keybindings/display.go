package keybindings

// Source: keybindings/useShortcutDisplay.ts, keybindings/shortcutFormat.ts

// ShortcutDisplay returns the display text for a configured shortcut action.
// Falls back to the provided fallback string if the action isn't found.
// This is the Go equivalent of the TS useShortcutDisplay React hook.
func ShortcutDisplay(bindings BindingMap, action Action, context Context, fallback string) string {
	ctxBindings, ok := bindings[context]
	if !ok {
		return fallback
	}
	for keystroke, act := range ctxBindings {
		if act == action {
			chord := ParseChord(keystroke)
			return ChordToDisplayString(chord, CurrentPlatform())
		}
	}
	return fallback
}

// ShortcutDisplayFromDefaults uses the default binding map.
func ShortcutDisplayFromDefaults(action Action, context Context, fallback string) string {
	return ShortcutDisplay(DefaultBindingMap(), action, context, fallback)
}
