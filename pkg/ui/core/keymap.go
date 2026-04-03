package core

import "strings"

// KeyMap holds a collection of key bindings for user actions.
// It supports customization and lookup by action name.
type KeyMap struct {
	bindings map[string][]string // action -> list of key sequences
}

// NewKeyMap creates a new empty key map.
func NewKeyMap() *KeyMap {
	return &KeyMap{
		bindings: make(map[string][]string),
	}
}

// DefaultKeyMap returns the default key bindings for Gopher.
func DefaultKeyMap() *KeyMap {
	km := NewKeyMap()

	// Navigation
	km.Set("next", "tab")
	km.Set("prev", "shift+tab")

	// Editor
	km.Set("submit", "enter")
	km.Set("escape", "esc")
	km.Set("newline", "ctrl+j")
	km.Set("multiline", "ctrl+m")

	// History
	km.Set("history-up", "up")
	km.Set("history-down", "down")

	// Application
	km.Set("quit", "ctrl+c")
	km.Set("help", "ctrl+h")

	// Modals
	km.Set("approve", "enter")
	km.Set("deny", "n")
	km.Set("always", "a")

	return km
}

// Set assigns key sequences to an action.
// Multiple keys can be assigned to one action.
func (km *KeyMap) Set(action string, keys ...string) {
	km.bindings[action] = keys
}

// Get returns all key sequences bound to an action.
func (km *KeyMap) Get(action string) []string {
	if keys, ok := km.bindings[action]; ok {
		return keys
	}
	return nil
}

// Has checks if an action has key bindings.
func (km *KeyMap) Has(action string) bool {
	_, ok := km.bindings[action]
	return ok
}

// Lookup finds the action for a given key sequence.
// Returns empty string if no action is bound.
func (km *KeyMap) Lookup(key string) string {
	key = strings.ToLower(key)
	for action, keys := range km.bindings {
		for _, k := range keys {
			if strings.ToLower(k) == key {
				return action
			}
		}
	}
	return ""
}

// Actions returns all registered action names.
func (km *KeyMap) Actions() []string {
	actions := make([]string, 0, len(km.bindings))
	for action := range km.bindings {
		actions = append(actions, action)
	}
	return actions
}

// String returns a human-readable representation of the key map.
func (km *KeyMap) String() string {
	var lines []string
	for _, action := range km.Actions() {
		keys := km.Get(action)
		lines = append(lines, action+": "+strings.Join(keys, ", "))
	}
	return strings.Join(lines, "\n")
}
