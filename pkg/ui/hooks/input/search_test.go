package input

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupSearchHistory() (*InputHistory, *HistorySearch) {
	h := NewInputHistory()
	h.Add("git status")
	h.Add("git commit -m 'fix bug'")
	h.Add("go test ./...")
	h.Add("git log --oneline")
	h.Add("echo hello")
	s := NewHistorySearch(h)
	return h, s
}

func TestHistorySearch_StartCancel(t *testing.T) {
	_, s := setupSearchHistory()

	s.Start("my draft", 5)
	assert.True(t, s.Active)

	text, cursor := s.Cancel()
	assert.Equal(t, "my draft", text, "cancel should restore original input")
	assert.Equal(t, 5, cursor, "cancel should restore original cursor")
	assert.False(t, s.Active)
}

func TestHistorySearch_QueryMatches(t *testing.T) {
	_, s := setupSearchHistory()
	s.Start("", 0)

	s.SetQuery("git")

	// Should match newest first.
	m, ok := s.CurrentMatch()
	require.True(t, ok)
	assert.Equal(t, "git log --oneline", m.Entry.Display)
}

func TestHistorySearch_QueryNoMatch(t *testing.T) {
	_, s := setupSearchHistory()
	s.Start("", 0)

	s.SetQuery("zzzznotfound")

	_, ok := s.CurrentMatch()
	assert.False(t, ok)
	assert.True(t, s.FailedMatch)
}

func TestHistorySearch_NextMatch(t *testing.T) {
	_, s := setupSearchHistory()
	s.Start("", 0)

	s.SetQuery("git")

	// First match: git log --oneline (newest).
	m, ok := s.CurrentMatch()
	require.True(t, ok)
	assert.Equal(t, "git log --oneline", m.Entry.Display)

	// Next: git commit.
	found := s.NextMatch()
	require.True(t, found)
	m, ok = s.CurrentMatch()
	require.True(t, ok)
	assert.Equal(t, "git commit -m 'fix bug'", m.Entry.Display)

	// Next: git status.
	found = s.NextMatch()
	require.True(t, found)
	m, ok = s.CurrentMatch()
	require.True(t, ok)
	assert.Equal(t, "git status", m.Entry.Display)

	// No more matches.
	found = s.NextMatch()
	assert.False(t, found)
	assert.True(t, s.FailedMatch)
}

func TestHistorySearch_CaseInsensitive(t *testing.T) {
	h := NewInputHistory()
	h.Add("Git Status")
	h.Add("GIT COMMIT")
	s := NewHistorySearch(h)
	s.Start("", 0)

	s.SetQuery("git")

	m, ok := s.CurrentMatch()
	require.True(t, ok)
	assert.Equal(t, "GIT COMMIT", m.Entry.Display)
}

func TestHistorySearch_MatchHighlightPosition(t *testing.T) {
	h := NewInputHistory()
	h.Add("hello world git test")
	s := NewHistorySearch(h)
	s.Start("", 0)

	s.SetQuery("git")

	m, ok := s.CurrentMatch()
	require.True(t, ok)
	assert.Equal(t, 12, m.MatchStart)
	assert.Equal(t, 15, m.MatchEnd)
	assert.Equal(t, "git", m.Entry.Display[m.MatchStart:m.MatchEnd])
}

func TestHistorySearch_Accept(t *testing.T) {
	_, s := setupSearchHistory()
	s.Start("original", 3)

	s.SetQuery("echo")
	text := s.Accept()
	assert.Equal(t, "echo hello", text)
	assert.False(t, s.Active)
}

func TestHistorySearch_AcceptNoMatch(t *testing.T) {
	_, s := setupSearchHistory()
	s.Start("original", 3)

	s.SetQuery("notfound")
	text := s.Accept()
	assert.Equal(t, "original", text, "accept with no match returns original")
}

func TestHistorySearch_AppendQuery(t *testing.T) {
	_, s := setupSearchHistory()
	s.Start("", 0)

	s.AppendQuery("g")
	s.AppendQuery("o")
	m, ok := s.CurrentMatch()
	require.True(t, ok)
	assert.Equal(t, "go test ./...", m.Entry.Display)
}

