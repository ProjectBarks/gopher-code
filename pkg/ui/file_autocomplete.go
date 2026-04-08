// T399: File/context suggestion integration for @-mention autocomplete.
// Source: useInputSuggestion.tsx — file path autocomplete triggered by @ prefix.
//
// This file wires pkg/ui/hooks.FileSuggester into the AppModel so it is
// reachable from main(). The FileSuggester provides file path completion
// when the user types @<partial> in the input pane.
package ui

import (
	"strings"

	"github.com/projectbarks/gopher-code/pkg/ui/hooks"
)

// initFileSuggester creates the FileSuggester rooted at the given cwd and
// attaches it to the AppModel. Called from NewAppModel.
func (a *AppModel) initFileSuggester(cwd string) {
	a.fileSuggester = hooks.NewFileSuggester(cwd)
}

// refreshFileAutocomplete detects @-mention partial paths in the input buffer
// and generates file suggestions. Called after each key press alongside
// refreshSlashAutocomplete.
func (a *AppModel) refreshFileAutocomplete() {
	if a.fileSuggester == nil || a.input == nil {
		a.fileSuggestActive = false
		a.fileSuggestions = nil
		return
	}
	text := a.input.Value()
	partial, ok := extractAtPartial(text)
	if !ok {
		a.fileSuggestActive = false
		a.fileSuggestions = nil
		return
	}
	items := a.fileSuggester.GenerateSuggestions(partial, true)
	a.fileSuggestions = items
	a.fileSuggestActive = len(items) > 0
}

// extractAtPartial finds the last @-mention partial in text.
// Returns the partial path after @ and true if found.
// Source: useInputSuggestion.tsx — regex /@([^\s@]*)$/
func extractAtPartial(text string) (string, bool) {
	idx := strings.LastIndex(text, "@")
	if idx < 0 {
		return "", false
	}
	// @ must be at start or preceded by whitespace.
	if idx > 0 && text[idx-1] != ' ' && text[idx-1] != '\t' {
		return "", false
	}
	partial := text[idx+1:]
	// No spaces allowed in the partial.
	if strings.ContainsAny(partial, " \t") {
		return "", false
	}
	return partial, true
}

// renderFileSuggestions returns the file suggestion dropdown lines, or empty
// string if no suggestions are active.
func (a *AppModel) renderFileSuggestions() string {
	if !a.fileSuggestActive || len(a.fileSuggestions) == 0 {
		return ""
	}
	var lines []string
	for _, item := range a.fileSuggestions {
		lines = append(lines, "  "+item.DisplayText)
	}
	return strings.Join(lines, "\n")
}

// FileSuggester returns the file suggester for testing and external access.
func (a *AppModel) FileSuggester() *hooks.FileSuggester {
	return a.fileSuggester
}

// FileSuggestionsActive reports whether file autocomplete is showing.
func (a *AppModel) FileSuggestionsActive() bool {
	return a.fileSuggestActive
}

// FileSuggestions returns the current file suggestion items.
func (a *AppModel) FileSuggestions() []hooks.SuggestionItem {
	return a.fileSuggestions
}
