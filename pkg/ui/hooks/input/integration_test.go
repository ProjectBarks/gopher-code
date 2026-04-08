package input

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIntegration_HistorySearchWithHistory exercises the full Ctrl+R search
// workflow through InputHistory and HistorySearch working together, mirroring
// the code path used by InputPane.handleKey → Search.HandleKey.
func TestIntegration_HistorySearchWithHistory(t *testing.T) {
	h := NewInputHistory()
	s := NewHistorySearch(h)

	// Populate history as InputPane.AddToHistory would.
	h.Add("git status")
	h.Add("go test ./...")
	h.Add("git log --oneline")
	h.Add("echo hello world")

	// Step 1: Ctrl+R triggers search start.
	_, cmd, handled := s.HandleKey(tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})
	require.True(t, handled)
	require.NotNil(t, cmd)
	msg := cmd()
	_, ok := msg.(HistorySearchStartMsg)
	require.True(t, ok, "Ctrl+R should produce HistorySearchStartMsg")

	// Step 2: Deliver start — InputPane would call s.Start().
	s.Start("my draft", 8)
	assert.True(t, s.Active)

	// Step 3: Type "git" character by character.
	for _, ch := range "git" {
		text, _, handled := s.HandleKey(tea.KeyPressMsg{Text: string(ch)})
		require.True(t, handled)
		if text != "" {
			assert.True(t, strings.Contains(strings.ToLower(text), "git"))
		}
	}

	// Current match should be "git log --oneline" (most recent "git" entry).
	m, ok := s.CurrentMatch()
	require.True(t, ok)
	assert.Equal(t, "git log --oneline", m.Entry.Display)

	// Step 4: Ctrl+R again cycles to next older match.
	text, _, handled := s.HandleKey(tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})
	require.True(t, handled)
	assert.Equal(t, "git status", text)

	// Step 5: Enter accepts the match.
	text, cmd, handled = s.HandleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.True(t, handled)
	assert.Equal(t, "git status", text)
	require.NotNil(t, cmd)

	endMsg := cmd()
	em, ok := endMsg.(HistorySearchEndMsg)
	require.True(t, ok)
	assert.True(t, em.Accepted)
	assert.Equal(t, "git status", em.Text)
	assert.False(t, s.Active, "search should be inactive after accept")
}

// TestIntegration_HistorySearchCancel verifies that Escape during search
// restores the original draft, the same way InputPane wires it.
func TestIntegration_HistorySearchCancel(t *testing.T) {
	h := NewInputHistory()
	s := NewHistorySearch(h)
	h.Add("important command")

	s.Start("my draft text", 5)
	s.SetQuery("imp")

	m, ok := s.CurrentMatch()
	require.True(t, ok)
	assert.Equal(t, "important command", m.Entry.Display)

	// Escape cancels.
	text, cmd, handled := s.HandleKey(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.True(t, handled)
	assert.Equal(t, "my draft text", text, "cancel should restore original draft")
	require.NotNil(t, cmd)

	endMsg := cmd()
	em := endMsg.(HistorySearchEndMsg)
	assert.False(t, em.Accepted)
}

// TestIntegration_PasteHandlerWithBuffer exercises the paste detection and
// completion flow through PasteHandler, matching how InputPane.Update
// delegates to Paste.HandleInput and Paste.Update.
func TestIntegration_PasteHandlerWithBuffer(t *testing.T) {
	p := NewPasteHandler()
	buf := NewInputBuffer()

	// Short input should not be treated as paste.
	handled, _ := p.HandleInput("short")
	assert.False(t, handled)

	// Long input triggers paste mode.
	longText := strings.Repeat("x", PasteThreshold+1)
	handled, cmd := p.HandleInput(longText)
	require.True(t, handled)
	require.NotNil(t, cmd)
	assert.True(t, p.IsPasting())

	// Simulate the timeout message arriving.
	// In production, bubbletea delivers the tick. Here we call HandlePasteTimeout
	// directly with the current timeout ID.
	timeoutMu.Lock()
	id := timeoutID
	timeoutMu.Unlock()

	resultCmd := p.HandlePasteTimeout(pasteTimeoutMsg{id: id})
	require.NotNil(t, resultCmd)

	msg := resultCmd()
	pm, ok := msg.(PasteCompleteMsg)
	require.True(t, ok)
	assert.Equal(t, longText, pm.Text)
	assert.False(t, pm.IsImage)

	// InputPane would insert the paste into the buffer.
	buf.Insert(pm.Text)
	assert.Equal(t, longText, buf.Value())
	assert.False(t, p.IsPasting(), "paste should be complete")
}

// TestIntegration_PasteImageDrop exercises image file drop detection.
func TestIntegration_PasteImageDrop(t *testing.T) {
	p := NewPasteHandler()

	handled, _ := p.HandleInput("/Users/me/screenshot.png")
	require.True(t, handled, "image path should trigger paste handler")
	assert.True(t, p.IsPasting())

	timeoutMu.Lock()
	id := timeoutID
	timeoutMu.Unlock()

	resultCmd := p.HandlePasteTimeout(pasteTimeoutMsg{id: id})
	require.NotNil(t, resultCmd)
	msg := resultCmd()
	pm := msg.(PasteCompleteMsg)
	assert.True(t, pm.IsImage)
	assert.Equal(t, "/Users/me/screenshot.png", pm.FilePath)
}

// TestIntegration_HistoryNavigationWithBuffer exercises arrow-key history
// navigation alongside InputBuffer sync, matching InputPane's wiring.
func TestIntegration_HistoryNavigationWithBuffer(t *testing.T) {
	h := NewInputHistory()
	buf := NewInputBuffer()

	h.Add("first command")
	h.Add("second command")

	// User types a draft.
	buf.SetValue("my draft")

	// Up arrow.
	text, changed := h.NavigateUp(buf.Value())
	require.True(t, changed)
	assert.Equal(t, "second command", text)
	buf.SetValue(text) // InputPane does this

	// Up again.
	text, changed = h.NavigateUp(buf.Value())
	require.True(t, changed)
	assert.Equal(t, "first command", text)
	buf.SetValue(text)

	// Down restores.
	text, changed = h.NavigateDown()
	require.True(t, changed)
	assert.Equal(t, "second command", text)
	buf.SetValue(text)

	// Down past newest → restore draft.
	text, changed = h.NavigateDown()
	require.True(t, changed)
	assert.Equal(t, "my draft", text)
	buf.SetValue(text)
	assert.Equal(t, "my draft", buf.Value())
}

// TestIntegration_AllHooksCoexist verifies that creating all hooks together
// (as NewInputPane does) works without conflicts.
func TestIntegration_AllHooksCoexist(t *testing.T) {
	h := NewInputHistory()
	s := NewHistorySearch(h)
	p := NewPasteHandler()
	buf := NewInputBuffer()

	// All should be in their initial states.
	assert.Equal(t, 0, h.Len())
	assert.False(t, s.Active)
	assert.False(t, p.IsPasting())
	assert.True(t, buf.IsEmpty())

	// Use all of them.
	h.Add("test command")
	buf.SetValue("hello")
	assert.Equal(t, "hello", buf.Value())
	assert.Equal(t, 1, h.Len())

	// Search should find the command.
	s.Start(buf.Value(), buf.Cursor())
	s.SetQuery("test")
	m, ok := s.CurrentMatch()
	require.True(t, ok)
	assert.Equal(t, "test command", m.Entry.Display)
	s.Cancel()

	// Paste handler should work independently.
	handled, _ := p.HandleInput("short")
	assert.False(t, handled)
}