func TestHistorySearch_BackspaceQuery(t *testing.T) {
	_, s := setupSearchHistory()
	s.Start("", 0)

	s.SetQuery("go")
	m, ok := s.CurrentMatch()
	require.True(t, ok)
	assert.Equal(t, "go test ./...", m.Entry.Display)

	// Backspace to "g" — now matches git entries too.
	removed := s.BackspaceQuery()
	assert.True(t, removed)
	m, ok = s.CurrentMatch()
	require.True(t, ok)
	// "git log --oneline" is newest match for "g".
	assert.Contains(t, m.Entry.Display, "g")
}

func TestHistorySearch_BackspaceEmptyQuery(t *testing.T) {
	_, s := setupSearchHistory()
	s.Start("", 0)

	removed := s.BackspaceQuery()
	assert.False(t, removed, "backspace on empty query should return false")
}

func TestHistorySearch_EmptyQuery_NoMatches(t *testing.T) {
	_, s := setupSearchHistory()
	s.Start("", 0)

	s.SetQuery("")
	_, ok := s.CurrentMatch()
	assert.False(t, ok, "empty query should have no matches")
	assert.False(t, s.FailedMatch, "empty query is not a failed match")
}

func TestHistorySearch_DeduplicatesResults(t *testing.T) {
	h := NewInputHistory()
	h.Add("cmd1")
	h.Add("cmd2")
	h.Add("cmd1") // non-consecutive duplicate allowed in history
	s := NewHistorySearch(h)
	s.Start("", 0)

	s.SetQuery("cmd1")
	m, ok := s.CurrentMatch()
	require.True(t, ok)
	assert.Equal(t, "cmd1", m.Entry.Display)

	// NextMatch should not find another "cmd1" since it's deduplicated in search.
	found := s.NextMatch()
	assert.False(t, found, "duplicate display should be skipped in search results")
}

// --- HandleKey tests ---

func TestHistorySearch_HandleKey_CtrlR_StartsSearch(t *testing.T) {
	_, s := setupSearchHistory()

	_, cmd, handled := s.HandleKey(tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})
	assert.True(t, handled)
	require.NotNil(t, cmd)
	msg := cmd()
	_, ok := msg.(HistorySearchStartMsg)
	assert.True(t, ok)
}

func TestHistorySearch_HandleKey_CtrlR_NextMatch(t *testing.T) {
	_, s := setupSearchHistory()
	s.Start("", 0)
	s.SetQuery("git")

	text, _, handled := s.HandleKey(tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})
	assert.True(t, handled)
	assert.Equal(t, "git commit -m 'fix bug'", text)
}

func TestHistorySearch_HandleKey_Escape_Cancels(t *testing.T) {
	_, s := setupSearchHistory()
	s.Start("original", 5)
	s.SetQuery("git")

	text, cmd, handled := s.HandleKey(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.True(t, handled)
	assert.Equal(t, "original", text)
	require.NotNil(t, cmd)
	msg := cmd()
	endMsg, ok := msg.(HistorySearchEndMsg)
	require.True(t, ok)
	assert.False(t, endMsg.Accepted)
}

func TestHistorySearch_HandleKey_Enter_Accepts(t *testing.T) {
	_, s := setupSearchHistory()
	s.Start("", 0)
	s.SetQuery("echo")

	text, cmd, handled := s.HandleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.True(t, handled)
	assert.Equal(t, "echo hello", text)
	require.NotNil(t, cmd)
	msg := cmd()
	endMsg, ok := msg.(HistorySearchEndMsg)
	require.True(t, ok)
	assert.True(t, endMsg.Accepted)
	assert.Equal(t, "echo hello", endMsg.Text)
}

func TestHistorySearch_HandleKey_Backspace_CancelsOnEmpty(t *testing.T) {
	_, s := setupSearchHistory()
	s.Start("original", 0)
	// Query is empty.

	text, cmd, handled := s.HandleKey(tea.KeyPressMsg{Code: tea.KeyBackspace})
	assert.True(t, handled)
	assert.Equal(t, "original", text)
	require.NotNil(t, cmd, "should emit cancel message")
}

func TestHistorySearch_HandleKey_Printable(t *testing.T) {
	_, s := setupSearchHistory()
	s.Start("", 0)

	text, _, handled := s.HandleKey(tea.KeyPressMsg{Text: "e"})
	assert.True(t, handled)
	assert.Equal(t, "echo hello", text)
}

func TestHistorySearch_HandleKey_NotActive_NonCtrlR(t *testing.T) {
	_, s := setupSearchHistory()

	_, _, handled := s.HandleKey(tea.KeyPressMsg{Code: 'a'})
	assert.False(t, handled)
}
