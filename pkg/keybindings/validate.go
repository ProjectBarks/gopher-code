package keybindings

import (
	"fmt"
	"strings"
)

// Source: keybindings/validate.ts
//
// Validates user keybinding configs: parse errors, duplicate keys,
// reserved shortcut conflicts, invalid contexts/actions.

// WarningSeverity is "error" or "warning".
type WarningSeverity string

const (
	SeverityError   WarningSeverity = "error"
	SeverityWarning WarningSeverity = "warning"
)

// WarningType classifies validation issues.
type WarningType string

const (
	WarnParseError     WarningType = "parse_error"
	WarnDuplicate      WarningType = "duplicate"
	WarnReserved       WarningType = "reserved"
	WarnInvalidContext WarningType = "invalid_context"
	WarnInvalidAction  WarningType = "invalid_action"
)

// Warning describes a keybinding configuration issue.
type Warning struct {
	Type       WarningType
	Severity   WarningSeverity
	Message    string
	Key        string
	Context    string
	Action     string
	Suggestion string
}

// FormatWarning returns a user-visible string for a single warning.
func FormatWarning(w Warning) string {
	icon := "⚠"
	if w.Severity == SeverityError {
		icon = "✗"
	}
	msg := fmt.Sprintf("%s Keybinding %s: %s", icon, w.Severity, w.Message)
	if w.Suggestion != "" {
		msg += "\n  " + w.Suggestion
	}
	return msg
}

// FormatWarnings formats all warnings grouped by severity.
func FormatWarnings(warnings []Warning) string {
	if len(warnings) == 0 {
		return ""
	}

	var errors, warns []Warning
	for _, w := range warnings {
		if w.Severity == SeverityError {
			errors = append(errors, w)
		} else {
			warns = append(warns, w)
		}
	}

	var lines []string
	if len(errors) > 0 {
		lines = append(lines, fmt.Sprintf("Found %d keybinding error(s):", len(errors)))
		for _, e := range errors {
			lines = append(lines, FormatWarning(e))
		}
	}
	if len(warns) > 0 {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, fmt.Sprintf("Found %d keybinding warning(s):", len(warns)))
		for _, w := range warns {
			lines = append(lines, FormatWarning(w))
		}
	}
	return strings.Join(lines, "\n")
}

// ValidateKeystroke checks a single keystroke string for parse errors.
func ValidateKeystroke(keystroke string) *Warning {
	parts := strings.Split(strings.ToLower(keystroke), "+")
	for _, p := range parts {
		if strings.TrimSpace(p) == "" {
			return &Warning{
				Type:       WarnParseError,
				Severity:   SeverityError,
				Message:    fmt.Sprintf("Empty key part in %q", keystroke),
				Key:        keystroke,
				Suggestion: `Remove extra "+" characters`,
			}
		}
	}

	parsed := ParseKeystroke(keystroke)
	if parsed.Key == "" && !parsed.Ctrl && !parsed.Alt && !parsed.Shift && !parsed.Meta {
		return &Warning{
			Type:     WarnParseError,
			Severity: SeverityError,
			Message:  fmt.Sprintf("Could not parse keystroke %q", keystroke),
			Key:      keystroke,
		}
	}
	return nil
}

// ValidateContext checks if a context name is valid.
func ValidateContext(ctx string) *Warning {
	if ValidContext(Context(ctx)) {
		return nil
	}
	return &Warning{
		Type:       WarnInvalidContext,
		Severity:   SeverityError,
		Message:    fmt.Sprintf("Unknown context %q", ctx),
		Context:    ctx,
		Suggestion: fmt.Sprintf("Valid contexts: %s", strings.Join(contextNames(), ", ")),
	}
}

// contextNames returns all valid context names as strings.
func contextNames() []string {
	names := make([]string, len(AllContexts))
	for i, c := range AllContexts {
		names[i] = string(c)
	}
	return names
}

// CheckDuplicates finds duplicate key bindings within the same context.
func CheckDuplicates(bindings []ParsedBinding) []Warning {
	type key struct {
		ctx     Context
		keyStr  string
	}
	seen := make(map[key]string) // key → first action
	var warnings []Warning

	for _, b := range bindings {
		chord := ChordToDisplayString(b.Chord, PlatformLinux) // normalize for comparison
		k := key{ctx: b.Context, keyStr: strings.ToLower(chord)}
		if prev, ok := seen[k]; ok && prev != b.Action {
			warnings = append(warnings, Warning{
				Type:       WarnDuplicate,
				Severity:   SeverityWarning,
				Message:    fmt.Sprintf("Duplicate binding %q in %s context", chord, b.Context),
				Key:        chord,
				Context:    string(b.Context),
				Action:     b.Action,
				Suggestion: fmt.Sprintf("Previously bound to %q. Only the last binding will be used.", prev),
			})
		}
		seen[k] = b.Action
	}
	return warnings
}

// CheckReservedShortcuts warns about bindings that conflict with reserved shortcuts.
func CheckReservedShortcuts(bindings []ParsedBinding) []Warning {
	var warnings []Warning
	for _, b := range bindings {
		chord := ChordToDisplayString(b.Chord, PlatformLinux)
		normalized := strings.ToLower(chord)
		if reason, ok := ReservedShortcuts[normalized]; ok {
			warnings = append(warnings, Warning{
				Type:     WarnReserved,
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("%q may not work: %s", chord, reason),
				Key:      chord,
				Context:  string(b.Context),
				Action:   b.Action,
			})
		}
	}
	return warnings
}

// ValidateBindings runs all validations and returns combined, deduplicated warnings.
func ValidateBindings(bindings []ParsedBinding) []Warning {
	var warnings []Warning
	warnings = append(warnings, CheckDuplicates(bindings)...)
	warnings = append(warnings, CheckReservedShortcuts(bindings)...)

	// Deduplicate by type+key+context
	seen := make(map[string]bool)
	var deduped []Warning
	for _, w := range warnings {
		k := string(w.Type) + ":" + w.Key + ":" + w.Context
		if !seen[k] {
			seen[k] = true
			deduped = append(deduped, w)
		}
	}
	return deduped
}
