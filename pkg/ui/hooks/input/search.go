package input

import (
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"
)

// HistorySearchResult holds the outcome of a fuzzy search against history.
type HistorySearchResult struct {
	Entry      HistoryEntry
	MatchStart int // byte offset of match within Entry.Display
	MatchEnd   int // byte offset past match
}

// HistorySearch implements Ctrl+R style reverse-incremental search against an
// InputHistory. It mirrors the TS useHistorySearch hook.
//
// Workflow:
//  1. User presses Ctrl+R to enter search mode.
//  2. Each keystroke appends to Query; the search finds the most recent entry
//     containing Query (case-insensitive substring).
//  3. Ctrl+R again cycles to the next older match.
//  4. Enter/Tab accepts the match; Escape/Ctrl+G cancels.
//  5. Backspace on an empty query cancels the search.
type HistorySearch struct {
	mu sync.Mutex

	// Active is true when the user is in search mode.
	Active bool

	// Query is the current search string typed by the user.
	Query string

	// matchIndex tracks which match we are on (0 = most recent match).
	matchIndex int

	// matches is the lazily-built list of matching entries in newest-first
	// order, populated by search().
	matches []HistorySearchResult

	// history is the backing InputHistory to search through.
	history *InputHistory

	// originalInput is saved when search starts so it can be restored on cancel.
	originalInput string

	// originalCursor is the cursor position saved when search starts.
	originalCursor int

	// FailedMatch is true when the query has no matches at all.
	FailedMatch bool
}

// NewHistorySearch creates a HistorySearch backed by the given InputHistory.
func NewHistorySearch(h *InputHistory) *HistorySearch {
	return &HistorySearch{history: h}
}

// Start enters search mode, saving the current input so it can be restored
// on cancel.
func (s *HistorySearch) Start(currentInput string, cursorPos int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Active = true
	s.Query = ""
	s.matchIndex = 0
	s.matches = nil
	s.FailedMatch = false
	s.originalInput = currentInput
	s.originalCursor = cursorPos
}

// Cancel exits search mode and returns the original input text + cursor.
func (s *HistorySearch) Cancel() (text string, cursor int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	text = s.originalInput
	cursor = s.originalCursor
	s.reset()
	return
}

// Accept exits search mode and returns the currently matched text. If there
// is no match it returns the original input.
func (s *HistorySearch) Accept() (text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.matches) > 0 && s.matchIndex < len(s.matches) {
		text = s.matches[s.matchIndex].Entry.Display
	} else {
		text = s.originalInput
	}
	s.reset()
	return
}

// reset clears search state. Caller must hold mu.
func (s *HistorySearch) reset() {
	s.Active = false
	s.Query = ""
	s.matchIndex = 0
	s.matches = nil
	s.FailedMatch = false
	s.originalInput = ""
	s.originalCursor = 0
}

// SetQuery replaces the query and recomputes matches from scratch.
func (s *HistorySearch) SetQuery(q string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Query = q
	s.matchIndex = 0
	s.search()
}

// AppendQuery appends a character to the query and recomputes matches.
func (s *HistorySearch) AppendQuery(ch string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Query += ch
	s.matchIndex = 0
	s.search()
}

// BackspaceQuery removes the last character from the query. Returns true if
// the query was non-empty (character removed), false if the query was already
// empty (caller should cancel search).
func (s *HistorySearch) BackspaceQuery() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Query == "" {
		return false
	}
	runes := []rune(s.Query)
	s.Query = string(runes[:len(runes)-1])
	s.matchIndex = 0
	s.search()
	return true
}

// NextMatch cycles to the next older match. Returns true if a new match was
// found.
func (s *HistorySearch) NextMatch() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.matches) == 0 {
		return false
	}
	if s.matchIndex+1 < len(s.matches) {
		s.matchIndex++
		s.FailedMatch = false
		return true
	}
	s.FailedMatch = true
	return false
}

// CurrentMatch returns the current match, if any.
func (s *HistorySearch) CurrentMatch() (HistorySearchResult, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.matches) == 0 || s.matchIndex >= len(s.matches) {
		return HistorySearchResult{}, false
	}
	return s.matches[s.matchIndex], true
}

// search recomputes s.matches from the backing history. It does a
// case-insensitive substring search, matching the TS lastIndexOf behavior.
// Caller must hold mu.
func (s *HistorySearch) search() {
	s.matches = s.matches[:0]
	s.FailedMatch = false

	if s.Query == "" {
		return
	}

	lowerQ := strings.ToLower(s.Query)

	s.history.mu.Lock()
	items := s.history.Items
	s.history.mu.Unlock()

	// Walk newest-first (same as TS).
	seen := make(map[string]struct{})
	for i := len(items) - 1; i >= 0; i-- {
		display := items[i].Display
		if _, dup := seen[display]; dup {
			continue
		}
		seen[display] = struct{}{}

		lower := strings.ToLower(display)
		idx := strings.LastIndex(lower, lowerQ)
		if idx >= 0 {
			s.matches = append(s.matches, HistorySearchResult{
				Entry:      items[i],
				MatchStart: idx,
				MatchEnd:   idx + len(s.Query),
			})
		}
	}

	if len(s.matches) == 0 {
		s.FailedMatch = true
	}
}

// HistorySearchStartMsg is emitted when the user enters search mode.
type HistorySearchStartMsg struct{}

// HistorySearchEndMsg is emitted when the user leaves search mode.
type HistorySearchEndMsg struct {
	Accepted bool
	Text     string
}

// HandleKey processes a key press while search is active and returns
// (displayText string, cmd tea.Cmd, handled bool). If handled is false the
// caller should process the key normally.
func (s *HistorySearch) HandleKey(msg tea.KeyPressMsg) (string, tea.Cmd, bool) {
	if !s.Active {
		// Check for Ctrl+R to start search.
		if msg.Code == 'r' && msg.Mod == tea.ModCtrl {
			return "", func() tea.Msg { return HistorySearchStartMsg{} }, true
		}
		return "", nil, false
	}

	switch {
	// Ctrl+R while searching = next match.
	case msg.Code == 'r' && msg.Mod == tea.ModCtrl:
		s.NextMatch()
		if m, ok := s.CurrentMatch(); ok {
			return m.Entry.Display, nil, true
		}
		return "", nil, true

	// Escape or Ctrl+G = cancel.
	case msg.Code == tea.KeyEscape, (msg.Code == 'g' && msg.Mod == tea.ModCtrl):
		text, _ := s.Cancel()
		return text, func() tea.Msg {
			return HistorySearchEndMsg{Accepted: false, Text: text}
		}, true

	// Enter or Tab = accept.
	case msg.Code == tea.KeyEnter, msg.Code == tea.KeyTab:
		text := s.Accept()
		return text, func() tea.Msg {
			return HistorySearchEndMsg{Accepted: true, Text: text}
		}, true

	// Backspace.
	case msg.Code == tea.KeyBackspace:
		if !s.BackspaceQuery() {
			// Empty query + backspace = cancel.
			text, _ := s.Cancel()
			return text, func() tea.Msg {
				return HistorySearchEndMsg{Accepted: false, Text: text}
			}, true
		}
		if m, ok := s.CurrentMatch(); ok {
			return m.Entry.Display, nil, true
		}
		// Query non-empty but no match after backspace: return original.
		return s.originalInput, nil, true

	// Printable text = append to query.
	default:
		if msg.Text != "" {
			s.AppendQuery(msg.Text)
			if m, ok := s.CurrentMatch(); ok {
				return m.Entry.Display, nil, true
			}
			return "", nil, true
		}
	}

	return "", nil, false
}
