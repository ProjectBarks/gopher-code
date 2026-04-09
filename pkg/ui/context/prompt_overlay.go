package context

// Source: context/promptOverlayContext.tsx
//
// In TS, PromptOverlayContext is a React context that portals content above
// the prompt to escape FullscreenLayout's overflow:hidden clip. Components
// write suggestion data or dialog nodes; FullscreenLayout reads and renders
// them outside the clipped slot.
//
// In Go/bubbletea, these become struct fields on the app model. The layout
// View() reads them directly — no context providers needed.

// SuggestionItem represents a single autocomplete suggestion.
type SuggestionItem struct {
	// Label is the display text (e.g., "/help", "@file.go").
	Label string
	// Description is optional help text shown alongside the label.
	Description string
	// Value is the actual value inserted on selection (may differ from label).
	Value string
	// Type categorizes the suggestion (e.g., "command", "file", "mention").
	Type string
}

// PromptOverlay holds the floating overlay state above the prompt input.
// Two channels matching the TS implementation:
//   - Suggestions: structured slash-command/autocomplete suggestions
//   - Dialog: arbitrary dialog content (e.g., AutoModeOptInDialog)
type PromptOverlay struct {
	// Suggestions is the current autocomplete suggestion list, or nil.
	Suggestions []SuggestionItem
	// SelectedIndex is the currently highlighted suggestion (-1 = none).
	SelectedIndex int
	// MaxColumnWidth caps the suggestion popup width (0 = no cap).
	MaxColumnWidth int
	// Dialog is an arbitrary dialog string rendered above the prompt.
	// Empty string means no dialog.
	Dialog string
}

// HasSuggestions returns true if there are suggestions to display.
func (o *PromptOverlay) HasSuggestions() bool {
	return len(o.Suggestions) > 0
}

// HasDialog returns true if there is a dialog to display.
func (o *PromptOverlay) HasDialog() bool {
	return o.Dialog != ""
}

// IsActive returns true if any overlay content is present.
func (o *PromptOverlay) IsActive() bool {
	return o.HasSuggestions() || o.HasDialog()
}

// SetSuggestions updates the suggestion list and selection.
func (o *PromptOverlay) SetSuggestions(items []SuggestionItem, selected int) {
	o.Suggestions = items
	o.SelectedIndex = selected
}

// ClearSuggestions removes all suggestions.
func (o *PromptOverlay) ClearSuggestions() {
	o.Suggestions = nil
	o.SelectedIndex = -1
}

// SetDialog sets the dialog content.
func (o *PromptOverlay) SetDialog(content string) {
	o.Dialog = content
}

// ClearDialog removes the dialog.
func (o *PromptOverlay) ClearDialog() {
	o.Dialog = ""
}

// Clear removes all overlay content.
func (o *PromptOverlay) Clear() {
	o.ClearSuggestions()
	o.ClearDialog()
}

// SelectedSuggestion returns the currently selected suggestion, or nil.
func (o *PromptOverlay) SelectedSuggestion() *SuggestionItem {
	if o.SelectedIndex < 0 || o.SelectedIndex >= len(o.Suggestions) {
		return nil
	}
	return &o.Suggestions[o.SelectedIndex]
}

// MoveSelection moves the selection up or down, wrapping around.
func (o *PromptOverlay) MoveSelection(delta int) {
	n := len(o.Suggestions)
	if n == 0 {
		return
	}
	o.SelectedIndex = ((o.SelectedIndex + delta) % n + n) % n
}
