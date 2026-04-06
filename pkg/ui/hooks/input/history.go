// Package input provides bubbletea-compatible input management hooks ported
// from the TS useArrowKeyHistory, useHistorySearch, usePasteHandler and
// related hooks. In Go these become plain structs with methods that return
// tea.Cmd values, rather than React hooks.
package input

import (
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"
)

// HistoryEntry represents a single history item. The Display field holds the
// full prompt text (including any mode prefix like "!" for bash mode).
type HistoryEntry struct {
	Display string
}

// InputHistory manages arrow-key (up/down) navigation through previous
// prompts. It mirrors the TS useArrowKeyHistory hook:
//   - Stores entries oldest-first in Items
//   - Preserves the current draft when the user first presses Up
//   - Restores the draft when the user navigates back past the newest entry
//   - Supports optional mode filtering (e.g. skip non-bash entries in bash mode)
type InputHistory struct {
	mu sync.Mutex

	// Items holds history entries oldest-first.
	Items []HistoryEntry

	// cursor is the 1-based navigation index. 0 means "not navigating".
	// cursor=1 means the newest entry, cursor=2 the second-newest, etc.
	cursor int

	// draft is the text the user was typing before they started navigating.
	draft string

	// draftSaved tracks whether we already captured a draft for this
	// navigation session (reset when cursor returns to 0).
	draftSaved bool

	// ModeFilter, when non-empty, causes navigation to skip entries whose
	// Display does not start with the expected prefix. For example "!" for
	// bash-mode entries.
	ModeFilter string

	// MaxItems caps the number of stored entries. 0 means unlimited.
	MaxItems int
}

// NewInputHistory returns an empty InputHistory.
func NewInputHistory() *InputHistory {
	return &InputHistory{}
}

// Add appends a non-empty entry to the end of Items (newest). Duplicates of
// the most recent entry are suppressed to avoid cluttering the list.
func (h *InputHistory) Add(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// Suppress consecutive duplicates.
	if len(h.Items) > 0 && h.Items[len(h.Items)-1].Display == text {
		return
	}

	h.Items = append(h.Items, HistoryEntry{Display: text})

	if h.MaxItems > 0 && len(h.Items) > h.MaxItems {
		h.Items = h.Items[len(h.Items)-h.MaxItems:]
	}

	// Reset navigation after adding.
	h.cursor = 0
	h.draftSaved = false
	h.draft = ""
}

// Len returns the number of history entries.
func (h *InputHistory) Len() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.Items)
}

// Cursor returns the current 1-based navigation index (0 = not navigating).
func (h *InputHistory) Cursor() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.cursor
}

// Draft returns the preserved draft text.
func (h *InputHistory) Draft() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.draft
}

// filteredItems returns items that match the current ModeFilter, in
// newest-first order (the order used for navigation). Caller must hold mu.
func (h *InputHistory) filteredItems() []HistoryEntry {
	out := make([]HistoryEntry, 0, len(h.Items))
	for i := len(h.Items) - 1; i >= 0; i-- {
		e := h.Items[i]
		if h.ModeFilter != "" && !strings.HasPrefix(e.Display, h.ModeFilter) {
			continue
		}
		out = append(out, e)
	}
	return out
}

// NavigateUp moves one step back in history. currentInput is the text
// currently in the input field -- it is saved as the draft on the first
// invocation. Returns (newText string, changed bool).
func (h *InputHistory) NavigateUp(currentInput string) (string, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	filtered := h.filteredItems()
	if len(filtered) == 0 {
		return currentInput, false
	}

	if h.cursor == 0 {
		// Save current input as draft before entering history.
		h.draft = currentInput
		h.draftSaved = true
		h.cursor = 1
		return filtered[0].Display, true
	}

	if h.cursor < len(filtered) {
		h.cursor++
		return filtered[h.cursor-1].Display, true
	}

	// Already at the oldest entry -- no change.
	return filtered[h.cursor-1].Display, false
}

// NavigateDown moves one step forward (toward newest) in history. When the
// user passes the newest entry the draft is restored and cursor resets.
// Returns (newText string, changed bool).
func (h *InputHistory) NavigateDown() (string, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.cursor <= 0 {
		return "", false
	}

	if h.cursor > 1 {
		h.cursor--
		filtered := h.filteredItems()
		if h.cursor-1 < len(filtered) {
			return filtered[h.cursor-1].Display, true
		}
		// Edge case: filtered list shrank.
		h.cursor = 0
		h.draftSaved = false
		return h.draft, true
	}

	// cursor == 1: restore draft.
	h.cursor = 0
	draft := h.draft
	h.draft = ""
	h.draftSaved = false
	return draft, true
}

// Reset exits history navigation without restoring the draft. Useful when
// the user submits or the input is cleared externally.
func (h *InputHistory) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.cursor = 0
	h.draft = ""
	h.draftSaved = false
}

// HandleKey processes Up/Down arrow keys and returns (newText, tea.Cmd).
// If the key is not an arrow key or history is empty it returns ("", nil)
// meaning the caller should handle the key normally.
func (h *InputHistory) HandleKey(msg tea.KeyPressMsg, currentInput string) (string, tea.Cmd) {
	switch msg.Code {
	case tea.KeyUp:
		text, changed := h.NavigateUp(currentInput)
		if changed {
			return text, nil
		}
	case tea.KeyDown:
		text, changed := h.NavigateDown()
		if changed {
			return text, nil
		}
	}
	return "", nil
}
