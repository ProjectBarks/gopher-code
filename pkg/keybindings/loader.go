package keybindings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Source: keybindings/loadUserBindings.ts + KeybindingProviderSetup.tsx
//
// Loads user keybindings from ~/.claude/keybindings.json, merges with defaults,
// validates, and supports hot-reload via file watching.

// ChordTimeoutMs is the timeout for chord sequences. If the user doesn't
// complete the chord within this time, it's cancelled.
const ChordTimeoutMs = 1000

// KeybindingsLoadResult holds parsed bindings and any validation warnings.
type KeybindingsLoadResult struct {
	Bindings []ParsedBinding
	Warnings []Warning
}

// Loader manages keybinding loading, merging, and hot-reload.
type Loader struct {
	mu       sync.RWMutex
	bindings []ParsedBinding
	warnings []Warning
	onChange func(KeybindingsLoadResult) // optional callback for hot-reload
}

// NewLoader creates a loader that immediately loads default + user bindings.
func NewLoader() *Loader {
	l := &Loader{}
	l.Reload()
	return l
}

// Bindings returns the current merged binding set (thread-safe).
func (l *Loader) Bindings() []ParsedBinding {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.bindings
}

// Warnings returns the current validation warnings (thread-safe).
func (l *Loader) Warnings() []Warning {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.warnings
}

// OnChange registers a callback invoked after each reload.
func (l *Loader) OnChange(fn func(KeybindingsLoadResult)) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.onChange = fn
}

// Reload loads default bindings, merges user bindings, and validates.
func (l *Loader) Reload() {
	// Start with defaults
	defaults := parseDefaultBindings()

	// Load and merge user bindings
	userBindings, userWarnings := loadUserBindings()
	merged := mergeBindings(defaults, userBindings)

	// Validate the merged set
	validationWarnings := ValidateBindings(merged)
	allWarnings := append(userWarnings, validationWarnings...)

	l.mu.Lock()
	l.bindings = merged
	l.warnings = allWarnings
	cb := l.onChange
	l.mu.Unlock()

	if cb != nil {
		cb(KeybindingsLoadResult{Bindings: merged, Warnings: allWarnings})
	}
}

// parseDefaultBindings converts the DefaultBindingMap to ParsedBinding slice.
func parseDefaultBindings() []ParsedBinding {
	dm := DefaultBindingMap()
	var bindings []ParsedBinding
	for ctx, ctxBindings := range dm {
		for keystroke, action := range ctxBindings {
			chord := ParseChord(string(keystroke))
			bindings = append(bindings, ParsedBinding{
				Chord:   chord,
				Action:  string(action),
				Context: ctx,
			})
		}
	}
	return bindings
}

// keybindingsFilePath returns ~/.claude/keybindings.json.
func keybindingsFilePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "keybindings.json")
}

// loadUserBindings reads and parses ~/.claude/keybindings.json.
// Returns empty bindings + warnings on any error.
func loadUserBindings() ([]ParsedBinding, []Warning) {
	path := keybindingsFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No user config — fine
		}
		return nil, []Warning{{
			Type:     WarnParseError,
			Severity: SeverityError,
			Message:  "Could not read keybindings.json: " + err.Error(),
		}}
	}

	// Parse JSON array of blocks (user file is []KeybindingBlock)
	var blocks []KeybindingBlock
	if err := json.Unmarshal(data, &blocks); err != nil {
		return nil, []Warning{{
			Type:       WarnParseError,
			Severity:   SeverityError,
			Message:    "keybindings.json: " + err.Error(),
			Suggestion: "keybindings.json must contain a JSON array",
		}}
	}

	var bindings []ParsedBinding
	var warnings []Warning

	for _, block := range blocks {
		if w := ValidateContext(string(block.Context)); w != nil {
			warnings = append(warnings, *w)
			continue
		}
		for keystroke, action := range block.Bindings {
			if w := ValidateKeystroke(keystroke); w != nil {
				w.Context = string(block.Context)
				warnings = append(warnings, *w)
				continue
			}
			chord := ParseChord(keystroke)
			bindings = append(bindings, ParsedBinding{
				Chord:   chord,
				Action:  action,
				Context: block.Context,
			})
		}
	}

	return bindings, warnings
}

// mergeBindings combines defaults and user bindings. User bindings come last
// so they override defaults (last-wins semantics in the resolver).
func mergeBindings(defaults, user []ParsedBinding) []ParsedBinding {
	result := make([]ParsedBinding, 0, len(defaults)+len(user))
	result = append(result, defaults...)
	result = append(result, user...)
	return result
}

// Note: KeybindingsFile is defined in schema.go, ParseChord in parser.go
